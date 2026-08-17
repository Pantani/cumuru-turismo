package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/proofofwork"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type StayRepository struct {
	store *Store
	codec *stay.InviteCodec
	// challenges is nil when the feature is off, and every open-channel route
	// answers not-found rather than issuing a challenge with no key.
	challenges *proofofwork.Issuer
	// hashKey is resolved once so no write path can reach the request digest
	// without it. There is no unkeyed fallback to degrade into.
	hashKey []byte
}

func NewStayRepository(store *Store) (*StayRepository, error) {
	codec, err := stay.NewInviteCodec(stay.InviteKeyring{
		CurrentVersion: store.core.InviteKeys.CurrentVersion,
		Keys:           store.core.InviteKeys.Keys,
	})
	if err != nil {
		return nil, err
	}
	hashKey, err := store.requestHashKey()
	if err != nil {
		return nil, err
	}
	challenges, err := newChallengeIssuer(store.selfService)
	if err != nil {
		return nil, err
	}
	return &StayRepository{
		store: store, codec: codec, hashKey: hashKey, challenges: challenges,
	}, nil
}

// newChallengeIssuer fails closed: with the feature on and no usable key the
// repository refuses to exist, because a challenge signed with an empty key is
// a MAC anybody can forge.
func newChallengeIssuer(selfService config.SelfServiceConfig) (*proofofwork.Issuer, error) {
	if !selfService.Enabled {
		return nil, nil
	}
	return proofofwork.NewIssuer(proofofwork.Keyring{
		CurrentVersion: selfService.ProofOfWorkKeys.CurrentVersion,
		Keys:           selfService.ProofOfWorkKeys.Keys,
	}, selfService.ChallengeTTL)
}

var _ stay.Repository = (*StayRepository)(nil)

func (r *StayRepository) Create(
	ctx context.Context,
	command stay.CreateCommand,
) (result stay.MutationResult, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := createStayIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.createStay(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		if decodeErr := json.Unmarshal(idempotent.response.body, &result); decodeErr != nil {
			return stay.ErrUnavailable
		}
		replayed = idempotent.replayed
		return nil
	})
	return result, replayed, err
}

// A stay may only be created against an accommodation whose status still
// allows it, so the identifier is minted only after that check passes.
func newStayID(property generated.GetAccessibleAccommodationRow) (uuid.UUID, error) {
	if !accommodation.Status(property.Status).Allows(accommodation.OperationCreateStay) {
		return uuid.Nil, stay.ErrConflict
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	return id, nil
}

func (r *StayRepository) createStay(
	ctx context.Context,
	q generated.Querier,
	command stay.CreateCommand,
	now time.Time,
) (storedMutation, error) {
	property, err := q.GetAccessibleAccommodation(ctx, accommodationKey(command.AccommodationID, command.Actor))
	if err != nil {
		return storedMutation{}, stayQueryError(err)
	}
	id, err := newStayID(property)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := q.CreateStay(ctx, generated.CreateStayParams{
		StayID: idToPG(id), ClientSubmissionID: idToPG(command.ClientSubmissionID),
		PlannedArrivalOn:   dateToPG(command.PlannedArrivalOn),
		PlannedDepartureOn: dateToPG(command.PlannedDepartureOn),
		ExpectedGuestCount: command.ExpectedGuestCount,
		AccommodationID:    idToPG(command.AccommodationID),
		OidcIssuer:         command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return storedMutation{}, stayMutationError(err)
	}
	result := stayFromCreate(row)
	event := stayEventSpec{
		actor: command.Actor, organizationID: idFromPG(property.OrganizationID),
		action: audit.ActionStayCreated, eventType: outbox.EventStayCreated,
		stayID: result.ID, version: result.Version, requestID: command.RequestID,
		fields: []audit.ChangedField{
			audit.FieldPlannedArrival, audit.FieldPlannedDeparture, audit.FieldExpectedGuests,
		}, now: now,
	}
	if err := r.store.recordStayEvents(ctx, q, event); err != nil {
		return storedMutation{}, err
	}
	mutation := mutationResult(result)
	return jsonMutation(201, result.ID, mutation, map[string]string{
		"Location": "/api/v1/stays/" + result.ID.String(), "ETag": entityTag(result.Version),
	})
}

func (r *StayRepository) List(
	ctx context.Context,
	actor access.Principal,
	page stay.PageRequest,
) (stay.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListAccessibleStays(ctx, listStaysParams(actor, page))
	if err != nil {
		return stay.Page{}, stay.ErrUnavailable
	}
	return stayPage(rows, page.Limit), nil
}

func (r *StayRepository) Get(
	ctx context.Context,
	actor access.Principal,
	id uuid.UUID,
) (stay.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.GetAccessibleStay(ctx, stayKey(id, actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return stay.Record{}, stay.ErrNotFound
	}
	if err != nil {
		return stay.Record{}, stay.ErrUnavailable
	}
	return stayFromGet(row), nil
}

func (r *StayRepository) Update(
	ctx context.Context,
	command stay.UpdateCommand,
) (result stay.Record, err error) {
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		result, err = r.updateStay(ctx, q, command)
		return err
	})
	return result, err
}

func updatableStay(current generated.GetAccessibleStayRow, patch stay.UpdatePatch) bool {
	return accommodation.Status(current.AccommodationStatus).
		Allows(accommodation.OperationUpdateStay) &&
		validMergedStay(current, patch)
}

func (r *StayRepository) updateStay(
	ctx context.Context,
	q generated.Querier,
	command stay.UpdateCommand,
) (result stay.Record, err error) {
	current, err := q.GetAccessibleStay(ctx, stayKey(command.StayID, command.Actor))
	if err != nil {
		return result, stayQueryError(err)
	}
	if current.Version != command.ExpectedVersion {
		return result, stay.ErrPreconditionFailed
	}
	if !updatableStay(current, command.Patch) {
		return result, stay.ErrConflict
	}
	updated, err := q.UpdateStay(ctx, updateStayParams(command, r.store.currentTime()))
	if err != nil {
		return result, stayCommandError(
			ctx, q, command.Actor, command.StayID, command.ExpectedVersion, err,
		)
	}
	result = stayFromUpdate(updated, current.VisitorCount, factsFromGet(current))
	err = r.store.recordStayUpdate(ctx, q, command, current, result)
	return result, err
}

func (r *StayRepository) GetGroup(
	ctx context.Context,
	actor access.Principal,
	id uuid.UUID,
) (stay.Group, int64, error) {
	var result stay.Group
	var version int64
	err := r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		var readErr error
		result, version, readErr = readStayGroup(ctx, q, actor, id)
		return readErr
	})
	return result, version, err
}

func readStayGroup(
	ctx context.Context,
	q generated.Querier,
	actor access.Principal,
	id uuid.UUID,
) (stay.Group, int64, error) {
	current, err := q.GetAccessibleStay(ctx, stayKey(id, actor))
	if err != nil {
		return stay.Group{}, 0, stayQueryError(err)
	}
	submission, err := q.GetStayGroupSubmission(ctx, generated.GetStayGroupSubmissionParams{
		StayID: idToPG(id), OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	})
	if err != nil {
		return stay.Group{}, 0, stayQueryError(err)
	}
	rows, err := q.ListVisitorsForStay(ctx, generated.ListVisitorsForStayParams{
		StayID: idToPG(id), OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	})
	if err != nil {
		return stay.Group{}, 0, stay.ErrUnavailable
	}
	visitors := make([]stay.VisitorRecord, 0, len(rows))
	for _, row := range rows {
		visitors = append(visitors, visitorRecord(row))
	}
	return stay.Group{
		StayID: id, PrivacyNoticeVersion: submission.PrivacyNoticeVersion, Visitors: visitors,
	}, current.Version, nil
}

func (r *StayRepository) SubmitAssistedGroup(
	ctx context.Context,
	command stay.GroupCommand,
) (result stay.SubmissionAccepted, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := assistedGroupIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.submitAssistedGroup(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		var payload groupReplayPayload
		if decodeErr := json.Unmarshal(idempotent.response.body, &payload); decodeErr != nil {
			return stay.ErrUnavailable
		}
		result, runErr = r.reconstructGroupSubmission(payload)
		if runErr != nil {
			return runErr
		}
		replayed = idempotent.replayed
		return nil
	})
	return result, replayed, err
}

// Revoking outstanding invites first makes the assisted path authoritative:
// a pending invite must not be able to overwrite what the operator typed.
func writeAssistedGroup(
	ctx context.Context,
	q generated.Querier,
	hashKey []byte,
	command stay.GroupCommand,
	locked generated.LockStayForCommandRow,
	now time.Time,
) (int64, error) {
	if _, err := q.RevokeActiveInvites(ctx, generated.RevokeActiveInvitesParams{
		RevokedAt: timeToPG(now), StayID: idToPG(command.StayID),
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}); err != nil {
		return 0, stay.ErrUnavailable
	}
	if _, err := insertAssistedGroup(ctx, q, hashKey, command, now); err != nil {
		return 0, err
	}
	if err := insertAssistedVisitors(ctx, q, command); err != nil {
		return 0, err
	}
	return updateAssistedStay(ctx, q, command, locked, now)
}

func (r *StayRepository) recordAssistedGroupEvents(
	ctx context.Context,
	q generated.Querier,
	command stay.GroupCommand,
	locked generated.LockStayForCommandRow,
	version int64,
	now time.Time,
) error {
	event := stayEventSpec{
		actor: command.Actor, organizationID: idFromPG(locked.OrganizationID),
		action: audit.ActionStayGroupSubmitted, eventType: outbox.EventStayGroupSubmitted,
		stayID: command.StayID, version: version, requestID: command.RequestID,
		fields: []audit.ChangedField{audit.FieldExpectedGuests, audit.FieldStatus}, now: now,
	}
	if err := r.store.recordStayEvents(ctx, q, event); err != nil {
		return err
	}
	if err := insertPresenceEvent(ctx, q, command.StayID, version); err != nil {
		return stay.ErrUnavailable
	}
	return nil
}

func (r *StayRepository) submitAssistedGroup(
	ctx context.Context,
	q generated.Querier,
	command stay.GroupCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := lockAssistedStay(ctx, q, command)
	if err != nil {
		return storedMutation{}, err
	}
	version, err := writeAssistedGroup(ctx, q, r.hashKey, command, locked, now)
	if err != nil {
		return storedMutation{}, err
	}
	result := acceptedSubmission(command.ClientSubmissionID, version)
	grant, err := r.store.issueSurveyCapability(ctx, q, command.StayID, now)
	if err != nil {
		return storedMutation{}, stay.ErrUnavailable
	}
	if err := r.recordAssistedGroupEvents(ctx, q, command, locked, version, now); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(
		200,
		command.StayID,
		groupReplay(result, grant),
		map[string]string{"ETag": entityTag(version)},
	)
}

func lockAssistedStay(
	ctx context.Context,
	q generated.Querier,
	command stay.GroupCommand,
) (generated.LockStayForCommandRow, error) {
	locked, err := q.LockStayForCommand(ctx, lockStayParams(command.StayID, command.ExpectedVersion, command.Actor))
	if err != nil {
		return locked, stayCommandError(
			ctx, q, command.Actor, command.StayID, command.ExpectedVersion, err,
		)
	}
	next, err := stay.Status(locked.Status).Transition(stay.EventSubmitGroup)
	if err != nil || next != stay.StatusPreRegistered {
		return locked, stay.ErrConflict
	}
	if !accommodation.Status(locked.AccommodationStatus).Allows(accommodation.OperationSubmitGroup) {
		return locked, stay.ErrConflict
	}
	return locked, nil
}

func (r *StayRepository) CreateInvite(
	ctx context.Context,
	command stay.InviteCommand,
) (result stay.InviteCreated, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := inviteIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.createInvite(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		var payload inviteReplayPayload
		if decodeErr := json.Unmarshal(idempotent.response.body, &payload); decodeErr != nil {
			return stay.ErrUnavailable
		}
		result, runErr = r.reconstructInvite(payload)
		replayed = idempotent.replayed
		return runErr
	})
	return result, replayed, err
}

type inviteReplayPayload struct {
	InviteID   uuid.UUID `json:"invite_id"`
	KeyVersion string    `json:"key_version"`
	ExpiresAt  time.Time `json:"expires_at"`
	Version    int64     `json:"version"`
}

func invitable(locked generated.LockStayForCommandRow) bool {
	next, err := stay.Status(locked.Status).Transition(stay.EventInvite)
	if err != nil || next != stay.StatusInvited {
		return false
	}
	return accommodation.Status(locked.AccommodationStatus).
		Allows(accommodation.OperationIssueInvite)
}

// Issuing a new invite revokes any outstanding one first, so a stay never has
// two live capabilities at the same time.
func (r *StayRepository) replaceInvite(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteCommand,
	locked generated.LockStayForCommandRow,
	now time.Time,
) (inviteReplayPayload, error) {
	if !invitable(locked) {
		return inviteReplayPayload{}, stay.ErrConflict
	}
	if _, err := q.RevokeActiveInvites(ctx, generated.RevokeActiveInvitesParams{
		RevokedAt: timeToPG(now), StayID: idToPG(command.StayID),
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}); err != nil {
		return inviteReplayPayload{}, stay.ErrUnavailable
	}
	payload, err := r.insertInvite(ctx, q, command, now)
	if err != nil {
		return inviteReplayPayload{}, err
	}
	transitioned, err := q.ApplyStayTransition(ctx, transitionParams(
		locked, command.Actor, stay.StatusInvited, now, "", "",
	))
	if err != nil {
		return inviteReplayPayload{}, stayUpdateError(err)
	}
	payload.Version = transitioned.Version
	return payload, nil
}

func (r *StayRepository) createInvite(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := q.LockStayForCommand(ctx, lockStayParams(command.StayID, command.ExpectedVersion, command.Actor))
	if err != nil {
		return storedMutation{}, stayCommandError(ctx, q, command.Actor, command.StayID, command.ExpectedVersion, err)
	}
	payload, err := r.replaceInvite(ctx, q, command, locked, now)
	if err != nil {
		return storedMutation{}, err
	}
	event := stayEventSpec{
		actor: command.Actor, organizationID: idFromPG(locked.OrganizationID),
		action: audit.ActionStayInvited, eventType: outbox.EventStayInvited,
		stayID: command.StayID, version: payload.Version,
		requestID: command.RequestID, fields: []audit.ChangedField{audit.FieldStatus}, now: now,
	}
	if err := r.store.recordStayEvents(ctx, q, event); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(201, payload.InviteID, payload, map[string]string{
		"Location": "/api/v1/stays/" + command.StayID.String() + "/invite",
		"ETag":     entityTag(payload.Version),
	})
}

func (r *StayRepository) insertInvite(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteCommand,
	now time.Time,
) (inviteReplayPayload, error) {
	inviteID, err := uuid.NewV7()
	if err != nil {
		return inviteReplayPayload{}, stay.ErrUnavailable
	}
	token, keyVersion, err := r.codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if err != nil {
		return inviteReplayPayload{}, stay.ErrUnavailable
	}
	digest, err := r.codec.StorageDigest(token, keyVersion)
	if err != nil {
		return inviteReplayPayload{}, stay.ErrUnavailable
	}
	expiresAt := now.Add(r.store.core.InviteTTL)
	_, err = q.CreateInvite(ctx, generated.CreateInviteParams{
		InviteID: idToPG(inviteID), TokenHmac: digest, TokenKeyVersion: keyVersion,
		PrivacyNoticeVersion: command.PrivacyNoticeVersion,
		ExpiresAt:            timeToPG(expiresAt), StayID: idToPG(command.StayID),
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return inviteReplayPayload{}, stayMutationError(err)
	}
	return inviteReplayPayload{
		InviteID: inviteID, KeyVersion: keyVersion, ExpiresAt: expiresAt,
	}, nil
}

func (r *StayRepository) reconstructInvite(payload inviteReplayPayload) (stay.InviteCreated, error) {
	token, err := r.codec.Reconstruct(stay.PurposeStayGroupSubmission, payload.InviteID, payload.KeyVersion)
	if err != nil {
		return stay.InviteCreated{}, stay.ErrUnavailable
	}
	base := *r.store.core.InviteBaseURL
	base.Path = strings.TrimRight(base.Path, "/") + "/" + token
	return stay.InviteCreated{
		InviteID: payload.InviteID, URL: base.String(),
		ExpiresAt: payload.ExpiresAt, Version: payload.Version,
	}, nil
}

func (r *StayRepository) Transition(
	ctx context.Context,
	command stay.TransitionCommand,
) (result stay.MutationResult, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := transitionIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.transition(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		if decodeErr := json.Unmarshal(idempotent.response.body, &result); decodeErr != nil {
			return stay.ErrUnavailable
		}
		replayed = idempotent.replayed
		return nil
	})
	return result, replayed, err
}

func allowedTransition(
	command stay.TransitionCommand,
	locked generated.LockStayForCommandRow,
	occurredAt time.Time,
) (stay.Status, error) {
	next, err := validateTransition(command, locked, occurredAt)
	if err != nil {
		return "", stay.ErrConflict
	}
	if !accommodation.Status(locked.AccommodationStatus).
		Allows(operationForTransition(command.Kind)) {
		return "", stay.ErrConflict
	}
	return next, nil
}

// An unset occurrence time means "now"; a supplied one is normalized to UTC.
func transitionTime(occurredAt, now time.Time) time.Time {
	value := occurredAt.UTC()
	if value.IsZero() {
		return now
	}
	return value
}

func (r *StayRepository) recordTransitionEvents(
	ctx context.Context,
	q generated.Querier,
	command stay.TransitionCommand,
	locked generated.LockStayForCommandRow,
	result stay.Record,
	occurredAt time.Time,
) error {
	action, eventType := transitionEvents(command.Kind)
	event := stayEventSpec{
		actor: command.Actor, organizationID: idFromPG(locked.OrganizationID),
		action: action, eventType: eventType, stayID: command.StayID,
		version: result.Version, requestID: command.RequestID,
		fields: []audit.ChangedField{audit.FieldStatus}, now: occurredAt,
	}
	if err := r.store.recordStayEvents(ctx, q, event); err != nil {
		return err
	}
	if err := insertPresenceEvent(ctx, q, command.StayID, result.Version); err != nil {
		return stay.ErrUnavailable
	}
	return nil
}

func (r *StayRepository) transition(
	ctx context.Context,
	q generated.Querier,
	command stay.TransitionCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := q.LockStayForCommand(ctx, lockStayParams(command.StayID, command.ExpectedVersion, command.Actor))
	if err != nil {
		return storedMutation{}, stayCommandError(ctx, q, command.Actor, command.StayID, command.ExpectedVersion, err)
	}
	occurredAt := transitionTime(command.OccurredAt, now)
	next, err := allowedTransition(command, locked, occurredAt)
	if err != nil {
		return storedMutation{}, err
	}
	updated, err := q.ApplyStayTransition(ctx, transitionParams(
		locked, command.Actor, next, occurredAt, command.ReasonCode, command.Kind,
	))
	if err != nil {
		return storedMutation{}, stayUpdateError(err)
	}
	result := stayFromTransition(updated, locked.VisitorCount, factsFromLock(locked))
	if err := r.recordTransitionEvents(ctx, q, command, locked, result, occurredAt); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, command.StayID, mutationResult(result), map[string]string{
		"ETag": entityTag(result.Version),
	})
}

func (r *StayRepository) GetInvite(
	ctx context.Context,
	request stay.InviteRequest,
) (stay.InviteContext, error) {
	now := r.store.currentTime()
	err := r.applyRateLimit(
		ctx, "invite_context", request.Token, request.RateSubject,
		r.store.core.InviteContextRateLimit, now,
	)
	if err != nil {
		return stay.InviteContext{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	capability, err := r.resolveCapability(ctx, r.store.queries, request.Token, now, false)
	if err != nil {
		return stay.InviteContext{}, err
	}
	return inviteContext(capability.row), nil
}

func (r *StayRepository) SubmitInviteGroup(
	ctx context.Context,
	command stay.InviteGroupCommand,
) (result stay.SubmissionAccepted, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.applyRateLimit(
		ctx, "invite_submit", command.Token, command.RateSubject,
		r.store.core.InviteSubmitRateLimit, now,
	)
	if err != nil {
		return result, false, err
	}
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		capability, resolveErr := r.resolveCapability(ctx, q, command.Token, now, true)
		if resolveErr != nil {
			return resolveErr
		}
		spec := inviteGroupIdempotency(command, capability.inviteID, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.submitInviteGroup(ctx, q, command, capability, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		result, runErr = r.decodeGroupSubmission(idempotent.response.body)
		replayed = idempotent.replayed
		return runErr
	})
	return result, replayed, err
}

type resolvedCapability struct {
	inviteID uuid.UUID
	digest   []byte
	row      generated.GetInviteForCapabilityRow
}

func (r *StayRepository) decodeGroupSubmission(
	body []byte,
) (stay.SubmissionAccepted, error) {
	var payload groupReplayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return stay.SubmissionAccepted{}, stay.ErrUnavailable
	}
	return r.reconstructGroupSubmission(payload)
}

func (r *StayRepository) resolveCapability(
	ctx context.Context,
	q generated.Querier,
	token string,
	now time.Time,
	allowConsumed bool,
) (resolvedCapability, error) {
	inviteID, err := capabilityInviteID(token)
	if err != nil {
		return resolvedCapability{}, stay.ErrNotFound
	}
	row, err := q.GetInviteForCapability(ctx, idToPG(inviteID))
	if err != nil {
		return resolvedCapability{}, stayQueryError(err)
	}
	digest, err := r.verifiedDigest(token, inviteID, row)
	if err != nil {
		return resolvedCapability{}, err
	}
	if !validCapabilityRow(row, now, allowConsumed) {
		return resolvedCapability{}, stay.ErrNotFound
	}
	return resolvedCapability{inviteID: inviteID, digest: digest, row: row}, nil
}

// Both the signature and the stored digest must match; a mismatch is reported
// as not-found so a probe cannot distinguish a bad token from a missing invite.
func (r *StayRepository) verifiedDigest(
	token string,
	inviteID uuid.UUID,
	row generated.GetInviteForCapabilityRow,
) ([]byte, error) {
	verifiedID, err := r.codec.Verify(stay.PurposeStayGroupSubmission, token, row.TokenKeyVersion)
	if err != nil || verifiedID != inviteID {
		return nil, stay.ErrNotFound
	}
	digest, err := r.codec.StorageDigest(token, row.TokenKeyVersion)
	if err != nil || !hmac.Equal(digest, row.TokenHmac) {
		return nil, stay.ErrNotFound
	}
	return digest, nil
}

// Consuming the invite is the serialization point: a second submission finds
// no consumable row and is reported as already consumed.
func consumeInvite(
	ctx context.Context,
	q generated.Querier,
	capability resolvedCapability,
	now time.Time,
) (generated.ConsumeInviteRow, error) {
	consumed, err := q.ConsumeInvite(ctx, generated.ConsumeInviteParams{
		ConsumedAt: timeToPG(now), InviteID: idToPG(capability.inviteID),
		TokenHmac: capability.digest,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return generated.ConsumeInviteRow{}, stay.ErrInviteConsumed
	}
	if err != nil {
		return generated.ConsumeInviteRow{}, stay.ErrUnavailable
	}
	return consumed, nil
}

// writeInviteGroup consumes the capability, stores the group and finalizes the
// stay in one transaction, so a partial submission can never survive.
func writeInviteGroup(
	ctx context.Context,
	q generated.Querier,
	hashKey []byte,
	command stay.InviteGroupCommand,
	capability resolvedCapability,
	now time.Time,
) (generated.ConsumeInviteRow, generated.FinalizeInviteSubmissionRow, error) {
	var finalized generated.FinalizeInviteSubmissionRow
	consumed, err := consumeInvite(ctx, q, capability, now)
	if err != nil {
		return consumed, finalized, err
	}
	if _, err := insertInviteGroup(ctx, q, hashKey, command, capability, now); err != nil {
		return consumed, finalized, err
	}
	if err := insertInviteVisitors(ctx, q, command, capability, now); err != nil {
		return consumed, finalized, err
	}
	finalized, err = q.FinalizeInviteSubmission(ctx, generated.FinalizeInviteSubmissionParams{
		ExpectedGuestCount: int32(len(command.Visitors)), FinalizedAt: timeToPG(now),
		InviteID: idToPG(capability.inviteID), TokenHmac: capability.digest,
		StayID: consumed.StayID, ExpectedVersion: capability.row.StayVersion,
	})
	if err != nil {
		return consumed, finalized, stayUpdateError(err)
	}
	return consumed, finalized, nil
}

func (r *StayRepository) submitInviteGroup(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteGroupCommand,
	capability resolvedCapability,
	now time.Time,
) (storedMutation, error) {
	if command.PrivacyNoticeVersion != capability.row.PrivacyNoticeVersion {
		return storedMutation{}, stay.ErrConflict
	}
	consumed, finalized, err := writeInviteGroup(ctx, q, r.hashKey, command, capability, now)
	if err != nil {
		return storedMutation{}, err
	}
	result := acceptedSubmission(command.ClientSubmissionID, finalized.Version)
	grant, err := r.store.issueSurveyCapability(ctx, q, idFromPG(consumed.StayID), now)
	if err != nil {
		return storedMutation{}, stay.ErrUnavailable
	}
	if err := r.store.recordInviteSubmission(ctx, q, command, capability, result, now); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, idFromPG(consumed.StayID), groupReplay(result, grant), map[string]string{
		"ETag": entityTag(finalized.Version),
	})
}

type groupReplayPayload struct {
	SubmissionID     uuid.UUID   `json:"submission_id"`
	Status           string      `json:"status"`
	StayStatus       stay.Status `json:"stay_status"`
	Version          int64       `json:"version"`
	CapabilityID     uuid.UUID   `json:"survey_capability_id,omitempty"`
	CapabilityKey    string      `json:"survey_capability_key_version,omitempty"`
	QuestionnaireID  uuid.UUID   `json:"questionnaire_version_id,omitempty"`
	CapabilityExpiry time.Time   `json:"survey_capability_expires_at,omitempty"`
}

func groupReplay(result stay.SubmissionAccepted, grant surveyGrant) groupReplayPayload {
	return groupReplayPayload{
		SubmissionID: result.SubmissionID, Status: result.Status,
		StayStatus: result.StayStatus, Version: result.Version,
		CapabilityID: grant.CapabilityID, CapabilityKey: grant.KeyVersion,
		QuestionnaireID: grant.VersionID, CapabilityExpiry: grant.ExpiresAt,
	}
}

func (r *StayRepository) reconstructGroupSubmission(
	payload groupReplayPayload,
) (stay.SubmissionAccepted, error) {
	grant, err := r.store.reconstructSurveyGrant(
		payload.CapabilityID, payload.CapabilityKey,
		payload.QuestionnaireID, payload.CapabilityExpiry,
	)
	if err != nil {
		return stay.SubmissionAccepted{}, stay.ErrUnavailable
	}
	return stay.SubmissionAccepted{
		SubmissionID: payload.SubmissionID, Status: payload.Status,
		StayStatus: payload.StayStatus, Version: payload.Version,
		SurveyCapability: grant.Token,
	}, nil
}

func createStayIdempotency(command stay.CreateCommand, now time.Time) idempotencySpec {
	request := struct {
		ClientSubmissionID uuid.UUID `json:"client_submission_id"`
		PlannedArrivalOn   string    `json:"planned_arrival_on"`
		PlannedDepartureOn string    `json:"planned_departure_on"`
		ExpectedGuestCount int32     `json:"expected_guest_count"`
	}{
		command.ClientSubmissionID, command.PlannedArrivalOn,
		command.PlannedDepartureOn, command.ExpectedGuestCount,
	}
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateStay, resourceID: command.AccommodationID,
		key: command.IdempotencyKey, request: request, now: now,
	}
}

func assistedGroupIdempotency(command stay.GroupCommand, now time.Time) idempotencySpec {
	request := groupRequestHashValue(
		command.ClientSubmissionID, command.PrivacyNoticeVersion, command.Visitors,
	)
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationSubmitAssistedGroup, resourceID: command.StayID,
		key: command.IdempotencyKey, request: request, now: now,
	}
}

func inviteIdempotency(command stay.InviteCommand, now time.Time) idempotencySpec {
	request := struct {
		PrivacyNoticeVersion string `json:"privacy_notice_version"`
	}{command.PrivacyNoticeVersion}
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateInvite, resourceID: command.StayID,
		key: command.IdempotencyKey, request: request, now: now,
	}
}

func transitionIdempotency(command stay.TransitionCommand, now time.Time) idempotencySpec {
	request := struct {
		OccurredAt time.Time `json:"occurred_at"`
		ReasonCode string    `json:"reason_code,omitempty"`
		Correction bool      `json:"correction,omitempty"`
	}{command.OccurredAt.UTC(), command.ReasonCode, command.Correction}
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  transitionOperation(command.Kind), resourceID: command.StayID,
		key: command.IdempotencyKey, request: request, now: now,
	}
}

func inviteGroupIdempotency(
	command stay.InviteGroupCommand,
	inviteID uuid.UUID,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: inviteID.String(), operation: idempotency.OperationSubmitInviteGroup,
		resourceID: inviteID, key: command.IdempotencyKey,
		request: groupRequestHashValue(
			command.ClientSubmissionID, command.PrivacyNoticeVersion, command.Visitors,
		),
		now: now,
	}
}

func groupRequestHashValue(
	submissionID uuid.UUID,
	privacyVersion string,
	visitors []stay.Visitor,
) any {
	return struct {
		ClientSubmissionID   uuid.UUID      `json:"client_submission_id"`
		PrivacyNoticeVersion string         `json:"privacy_notice_version"`
		Visitors             []stay.Visitor `json:"visitors"`
	}{submissionID, privacyVersion, visitors}
}

func transitionOperation(kind stay.TransitionKind) idempotency.Operation {
	operations := map[stay.TransitionKind]idempotency.Operation{
		stay.TransitionCheckIn:  idempotency.OperationCheckIn,
		stay.TransitionCheckOut: idempotency.OperationCheckOut,
		stay.TransitionCancel:   idempotency.OperationCancel,
		stay.TransitionNoShow:   idempotency.OperationNoShow,
	}
	return operations[kind]
}

func jsonMutation(
	statusCode int,
	resourceID uuid.UUID,
	value any,
	headers map[string]string,
) (storedMutation, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return storedMutation{}, stay.ErrUnavailable
	}
	return storedMutation{
		status: statusCode, resourceID: resourceID, body: body, headers: headers,
	}, nil
}

func mutationResult(record stay.Record) stay.MutationResult {
	return stay.MutationResult{ID: record.ID, Status: record.Status, Version: record.Version}
}

func acceptedSubmission(clientSubmissionID uuid.UUID, version int64) stay.SubmissionAccepted {
	return stay.SubmissionAccepted{
		SubmissionID: clientSubmissionID,
		Status:       "accepted",
		StayStatus:   stay.StatusPreRegistered,
		Version:      version,
	}
}

func listStaysParams(
	actor access.Principal,
	page stay.PageRequest,
) generated.ListAccessibleStaysParams {
	var status *generated.CoreStayStatus
	if page.Status != "" {
		value := generated.CoreStayStatus(page.Status)
		status = &value
	}
	return generated.ListAccessibleStaysParams{
		OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
		AccommodationID: idToPG(page.AccommodationID), StayStatus: status,
		ApprovalState: optionalText(string(page.ApprovalState)),
		Provenance:    optionalText(string(page.Provenance)),
		ArrivalFrom:   dateToPG(page.ArrivalFrom), ArrivalTo: dateToPG(page.ArrivalTo),
		CursorCreatedAt: timeToPG(page.CursorCreatedAt), CursorID: idToPG(page.CursorID),
		PageLimit: page.Limit + 1,
	}
}

func stayKey(id uuid.UUID, actor access.Principal) generated.GetAccessibleStayParams {
	return generated.GetAccessibleStayParams{
		StayID: idToPG(id), OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	}
}

func lockStayParams(
	id uuid.UUID,
	version int64,
	actor access.Principal,
) generated.LockStayForCommandParams {
	return generated.LockStayForCommandParams{
		StayID: idToPG(id), ExpectedVersion: version,
		OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	}
}

func updateStayParams(command stay.UpdateCommand, now time.Time) generated.UpdateStayParams {
	patch := command.Patch
	return generated.UpdateStayParams{
		SetPlannedArrival:     patch.SetPlannedArrival,
		PlannedArrivalOn:      dateToPG(patch.PlannedArrivalOn),
		SetPlannedDeparture:   patch.SetPlannedDeparture,
		PlannedDepartureOn:    dateToPG(patch.PlannedDepartureOn),
		SetExpectedGuestCount: patch.SetExpectedGuestCount,
		ExpectedGuestCount:    patch.ExpectedGuestCount, UpdatedAt: timeToPG(now),
		StayID: idToPG(command.StayID), ExpectedVersion: command.ExpectedVersion,
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}
}

func mergedStayDates(
	current generated.GetAccessibleStayRow,
	patch stay.UpdatePatch,
) (string, string) {
	arrival := dateString(current.PlannedArrivalOn)
	departure := dateString(current.PlannedDepartureOn)
	if patch.SetPlannedArrival {
		arrival = patch.PlannedArrivalOn
	}
	if patch.SetPlannedDeparture {
		departure = patch.PlannedDepartureOn
	}
	return arrival, departure
}

func validMergedStay(current generated.GetAccessibleStayRow, patch stay.UpdatePatch) bool {
	arrival, departure := mergedStayDates(current, patch)
	arrivalDate, firstErr := stay.ParseCivilDate(arrival)
	departureDate, secondErr := stay.ParseCivilDate(departure)
	if firstErr != nil || secondErr != nil || !arrivalDate.Before(departureDate) {
		return false
	}
	return validExpectedGuests(current, patch)
}

func validExpectedGuests(current generated.GetAccessibleStayRow, patch stay.UpdatePatch) bool {
	if !patch.SetExpectedGuestCount {
		return true
	}
	if stay.Status(current.Status) != stay.StatusPreRegistered {
		return true
	}
	return patch.ExpectedGuestCount == current.VisitorCount
}

func stayPage(rows []generated.ListAccessibleStaysRow, limit int32) stay.Page {
	more := len(rows) > int(limit)
	if more {
		rows = rows[:limit]
	}
	items := make([]stay.Record, 0, len(rows))
	for _, row := range rows {
		items = append(items, stayFromList(row))
	}
	var cursor *stay.PageCursor
	if more {
		last := items[len(items)-1]
		cursor = &stay.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return stay.Page{Items: items, NextCursor: cursor}
}

// approvalFacts travels from the row that projected it to the record that has
// to emit it. CreateStay, UpdateStay and ApplyStayTransition do not project the
// three columns, so the facts come from the row the command already read to
// authorize itself, never from a default.
type approvalFacts struct {
	provenance stay.Provenance
	state      *string
	expiresAt  *time.Time
}

func factsFromLock(row generated.LockStayForCommandRow) approvalFacts {
	return approvalFacts{
		provenance: stay.Provenance(row.Provenance),
		state:      row.ApprovalState,
		expiresAt:  timePointer(row.ApprovalExpiresAt),
	}
}

func factsFromGet(row generated.GetAccessibleStayRow) approvalFacts {
	return approvalFacts{
		provenance: stay.Provenance(row.Provenance),
		state:      row.ApprovalState,
		expiresAt:  timePointer(row.ApprovalExpiresAt),
	}
}

func stayFromCreate(row generated.CreateStayRow) stay.Record {
	return stay.Record{
		Provenance: stay.ProvenanceAssisted,
		ID:         idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Status: stay.Status(row.Status), PlannedArrivalOn: dateString(row.PlannedArrivalOn),
		PlannedDepartureOn: dateString(row.PlannedDepartureOn),
		ExpectedGuestCount: row.ExpectedGuestCount, VisitorCount: 0,
		CheckedInAt: timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
		CancelledAt: timePointer(row.CancelledAt), NoShowAt: timePointer(row.NoShowAt),
		CancellationReasonCode: row.CancellationReasonCode, NoShowReasonCode: row.NoShowReasonCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func stayFromGet(row generated.GetAccessibleStayRow) stay.Record {
	return stay.Record{
		Provenance:        stay.Provenance(row.Provenance),
		ApprovalState:     row.ApprovalState,
		ApprovalExpiresAt: timePointer(row.ApprovalExpiresAt),
		ID:                idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Status: stay.Status(row.Status), PlannedArrivalOn: dateString(row.PlannedArrivalOn),
		PlannedDepartureOn: dateString(row.PlannedDepartureOn),
		ExpectedGuestCount: row.ExpectedGuestCount, VisitorCount: row.VisitorCount,
		CheckedInAt: timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
		CancelledAt: timePointer(row.CancelledAt), NoShowAt: timePointer(row.NoShowAt),
		CancellationReasonCode: row.CancellationReasonCode, NoShowReasonCode: row.NoShowReasonCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func stayFromList(row generated.ListAccessibleStaysRow) stay.Record {
	return stay.Record{
		Provenance:        stay.Provenance(row.Provenance),
		ApprovalState:     row.ApprovalState,
		ApprovalExpiresAt: timePointer(row.ApprovalExpiresAt),
		ID:                idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Status: stay.Status(row.Status), PlannedArrivalOn: dateString(row.PlannedArrivalOn),
		PlannedDepartureOn: dateString(row.PlannedDepartureOn),
		ExpectedGuestCount: row.ExpectedGuestCount, VisitorCount: row.VisitorCount,
		CheckedInAt: timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
		CancelledAt: timePointer(row.CancelledAt), NoShowAt: timePointer(row.NoShowAt),
		CancellationReasonCode: row.CancellationReasonCode, NoShowReasonCode: row.NoShowReasonCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func stayFromUpdate(
	row generated.UpdateStayRow,
	visitorCount int32,
	facts approvalFacts,
) stay.Record {
	return stay.Record{
		Provenance:        facts.provenance,
		ApprovalState:     facts.state,
		ApprovalExpiresAt: facts.expiresAt,
		ID:                idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Status: stay.Status(row.Status), PlannedArrivalOn: dateString(row.PlannedArrivalOn),
		PlannedDepartureOn: dateString(row.PlannedDepartureOn),
		ExpectedGuestCount: row.ExpectedGuestCount, VisitorCount: visitorCount,
		CheckedInAt: timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
		CancelledAt: timePointer(row.CancelledAt), NoShowAt: timePointer(row.NoShowAt),
		CancellationReasonCode: row.CancellationReasonCode, NoShowReasonCode: row.NoShowReasonCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func stayFromTransition(
	row generated.ApplyStayTransitionRow,
	visitorCount int32,
	facts approvalFacts,
) stay.Record {
	return stay.Record{
		Provenance:        facts.provenance,
		ApprovalState:     facts.state,
		ApprovalExpiresAt: facts.expiresAt,
		ID:                idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Status: stay.Status(row.Status), PlannedArrivalOn: dateString(row.PlannedArrivalOn),
		PlannedDepartureOn: dateString(row.PlannedDepartureOn),
		ExpectedGuestCount: row.ExpectedGuestCount, VisitorCount: visitorCount,
		CheckedInAt: timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
		CancelledAt: timePointer(row.CancelledAt), NoShowAt: timePointer(row.NoShowAt),
		CancellationReasonCode: row.CancellationReasonCode, NoShowReasonCode: row.NoShowReasonCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func visitorRecord(row generated.ListVisitorsForStayRow) stay.VisitorRecord {
	return stay.VisitorRecord{
		ID: idFromPG(row.ID), Role: stay.VisitorRole(row.Role),
		AgeBand: stay.AgeBand(row.AgeBand), ResidenceCountry: row.ResidenceCountry,
		ResidenceState: row.ResidenceState, ResidenceCityCode: row.ResidenceCityCode,
		Version: row.Version,
	}
}

func insertAssistedGroup(
	ctx context.Context,
	q generated.Querier,
	hashKey []byte,
	command stay.GroupCommand,
	now time.Time,
) (uuid.UUID, error) {
	submissionID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	hash, err := idempotency.RequestHash(hashKey, groupRequestHashValue(
		command.ClientSubmissionID, command.PrivacyNoticeVersion, command.Visitors,
	))
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	row, err := q.CreateAssistedGroupSubmission(ctx, generated.CreateAssistedGroupSubmissionParams{
		SubmissionID: idToPG(submissionID), ClientSubmissionID: idToPG(command.ClientSubmissionID),
		RequestHash: hash[:], PrivacyNoticeVersion: command.PrivacyNoticeVersion,
		SubmittedAt: timeToPG(now), StayID: idToPG(command.StayID),
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return uuid.Nil, stayMutationError(err)
	}
	return idFromPG(row.ID), nil
}

func insertAssistedVisitors(
	ctx context.Context,
	q generated.Querier,
	command stay.GroupCommand,
) error {
	for _, visitor := range command.Visitors {
		id, err := uuid.NewV7()
		if err != nil {
			return stay.ErrUnavailable
		}
		_, err = q.InsertAssistedVisitor(ctx, generated.InsertAssistedVisitorParams{
			VisitorID: idToPG(id), ClientID: idToPG(uuid.MustParse(visitor.ClientID)),
			VisitorRole: generated.CoreVisitorRole(visitor.Role), AgeBand: string(visitor.AgeBand),
			ResidenceCountry:  visitor.ResidenceCountry,
			ResidenceState:    optionalString(visitor.ResidenceState),
			ResidenceCityCode: optionalString(visitor.ResidenceCityCode),
			StayID:            idToPG(command.StayID),
			OidcIssuer:        command.Actor.Issuer, OidcSubject: command.Actor.Subject,
		})
		if err != nil {
			return stayMutationError(err)
		}
	}
	return nil
}

func updateAssistedStay(
	ctx context.Context,
	q generated.Querier,
	command stay.GroupCommand,
	locked generated.LockStayForCommandRow,
	now time.Time,
) (int64, error) {
	updated, err := q.UpdateStay(ctx, generated.UpdateStayParams{
		SetExpectedGuestCount: true, ExpectedGuestCount: int32(len(command.Visitors)),
		UpdatedAt: timeToPG(now), StayID: idToPG(command.StayID),
		ExpectedVersion: command.ExpectedVersion,
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return 0, stayUpdateError(err)
	}
	locked.Version = updated.Version
	transitioned, err := q.ApplyStayTransition(ctx, transitionParams(
		locked, command.Actor, stay.StatusPreRegistered, now, "", "",
	))
	if err != nil {
		return 0, stayUpdateError(err)
	}
	return transitioned.Version, nil
}

func insertInviteGroup(
	ctx context.Context,
	q generated.Querier,
	hashKey []byte,
	command stay.InviteGroupCommand,
	capability resolvedCapability,
	now time.Time,
) (uuid.UUID, error) {
	submissionID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	hash, err := idempotency.RequestHash(hashKey, groupRequestHashValue(
		command.ClientSubmissionID, command.PrivacyNoticeVersion, command.Visitors,
	))
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	row, err := q.CreateInviteGroupSubmission(ctx, generated.CreateInviteGroupSubmissionParams{
		SubmissionID: idToPG(submissionID), ClientSubmissionID: idToPG(command.ClientSubmissionID),
		RequestHash: hash[:], SubmittedAt: timeToPG(now),
		InviteID: idToPG(capability.inviteID), TokenHmac: capability.digest,
	})
	if err != nil {
		return uuid.Nil, stayMutationError(err)
	}
	return idFromPG(row.ID), nil
}

func insertInviteVisitors(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteGroupCommand,
	capability resolvedCapability,
	now time.Time,
) error {
	for _, visitor := range command.Visitors {
		id, err := uuid.NewV7()
		if err != nil {
			return stay.ErrUnavailable
		}
		_, err = q.InsertInviteVisitor(ctx, generated.InsertInviteVisitorParams{
			VisitorID: idToPG(id), ClientID: idToPG(uuid.MustParse(visitor.ClientID)),
			VisitorRole: generated.CoreVisitorRole(visitor.Role), AgeBand: string(visitor.AgeBand),
			ResidenceCountry:  visitor.ResidenceCountry,
			ResidenceState:    optionalString(visitor.ResidenceState),
			ResidenceCityCode: optionalString(visitor.ResidenceCityCode),
			InviteID:          idToPG(capability.inviteID), TokenHmac: capability.digest,
			Now: timeToPG(now),
		})
		if err != nil {
			return stayMutationError(err)
		}
	}
	return nil
}

func validateTransition(
	command stay.TransitionCommand,
	locked generated.LockStayForCommandRow,
	occurredAt time.Time,
) (stay.Status, error) {
	current := stay.Status(locked.Status)
	switch command.Kind {
	case stay.TransitionCheckIn:
		return validateCheckIn(current, locked.VisitorCount)
	case stay.TransitionCheckOut:
		return validateCheckOut(current, locked.CheckedInAt, occurredAt)
	case stay.TransitionCancel:
		return validateCancel(current, command, locked.ActorRole)
	case stay.TransitionNoShow:
		return validateNoShow(current, command, locked.PlannedArrivalOn, occurredAt)
	default:
		return "", stay.ErrInvalidTransition
	}
}

func validateCheckIn(current stay.Status, visitorCount int32) (stay.Status, error) {
	if visitorCount < 1 {
		return "", stay.ErrInvalidTransition
	}
	return current.Transition(stay.EventCheckIn)
}

func validateCheckOut(
	current stay.Status,
	checkedInAt pgtype.Timestamptz,
	occurredAt time.Time,
) (stay.Status, error) {
	if !checkedInAt.Valid || occurredAt.Before(checkedInAt.Time.UTC()) {
		return "", stay.ErrInvalidTransition
	}
	return current.Transition(stay.EventCheckOut)
}

func validateCancel(
	current stay.Status,
	command stay.TransitionCommand,
	actorRole string,
) (stay.Status, error) {
	cancel := stay.CancelCommand{
		Role: accommodation.Role(actorRole), Correction: command.Correction,
		Reason: stay.CancelReason(command.ReasonCode),
	}
	if err := cancel.Validate(current); err != nil {
		return "", err
	}
	return stay.StatusCancelled, nil
}

func validateNoShow(
	current stay.Status,
	command stay.TransitionCommand,
	arrival pgtype.Date,
	occurredAt time.Time,
) (stay.Status, error) {
	parsed, err := stay.ParseCivilDate(dateString(arrival))
	if err != nil {
		return "", err
	}
	if err := stay.ValidateNoShowTime(parsed, occurredAt); err != nil {
		return "", err
	}
	return current.Transition(stay.EventNoShow)
}

func operationForTransition(kind stay.TransitionKind) accommodation.Operation {
	operations := map[stay.TransitionKind]accommodation.Operation{
		stay.TransitionCheckIn:  accommodation.OperationCheckIn,
		stay.TransitionCheckOut: accommodation.OperationCheckOut,
		stay.TransitionCancel:   accommodation.OperationCancel,
		stay.TransitionNoShow:   accommodation.OperationNoShow,
	}
	return operations[kind]
}

func transitionEvents(kind stay.TransitionKind) (audit.Action, outbox.EventType) {
	actions := map[stay.TransitionKind]audit.Action{
		stay.TransitionCheckIn:  audit.ActionStayCheckedIn,
		stay.TransitionCheckOut: audit.ActionStayCheckedOut,
		stay.TransitionCancel:   audit.ActionStayCancelled,
		stay.TransitionNoShow:   audit.ActionStayNoShow,
	}
	events := map[stay.TransitionKind]outbox.EventType{
		stay.TransitionCheckIn:  outbox.EventStayCheckedIn,
		stay.TransitionCheckOut: outbox.EventStayCheckedOut,
		stay.TransitionCancel:   outbox.EventStayCancelled,
		stay.TransitionNoShow:   outbox.EventStayNoShow,
	}
	return actions[kind], events[kind]
}

func transitionParams(
	locked generated.LockStayForCommandRow,
	actor access.Principal,
	next stay.Status,
	occurredAt time.Time,
	reason string,
	kind stay.TransitionKind,
) generated.ApplyStayTransitionParams {
	params := generated.ApplyStayTransitionParams{
		NextStatus:  generated.CoreStayStatus(next),
		CheckedInAt: locked.CheckedInAt, CheckedOutAt: locked.CheckedOutAt,
		CancelledAt: locked.CancelledAt, NoShowAt: locked.NoShowAt,
		CancellationReasonCode: locked.CancellationReasonCode,
		NoShowReasonCode:       locked.NoShowReasonCode,
		UpdatedAt:              timeToPG(occurredAt), StayID: locked.ID,
		ExpectedVersion: locked.Version,
		OidcIssuer:      actor.Issuer, OidcSubject: actor.Subject,
	}
	applyTransitionOccurrence(&params, kind, occurredAt, reason)
	return params
}

func applyNoShowOccurrence(
	params *generated.ApplyStayTransitionParams,
	occurredAt time.Time,
	reason string,
) {
	params.NoShowAt = timeToPG(occurredAt)
	if reason != "" {
		params.NoShowReasonCode = &reason
	}
}

// Each transition stamps its own column; the reason code only accompanies the
// two transitions the contract declares one for.
func applyTransitionOccurrence(
	params *generated.ApplyStayTransitionParams,
	kind stay.TransitionKind,
	occurredAt time.Time,
	reason string,
) {
	switch kind {
	case stay.TransitionCheckIn:
		params.CheckedInAt = timeToPG(occurredAt)
	case stay.TransitionCheckOut:
		params.CheckedOutAt = timeToPG(occurredAt)
	case stay.TransitionCancel:
		params.CancelledAt = timeToPG(occurredAt)
		params.CancellationReasonCode = &reason
	case stay.TransitionNoShow:
		applyNoShowOccurrence(params, occurredAt, reason)
	}
}

func (r *StayRepository) applyRateLimit(
	ctx context.Context,
	scope string,
	token string,
	subject string,
	limit int,
	now time.Time,
) error {
	key, ok := r.store.core.RateLimitKeys.Key(r.store.core.RateLimitKeys.CurrentVersion)
	if !ok {
		return stay.ErrUnavailable
	}
	window := now.Truncate(r.store.core.RateLimitWindow)
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.IncrementRateLimit(ctx, generated.IncrementRateLimitParams{
		Scope: scope, SubjectHmac: rateLimitDigest(key, scope, token, subject),
		SubjectKeyVersion: r.store.core.RateLimitKeys.CurrentVersion,
		WindowStartedAt:   timeToPG(window),
		ExpiresAt:         timeToPG(window.Add(2 * r.store.core.RateLimitWindow)),
	})
	if err != nil {
		return stay.ErrUnavailable
	}
	if row.RequestCount > int32(limit) {
		return stay.ErrRateLimited
	}
	return nil
}

func rateLimitDigest(key []byte, scope, token, subject string) []byte {
	inviteID, err := capabilityInviteID(token)
	capability := inviteID.String()
	if err != nil {
		digest := sha256.Sum256([]byte(token))
		capability = base64.RawURLEncoding.EncodeToString(digest[:])
	}
	return keyedDigest(key, "rate-limit:"+scope, capability+"\x00"+subject)
}

func capabilityInviteID(token string) (uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 48 {
		return uuid.Nil, stay.ErrNotFound
	}
	id, err := uuid.FromBytes(decoded[:16])
	if err != nil || id == uuid.Nil {
		return uuid.Nil, stay.ErrNotFound
	}
	return id, nil
}

func validCapabilityRow(
	row generated.GetInviteForCapabilityRow,
	now time.Time,
	allowConsumed bool,
) bool {
	if !validCapabilityLifetime(row, now) {
		return false
	}
	if accommodation.Status(row.AccommodationStatus) != accommodation.StatusActive {
		return false
	}
	return validCapabilityUse(row, allowConsumed)
}

func validCapabilityLifetime(row generated.GetInviteForCapabilityRow, now time.Time) bool {
	return row.ExpiresAt.Valid && row.ExpiresAt.Time.After(now) && !row.RevokedAt.Valid
}

// A pre-registered stay may only be revisited as an already spent capability;
// an open stay may still be used while uses remain.
func validCapabilityUse(row generated.GetInviteForCapabilityRow, allowConsumed bool) bool {
	status := stay.Status(row.StayStatus)
	if status == stay.StatusPreRegistered {
		return allowConsumed && exhaustedInvite(row)
	}
	if !openStayStatus(status) {
		return false
	}
	return allowConsumed || !exhaustedInvite(row)
}

// max_uses nulo significa uso ilimitado, e só o convite por acomodação pode
// tê-lo (ADR-039): invites_target_valid mantém o convite de estadia com o
// limite obrigatório. O caminho de estadia nunca chega aqui com nulo, mas o
// tipo passou a admiti-lo e "ilimitado" precisa ser explícito em vez de virar
// zero por acidente, o que declararia todo convite esgotado.
func exhaustedInvite(row generated.GetInviteForCapabilityRow) bool {
	return row.MaxUses != nil && row.UseCount >= *row.MaxUses
}

func openStayStatus(status stay.Status) bool {
	return status == stay.StatusDraft || status == stay.StatusInvited
}

func inviteContext(row generated.GetInviteForCapabilityRow) stay.InviteContext {
	return stay.InviteContext{
		AccommodationName:    row.AccommodationName,
		PlannedArrivalOn:     dateString(row.PlannedArrivalOn),
		PlannedDepartureOn:   dateString(row.PlannedDepartureOn),
		ExpectedGuestCount:   row.ExpectedGuestCount,
		PrivacyNoticeVersion: row.PrivacyNoticeVersion,
	}
}

type stayEventSpec struct {
	actor          access.Principal
	organizationID uuid.UUID
	action         audit.Action
	eventType      outbox.EventType
	stayID         uuid.UUID
	version        int64
	requestID      string
	fields         []audit.ChangedField
	now            time.Time
}

func (s *Store) recordStayEvents(
	ctx context.Context,
	q generated.Querier,
	spec stayEventSpec,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: spec.actor.Issuer,
		actorSubject: spec.actor.Subject, organization: spec.organizationID,
		action: spec.action, entityType: audit.EntityStay, entityID: spec.stayID,
		requestID: spec.requestID, changedFields: spec.fields, version: spec.version,
		aggregateType: outbox.AggregateStay, eventType: spec.eventType, now: spec.now,
	})
}

func (s *Store) recordStayUpdate(
	ctx context.Context,
	q generated.Querier,
	command stay.UpdateCommand,
	current generated.GetAccessibleStayRow,
	result stay.Record,
) error {
	now := result.UpdatedAt
	err := s.recordStayEvents(ctx, q, stayEventSpec{
		actor: command.Actor, organizationID: idFromPG(current.OrganizationID),
		action: audit.ActionStayUpdated, eventType: outbox.EventStayUpdated,
		stayID: result.ID, version: result.Version, requestID: command.RequestID,
		fields: stayChangedFields(command.Patch), now: now,
	})
	if err != nil {
		return err
	}
	if command.Patch.SetPlannedArrival ||
		command.Patch.SetPlannedDeparture ||
		command.Patch.SetExpectedGuestCount {
		return insertPresenceEvent(ctx, q, result.ID, result.Version)
	}
	return nil
}

func (s *Store) recordInviteSubmission(
	ctx context.Context,
	q generated.Querier,
	command stay.InviteGroupCommand,
	capability resolvedCapability,
	result stay.SubmissionAccepted,
	now time.Time,
) error {
	if err := s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorInvite, actorIssuer: "urn:cumuru:invite",
		actorSubject: capability.inviteID.String(),
		organization: idFromPG(capability.row.OrganizationID),
		action:       audit.ActionStayGroupSubmitted, entityType: audit.EntityStay,
		entityID: idFromPG(capability.row.StayID), requestID: command.RequestID,
		changedFields: []audit.ChangedField{audit.FieldExpectedGuests, audit.FieldStatus},
		version:       result.Version, aggregateType: outbox.AggregateStay,
		eventType: outbox.EventStayGroupSubmitted, now: now,
	}); err != nil {
		return err
	}
	return insertPresenceEvent(
		ctx, q, idFromPG(capability.row.StayID), result.Version,
	)
}

func insertPresenceEvent(
	ctx context.Context,
	q generated.Querier,
	stayID uuid.UUID,
	version int64,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	event := outbox.Event{
		ID: id, AggregateType: outbox.AggregateStay,
		AggregateID: stayID, AggregateVersion: version,
		Type: outbox.EventStayPresenceRecalculationRequested,
	}
	if err := event.Validate(); err != nil {
		return err
	}
	return insertOutbox(ctx, q, event)
}

func stayChangedFields(patch stay.UpdatePatch) []audit.ChangedField {
	fields := make([]audit.ChangedField, 0, 3)
	if patch.SetPlannedArrival {
		fields = append(fields, audit.FieldPlannedArrival)
	}
	if patch.SetPlannedDeparture {
		fields = append(fields, audit.FieldPlannedDeparture)
	}
	if patch.SetExpectedGuestCount {
		fields = append(fields, audit.FieldExpectedGuests)
	}
	return fields
}

func stayCommandError(
	ctx context.Context,
	q generated.Querier,
	actor access.Principal,
	id uuid.UUID,
	version int64,
	lockError error,
) error {
	if !errors.Is(lockError, pgx.ErrNoRows) {
		return stay.ErrUnavailable
	}
	current, err := q.GetAccessibleStay(ctx, stayKey(id, actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return stay.ErrNotFound
	}
	if err != nil {
		return stay.ErrUnavailable
	}
	if current.Version != version {
		return stay.ErrPreconditionFailed
	}
	return stay.ErrConflict
}

func stayQueryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return stay.ErrNotFound
	}
	return stay.ErrUnavailable
}

func stayUpdateError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return stay.ErrPreconditionFailed
	}
	return stayMutationError(err)
}

var knownStayErrors = []error{
	stay.ErrNotFound, stay.ErrConflict, stay.ErrPreconditionFailed,
	stay.ErrInviteConsumed, stay.ErrRateLimited,
}

func firstKnownError(err error, known []error) (error, bool) {
	for _, candidate := range known {
		if errors.Is(err, candidate) {
			return candidate, true
		}
	}
	return nil, false
}

func stayMutationError(err error) error {
	if errors.Is(err, idempotency.ErrProcessing) {
		return err
	}
	if errors.Is(err, errIdempotencyConflict) || isUniqueViolation(err) {
		return stay.ErrConflict
	}
	if known, ok := firstKnownError(err, knownStayErrors); ok {
		return known
	}
	return stay.ErrUnavailable
}

func idToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: id != uuid.Nil}
}

func idFromPG(value pgtype.UUID) uuid.UUID {
	return uuid.UUID(value.Bytes)
}

func timeToPG(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

func dateToPG(value string) pgtype.Date {
	parsed, err := time.Parse("2006-01-02", value)
	return pgtype.Date{Time: parsed, Valid: err == nil && value != ""}
}

func dateString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
