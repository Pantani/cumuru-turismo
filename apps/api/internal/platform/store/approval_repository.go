package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

func (r *StayRepository) Approve(
	ctx context.Context,
	command stay.ApprovalCommand,
) (result stay.MutationResult, replayed bool, err error) {
	if !r.store.phase7.Enabled {
		return result, false, stay.ErrNotFound
	}
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := decisionIdempotency(
			command.Actor, command.StayID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationApproveStay, "", now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.approveStay(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeJSON(idempotent.response.body, &result)
	})
	return result, replayed, err
}

func (r *StayRepository) Reject(
	ctx context.Context,
	command stay.RejectionCommand,
) (result stay.MutationResult, replayed bool, err error) {
	if !r.store.phase7.Enabled {
		return result, false, stay.ErrNotFound
	}
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := decisionIdempotency(
			command.Actor, command.StayID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationRejectStay,
			string(command.ReasonCode), now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.rejectStay(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeJSON(idempotent.response.body, &result)
	})
	return result, replayed, err
}

// The reason code is part of the hashed body, so the same key with a different
// reason is a 409 rather than a silent overwrite of the audited fact.
func decisionIdempotency(
	actor access.Principal,
	stayID uuid.UUID,
	expectedVersion int64,
	key string,
	operation idempotency.Operation,
	reasonCode string,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(actor.Issuer, actor.Subject),
		operation:  operation, resourceID: stayID, key: key,
		request: struct {
			StayID          uuid.UUID `json:"stay_id"`
			ExpectedVersion int64     `json:"expected_version"`
			ReasonCode      string    `json:"reason_code"`
		}{stayID, expectedVersion, reasonCode},
		now: now,
	}
}

// approveStay demands the phase's own operation, never update_stay: reusing the
// edit permission would hand approval to every operator allowed to correct a
// date, and approval is the control standing between an anonymous submission
// and a statistic (N-25, ADR-040).
func (r *StayRepository) approveStay(
	ctx context.Context,
	q generated.Querier,
	command stay.ApprovalCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := r.lockPendingStay(ctx, q, command.Actor, command.StayID, command.ExpectedVersion)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := q.ApproveSelfServiceStay(ctx, generated.ApproveSelfServiceStayParams{
		ApprovedAt: timeToPG(now), DecidedByMembershipID: locked.ActorMembershipID,
		StayID: idToPG(command.StayID), ExpectedVersion: command.ExpectedVersion,
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return storedMutation{}, decisionError(err)
	}
	result := stay.MutationResult{
		ID: idFromPG(row.ID), Status: stay.Status(row.Status), Version: row.Version,
	}
	if err := r.store.recordDecision(ctx, q, decisionEvent{
		actor: command.Actor, organizationID: idFromPG(locked.OrganizationID),
		stayID: result.ID, requestID: command.RequestID,
		action: audit.ActionStayApproved, event: outbox.EventStayApproved,
		version: result.Version, now: now, recomputePresence: true,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, result.ID, result, map[string]string{
		"ETag": entityTag(result.Version),
	})
}

// rejectStay erases the generalized visitors in the same transaction that
// stamps the decision. What survives is the auditable fact and the shell of the
// stay; keeping the "ghost guest" of a record the accommodation declared false
// serves no purpose, and age band plus municipality plus dates against a small
// accommodation are quasi-identifiers (N-29).
func (r *StayRepository) rejectStay(
	ctx context.Context,
	q generated.Querier,
	command stay.RejectionCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := r.lockPendingStay(ctx, q, command.Actor, command.StayID, command.ExpectedVersion)
	if err != nil {
		return storedMutation{}, err
	}
	reason := string(command.ReasonCode)
	row, err := q.RejectSelfServiceStay(ctx, generated.RejectSelfServiceStayParams{
		ReasonCode: &reason, DecidedByMembershipID: locked.ActorMembershipID,
		RejectedAt: timeToPG(now), StayID: idToPG(command.StayID),
		ExpectedVersion: command.ExpectedVersion,
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return storedMutation{}, decisionError(err)
	}
	if _, err := q.DeleteSelfServiceStayVisitors(ctx, idToPG(command.StayID)); err != nil {
		return storedMutation{}, stay.ErrUnavailable
	}
	return r.finishRejection(ctx, q, command, locked, row, now)
}

func (r *StayRepository) finishRejection(
	ctx context.Context,
	q generated.Querier,
	command stay.RejectionCommand,
	locked generated.LockStayForCommandRow,
	row generated.RejectSelfServiceStayRow,
	now time.Time,
) (storedMutation, error) {
	result := stay.MutationResult{
		ID: idFromPG(row.ID), Status: stay.Status(row.Status), Version: row.Version,
	}
	// The presence recalculation is emitted even though the stay is now
	// ineligible: ListPresenceReconciliationStays still returns it, and that is
	// what lets the diff erase anything already materialized.
	if err := r.store.recordDecision(ctx, q, decisionEvent{
		actor: command.Actor, organizationID: idFromPG(locked.OrganizationID),
		stayID: result.ID, requestID: command.RequestID,
		action: audit.ActionStayRejected, event: outbox.EventStayRejected,
		version: result.Version, now: now, recomputePresence: true,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, result.ID, result, map[string]string{
		"ETag": entityTag(result.Version),
	})
}

// lockPendingStay refuses an assisted stay with 422 and a decided one with a
// conflict, so approving an already rejected stay and rejecting an already
// approved one both fail deterministically (N-34).
func (r *StayRepository) lockPendingStay(
	ctx context.Context,
	q generated.Querier,
	actor access.Principal,
	stayID uuid.UUID,
	expectedVersion int64,
) (generated.LockStayForCommandRow, error) {
	locked, err := q.LockStayForCommand(ctx, lockStayParams(stayID, expectedVersion, actor))
	if err != nil {
		return locked, stayCommandError(ctx, q, actor, stayID, expectedVersion, err)
	}
	if stay.Provenance(locked.Provenance) != stay.ProvenanceSelfService {
		return locked, stay.ErrInvalidInput
	}
	if stay.ApprovalStateFromColumn(locked.ApprovalState) != stay.ApprovalPending {
		return locked, stay.ErrConflict
	}
	return locked, approvalAllowed(locked)
}

func approvalAllowed(locked generated.LockStayForCommandRow) error {
	if !accommodation.Status(locked.AccommodationStatus).
		Allows(accommodation.OperationApproveStay) {
		return stay.ErrConflict
	}
	if accommodation.Role(locked.ActorRole) != accommodation.RoleManager {
		return stay.ErrForbidden
	}
	return nil
}

// Zero rows means the row moved between the lock and the write, which is a
// conflict and never a 500.
func decisionError(err error) error {
	if err == nil {
		return nil
	}
	return stayUpdateError(err)
}

type decisionEvent struct {
	actor             access.Principal
	organizationID    uuid.UUID
	stayID            uuid.UUID
	requestID         string
	action            audit.Action
	event             outbox.EventType
	version           int64
	now               time.Time
	recomputePresence bool
}

// The audit event carries the action, the actor digest, the entity and the
// changed field. It carries no reason text, because the reason is a closed code
// and platform.audit_events is append-only: free text there would be permanent
// personal data on the path designed to erase it.
func (s *Store) recordDecision(
	ctx context.Context,
	q generated.Querier,
	spec decisionEvent,
) error {
	err := s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: spec.actor.Issuer,
		actorSubject: spec.actor.Subject, organization: spec.organizationID,
		action: spec.action, entityType: audit.EntityStay, entityID: spec.stayID,
		requestID: spec.requestID, changedFields: []audit.ChangedField{audit.FieldStatus},
		version: spec.version, aggregateType: outbox.AggregateStay,
		eventType: spec.event, now: spec.now,
	})
	if err != nil || !spec.recomputePresence {
		return err
	}
	return insertPresenceEvent(ctx, q, spec.stayID, spec.version)
}

// ExpireApprovals is the sweep the worker runs. Erasing only on rejection would
// make the retention avoidable by inaction: doing nothing would be the easiest
// way to keep the data forever. The expiry therefore performs exactly the same
// purge as the rejection (E-05, N-30).
func (r *StayRepository) ExpireApprovals(ctx context.Context) (int, error) {
	if !r.store.phase7.Enabled {
		return 0, nil
	}
	now := r.store.currentTime()
	// One identifier per sweep, so every row a run expired correlates in the
	// audit trail. It is the only thing the sweep has that a request id is for.
	runID, err := uuid.NewV7()
	if err != nil {
		return 0, stay.ErrUnavailable
	}
	var expired int
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		rows, sweepErr := q.ExpirePendingSelfServiceStays(
			ctx, generated.ExpirePendingSelfServiceStaysParams{
				Cutoff: timeToPG(now), BatchSize: r.store.phase7.ExpirySweepBatchSize,
			},
		)
		if sweepErr != nil {
			return stay.ErrUnavailable
		}
		expired = len(rows)
		return r.purgeExpired(ctx, q, rows, runID.String(), now)
	})
	return expired, err
}

func (r *StayRepository) purgeExpired(
	ctx context.Context,
	q generated.Querier,
	rows []generated.ExpirePendingSelfServiceStaysRow,
	runID string,
	now time.Time,
) error {
	for _, row := range rows {
		if err := r.purgeExpiredStay(ctx, q, row, runID, now); err != nil {
			return err
		}
	}
	return nil
}

// The purge and the audit row land in the same transaction. That ordering is
// the point: an audit event that fails to validate aborts the transaction, and
// an aborted transaction means the purge never happened — which is exactly the
// retention hole E-05 describes. The organization comes from the accommodation
// projected by the sweep query, because the sweep has no membership to resolve
// it through.
func (r *StayRepository) purgeExpiredStay(
	ctx context.Context,
	q generated.Querier,
	row generated.ExpirePendingSelfServiceStaysRow,
	runID string,
	now time.Time,
) error {
	if _, err := q.DeleteSelfServiceStayVisitors(ctx, row.ID); err != nil {
		return stay.ErrUnavailable
	}
	if err := r.store.recordEvents(ctx, q, expirySweepEvent(
		idFromPG(row.ID), idFromPG(row.OrganizationID), row.Version, runID, now,
	)); err != nil {
		return err
	}
	// The stay left the eligible set, so anything already materialized for it
	// has to be erased by the next reconciliation diff.
	return insertPresenceEvent(ctx, q, idFromPG(row.ID), row.Version)
}

const (
	// The expiry has no deciding membership: nobody decided, the clock did, and
	// the 'expired' branch of stays_approval_fields_valid forbids one. The actor
	// is therefore the process, named by a URN in the same shape the invite path
	// already uses. Naming the process is honest; borrowing a person's identity
	// to satisfy a NOT NULL would not be.
	systemActorIssuer = "urn:cumuru:system"
	expirySweepActor  = "approval-expiry"
)

func expirySweepEvent(
	stayID uuid.UUID,
	organizationID uuid.UUID,
	version int64,
	runID string,
	now time.Time,
) eventSpec {
	return eventSpec{
		actorType: audit.ActorSystem, actorIssuer: systemActorIssuer,
		actorSubject: expirySweepActor, organization: organizationID,
		action: audit.ActionStayApprovalExpired, entityType: audit.EntityStay,
		entityID: stayID, requestID: runID,
		changedFields: []audit.ChangedField{audit.FieldStatus},
		version:       version, aggregateType: outbox.AggregateStay,
		eventType: outbox.EventStayApprovalExpired, now: now,
	}
}
