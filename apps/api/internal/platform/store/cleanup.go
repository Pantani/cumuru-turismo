package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxExpiredRecordCleanupBatch int32 = 1000

// The batch is bounded in both sweeps for the same reason: an absent cutoff or
// an uncapped batch would turn the cleanup into a transaction that holds the
// whole table.
func validCleanupBounds(cutoff time.Time, batchSize int32) bool {
	return !cutoff.IsZero() &&
		batchSize >= 1 && batchSize <= maxExpiredRecordCleanupBatch
}

type ExpiredRecordCleanupResult struct {
	IdempotencyRecords int64
	RateLimitBuckets   int64
}

func (s *Store) CleanupExpiredOperationalRecords(
	ctx context.Context,
	cutoff time.Time,
	batchSize int32,
) (ExpiredRecordCleanupResult, error) {
	if !validCleanupBounds(cutoff, batchSize) {
		return ExpiredRecordCleanupResult{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	result, err := s.queries.CleanupExpiredOperationalRecords(
		ctx,
		generated.CleanupExpiredOperationalRecordsParams{
			ExpiredBefore: pgTime(cutoff),
			BatchSize:     batchSize,
		},
	)
	if err != nil {
		return ExpiredRecordCleanupResult{}, ErrUnavailable
	}
	return ExpiredRecordCleanupResult{
		IdempotencyRecords: result.IdempotencyRecords,
		RateLimitBuckets:   result.RateLimitBuckets,
	}, nil
}

// ExpireAccommodationAccessRequests is the sweep that stops inaction from
// becoming indefinite retention: erasing the contact only on rejection would
// make "not deciding" the easiest way to keep the data forever. The overdue
// request becomes 'expired' and loses name, e-mail and phone in the same
// transaction that writes the state, and the fact that it existed remains
// (ADR-042).
//
// There is no decider: nobody decided, the clock ran out. The decision
// constraint forbids an actor on 'expired' precisely so the sweep cannot invent
// a decision that never happened.
func (s *Store) ExpireAccommodationAccessRequests(
	ctx context.Context,
	cutoff time.Time,
	batchSize int32,
) (int64, error) {
	if !validCleanupBounds(cutoff, batchSize) {
		return 0, ErrUnavailable
	}
	// One identifier per sweep, so every row a run expired correlates in the
	// trail. It is the only "request id" the sweep has.
	runID, err := uuid.NewV7()
	if err != nil {
		return 0, ErrUnavailable
	}
	// inTransactionWithOptions bounds its own ctx, but the closure below uses
	// this one. An HTTP handler would inherit the request deadline; the worker
	// inherits none, so without this bound the sweep UPDATE and the audit
	// inserts would hang until the pool gives up. Same reason
	// CleanupExpiredOperationalRecords above applies it.
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var expired int64
	err = s.inTransaction(ctx, func(q generated.Querier) error {
		rows, sweepErr := q.ExpireAccommodationAccessRequests(
			ctx, generated.ExpireAccommodationAccessRequestsParams{
				ExpiredAt: pgTime(cutoff), Cutoff: pgTime(cutoff), BatchSize: batchSize,
			},
		)
		if sweepErr != nil {
			return ErrUnavailable
		}
		expired = int64(len(rows))
		return s.recordAccessRequestExpiries(ctx, q, rows, runID.String(), cutoff)
	})
	return expired, err
}

// The purge and the audit row land in the same transaction, and that ordering is
// the point: an event that fails to validate aborts the transaction, and an
// aborted transaction means the purge never happened — rather than the purge
// happening with no trail.
//
// The outbox event is left out here, and only here. The worker grant
// deliberately withholds SELECT on `version`, so the sweep has no way to read
// the aggregate version; an outbox event needs it, and inventing a number would
// make the (aggregate, version, type) identity either lie or collide. The audit
// row does not depend on a version and is what retention requires recording.
func (s *Store) recordAccessRequestExpiries(
	ctx context.Context,
	q generated.Querier,
	rows []pgtype.UUID,
	runID string,
	now time.Time,
) error {
	for _, row := range rows {
		event, err := s.newAuditEvent(accessRequestExpiryEvent(
			uuid.UUID(row.Bytes), runID,
		))
		if err != nil {
			return ErrUnavailable
		}
		if err := insertAudit(ctx, q, event, now); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func accessRequestExpiryEvent(requestID uuid.UUID, runID string) eventSpec {
	return eventSpec{
		actorType: audit.ActorSystem, actorIssuer: systemActorIssuer,
		actorSubject: accessRequestExpiryActor,
		action:       audit.ActionAccessRequestExpired,
		entityType:   audit.EntityAccessRequest, entityID: requestID,
		requestID:     runID,
		changedFields: []audit.ChangedField{audit.FieldStatus},
	}
}

const accessRequestExpiryActor = "access-request-expiry"
