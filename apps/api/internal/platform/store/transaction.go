package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var errIdempotencyConflict = errors.New("idempotency conflict")

type transactionWork func(generated.Querier) error

func (s *Store) inTransaction(ctx context.Context, work transactionWork) error {
	return s.inTransactionWithOptions(ctx, pgx.TxOptions{}, work)
}

func (s *Store) inReadOnlyTransaction(ctx context.Context, work transactionWork) error {
	return s.inTransactionWithOptions(ctx, readOnlyTransactionOptions(), work)
}

func readOnlyTransactionOptions() pgx.TxOptions {
	return pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
}

func (s *Store) inTransactionWithOptions(
	ctx context.Context,
	options pgx.TxOptions,
	work transactionWork,
) error {
	if s.pool == nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		return ErrUnavailable
	}
	if err := work(generated.New(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}

type versionedDigest struct {
	version string
	sum     []byte
}

type idempotencySpec struct {
	actorValue string
	operation  idempotency.Operation
	resourceID uuid.UUID
	key        string
	request    any
	now        time.Time
}

type storedMutation struct {
	status     int
	headers    map[string]string
	body       []byte
	resourceID uuid.UUID
}

type idempotencyResult struct {
	response storedMutation
	replayed bool
}

type mutationWork func() (storedMutation, error)

func (s *Store) runIdempotent(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	work mutationWork,
) (idempotencyResult, error) {
	requestHash, err := idempotency.RequestHash(spec.request)
	if err != nil {
		return idempotencyResult{}, errIdempotencyConflict
	}
	return s.replayOrExecute(ctx, q, spec, requestHash[:], work)
}

func (s *Store) replayOrExecute(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
	work mutationWork,
) (idempotencyResult, error) {
	result, found, err := s.findIdempotency(ctx, q, spec, requestHash)
	if err != nil || found {
		return result, err
	}
	replay, claimed, err := s.claimOrReplay(ctx, q, spec, requestHash)
	if err != nil || !claimed {
		return replay, err
	}
	return s.executeClaimed(ctx, q, spec, requestHash, work)
}

// claimOrReplay races for the claim. Losing the race is not an error: the
// winner's record is replayed instead.
func (s *Store) claimOrReplay(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
) (idempotencyResult, bool, error) {
	claimed, err := s.claimIdempotency(ctx, q, spec, requestHash)
	if errors.Is(err, pgx.ErrNoRows) {
		result, _, findErr := s.findIdempotency(ctx, q, spec, requestHash)
		return result, false, findErr
	}
	if err != nil || claimed.State != "processing" {
		return idempotencyResult{}, false, ErrUnavailable
	}
	return idempotencyResult{}, true, nil
}

func (s *Store) executeClaimed(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
	work mutationWork,
) (idempotencyResult, error) {
	response, err := work()
	if err != nil {
		return idempotencyResult{}, err
	}
	if err := s.completeIdempotency(ctx, q, spec, requestHash, response); err != nil {
		return idempotencyResult{}, err
	}
	return idempotencyResult{response: response}, nil
}

func (s *Store) findIdempotency(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
) (idempotencyResult, bool, error) {
	actors := digests(s.phase2.ActorKeys, "actor", spec.actorValue)
	keys := digests(s.phase2.IdempotencyKeys, "idempotency-key", spec.key)
	for _, actor := range actors {
		result, found, err := s.lockUnderActor(ctx, q, spec, requestHash, actor, keys)
		if found || err != nil {
			return result, found, err
		}
	}
	return idempotencyResult{}, false, nil
}

// Every key version is probed so a key rotation does not turn a replay into a
// second execution.
func (s *Store) lockUnderActor(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
	actor versionedDigest,
	keys []versionedDigest,
) (idempotencyResult, bool, error) {
	for _, key := range keys {
		row, err := q.LockIdempotencyKey(ctx, generated.LockIdempotencyKeyParams{
			ActorKeyHmac: actor.sum, OperationKey: string(spec.operation),
			ResourceID: pgUUID(spec.resourceID), IdempotencyKeyHmac: key.sum,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return idempotencyResult{}, false, ErrUnavailable
		}
		result, err := existingIdempotency(row, requestHash, spec.now)
		return result, true, err
	}
	return idempotencyResult{}, false, nil
}

func (s *Store) claimIdempotency(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
) (generated.PlatformIdempotencyRecord, error) {
	actor := digests(s.phase2.ActorKeys, "actor", spec.actorValue)[0]
	key := digests(s.phase2.IdempotencyKeys, "idempotency-key", spec.key)[0]
	return q.ClaimIdempotencyKey(ctx, generated.ClaimIdempotencyKeyParams{
		ActorKeyHmac: actor.sum, ActorKeyVersion: actor.version,
		OperationKey: string(spec.operation), ResourceID: pgUUID(spec.resourceID),
		IdempotencyKeyHmac: key.sum, IdempotencyKeyVersion: key.version,
		RequestHash: requestHash, ExpiresAt: pgTime(spec.now.Add(s.phase2.IdempotencyTTL)),
	})
}

// A replay is only honoured for the same request body within the retention
// window; a divergent body under the same key is a conflict, never a replay.
func replayable(row generated.PlatformIdempotencyRecord, requestHash []byte, now time.Time) bool {
	if !bytes.Equal(row.RequestHash, requestHash) {
		return false
	}
	return row.ExpiresAt.Valid && row.ExpiresAt.Time.After(now)
}

func existingIdempotency(
	row generated.PlatformIdempotencyRecord,
	requestHash []byte,
	now time.Time,
) (idempotencyResult, error) {
	if !replayable(row, requestHash, now) {
		return idempotencyResult{}, errIdempotencyConflict
	}
	if row.State == "processing" {
		return idempotencyResult{}, idempotency.NewProcessingError(time.Second)
	}
	if row.State != "completed" {
		return idempotencyResult{}, errIdempotencyConflict
	}
	response, err := storedResponse(row)
	if err != nil {
		return idempotencyResult{}, ErrUnavailable
	}
	return idempotencyResult{response: response, replayed: true}, nil
}

func storedResponse(row generated.PlatformIdempotencyRecord) (storedMutation, error) {
	if row.ResponseStatus == nil || !row.ResponseResourceID.Valid {
		return storedMutation{}, ErrUnavailable
	}
	headers := make(map[string]string)
	if len(row.ResponseHeaders) > 0 {
		if err := json.Unmarshal(row.ResponseHeaders, &headers); err != nil {
			return storedMutation{}, ErrUnavailable
		}
	}
	return storedMutation{
		status: int(*row.ResponseStatus), headers: headers,
		body:       append([]byte(nil), row.ResponseBody...),
		resourceID: uuid.UUID(row.ResponseResourceID.Bytes),
	}, nil
}

func (s *Store) completeIdempotency(
	ctx context.Context,
	q generated.Querier,
	spec idempotencySpec,
	requestHash []byte,
	response storedMutation,
) error {
	actor := digests(s.phase2.ActorKeys, "actor", spec.actorValue)[0]
	key := digests(s.phase2.IdempotencyKeys, "idempotency-key", spec.key)[0]
	headers, err := json.Marshal(response.headers)
	if err != nil {
		return ErrUnavailable
	}
	status := int32(response.status)
	_, err = q.CompleteIdempotencyKey(ctx, generated.CompleteIdempotencyKeyParams{
		ResponseStatus: &status, ResponseHeaders: headers, ResponseBody: response.body,
		ResponseResourceID: pgUUID(response.resourceID), CompletedAt: pgTime(spec.now),
		ActorKeyHmac: actor.sum, OperationKey: string(spec.operation),
		ResourceID: pgUUID(spec.resourceID), IdempotencyKeyHmac: key.sum,
		RequestHash: requestHash,
	})
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func digests(keyring config.KeyringConfig, purpose, value string) []versionedDigest {
	versions := make([]string, 0, len(keyring.Keys))
	versions = append(versions, keyring.CurrentVersion)
	for version := range keyring.Keys {
		if version != keyring.CurrentVersion {
			versions = append(versions, version)
		}
	}
	sort.Strings(versions[1:])
	result := make([]versionedDigest, 0, len(versions))
	for _, version := range versions {
		result = append(result, versionedDigest{
			version: version,
			sum:     keyedDigest(keyring.Keys[version], purpose, value),
		})
	}
	return result
}

func keyedDigest(key []byte, purpose, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func actorValue(issuer, subject string) string {
	return issuer + "\x00" + subject
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: id != uuid.Nil}
}

func pgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

func (s *Store) currentTime() time.Time {
	return s.now().UTC()
}

func entityTag(version int64) string {
	return `"` + strconv.FormatInt(version, 10) + `"`
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

type eventSpec struct {
	actorType     audit.ActorType
	actorIssuer   string
	actorSubject  string
	organization  uuid.UUID
	action        audit.Action
	entityType    audit.EntityType
	entityID      uuid.UUID
	requestID     string
	changedFields []audit.ChangedField
	version       int64
	aggregateType outbox.AggregateType
	eventType     outbox.EventType
	purpose       audit.PurposeCode
	now           time.Time
}

func (s *Store) recordEvents(
	ctx context.Context,
	q generated.Querier,
	spec eventSpec,
) error {
	auditEvent, err := s.newAuditEvent(spec)
	if err != nil {
		return ErrUnavailable
	}
	if err := insertAudit(ctx, q, auditEvent, spec.now); err != nil {
		return ErrUnavailable
	}
	outboxEvent, err := newOutboxEvent(spec)
	if err != nil {
		return ErrUnavailable
	}
	if err := insertOutbox(ctx, q, outboxEvent); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *Store) pseudonymizeActor(issuer, subject string) (audit.ActorDigest, error) {
	key, ok := s.phase2.ActorKeys.Key(s.phase2.ActorKeys.CurrentVersion)
	if !ok {
		return audit.ActorDigest{}, audit.ErrInvalidActor
	}
	hasher, err := audit.NewActorHasher(s.phase2.ActorKeys.CurrentVersion, key)
	if err != nil {
		return audit.ActorDigest{}, err
	}
	return hasher.Pseudonymize(issuer, subject)
}

func (s *Store) newAuditEvent(spec eventSpec) (audit.Event, error) {
	actor, err := s.pseudonymizeActor(spec.actorIssuer, spec.actorSubject)
	if err != nil {
		return audit.Event{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return audit.Event{}, err
	}
	purpose := spec.purpose
	if purpose == "" {
		purpose = audit.PurposeStayOperation
	}
	event := audit.Event{
		ID: id, Actor: actor, ActorType: spec.actorType,
		OrganizationID: spec.organization, Action: spec.action,
		EntityType: spec.entityType, EntityID: spec.entityID,
		PurposeCode: purpose, RequestID: spec.requestID,
		ChangedFields: spec.changedFields, Outcome: audit.OutcomeSucceeded,
	}
	return event, event.Validate()
}

func insertAudit(
	ctx context.Context,
	q generated.Querier,
	event audit.Event,
	now time.Time,
) error {
	version := event.Actor.Version
	purpose := string(event.PurposeCode)
	requestID := event.RequestID
	fields := make([]string, 0, len(event.ChangedFields))
	for _, field := range event.ChangedFields {
		fields = append(fields, string(field))
	}
	err := q.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		ID: pgUUID(event.ID), OccurredAt: pgTime(now),
		ActorSubjectHmac: event.Actor.Sum[:], ActorHmacKeyVersion: &version,
		ActorType: string(event.ActorType), OrganizationID: pgUUID(event.OrganizationID),
		Action: string(event.Action), EntityType: string(event.EntityType),
		EntityID: pgUUID(event.EntityID), PurposeCode: &purpose,
		RequestID: &requestID, ChangedFields: fields, Outcome: string(event.Outcome),
	})
	return err
}

func newOutboxEvent(spec eventSpec) (outbox.Event, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return outbox.Event{}, err
	}
	event := outbox.Event{
		ID: id, AggregateType: spec.aggregateType,
		AggregateID: spec.entityID, AggregateVersion: spec.version,
		Type: spec.eventType,
	}
	return event, event.Validate()
}

func insertOutbox(ctx context.Context, q generated.Querier, event outbox.Event) error {
	err := q.InsertOutboxEvent(ctx, generated.InsertOutboxEventParams{
		ID: pgUUID(event.ID), AggregateType: string(event.AggregateType),
		AggregateID: pgUUID(event.AggregateID), AggregateVersion: event.AggregateVersion,
		EventType: string(event.Type),
	})
	return err
}
