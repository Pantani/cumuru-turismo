package store

import (
	"context"
	"crypto/hmac"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type actorDigest struct {
	version string
	sum     []byte
}

func currentActorDigest(store *Store, actor access.Principal) actorDigest {
	values := digests(store.phase2.ActorKeys, "actor", actorValue(actor.Issuer, actor.Subject))
	if len(values) == 0 {
		return actorDigest{}
	}
	return actorDigest{version: values[0].version, sum: values[0].sum}
}

func reviewerDistinctFromEditor(
	store *Store,
	version questionnaire.Version,
	reviewer access.Principal,
) bool {
	values := digests(
		store.phase2.ActorKeys,
		"actor",
		actorValue(reviewer.Issuer, reviewer.Subject),
	)
	for _, value := range values {
		if value.version == version.LastEditorKeyVersion {
			return !hmac.Equal(value.sum, version.LastEditorHMAC)
		}
	}
	return false
}

func questionnaireIdempotency(
	actor access.Principal,
	operation idempotency.Operation,
	resourceID uuid.UUID,
	key string,
	request any,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(actor.Issuer, actor.Subject),
		operation:  operation, resourceID: resourceID,
		key: key, request: request, now: now,
	}
}

func createQuestionnaireHash(command questionnaire.CreateCommand) any {
	return struct {
		ID                   uuid.UUID `json:"id"`
		VersionID            uuid.UUID `json:"version_id"`
		StableKey            string    `json:"stable_key"`
		Name                 string    `json:"name"`
		Title                string    `json:"title"`
		PrivacyNoticeVersion string    `json:"privacy_notice_version"`
	}{
		ID: command.ID, VersionID: command.VersionID,
		StableKey: command.StableKey, Name: command.Name,
		Title: command.Title, PrivacyNoticeVersion: command.PrivacyNoticeVersion,
	}
}

func cloneQuestionnaireHash(command questionnaire.CloneCommand) any {
	return struct {
		QuestionnaireID uuid.UUID `json:"questionnaire_id"`
		SourceVersionID uuid.UUID `json:"source_version_id"`
		NewVersionID    uuid.UUID `json:"new_version_id"`
	}{
		QuestionnaireID: command.QuestionnaireID,
		SourceVersionID: command.SourceVersionID,
		NewVersionID:    command.NewVersionID,
	}
}

func transitionQuestionnaireHash(command questionnaire.TransitionCommand) any {
	return struct {
		VersionID  uuid.UUID                `json:"version_id"`
		Expected   int64                    `json:"expected_version"`
		Transition questionnaire.Transition `json:"transition"`
		ReasonCode string                   `json:"reason_code,omitempty"`
	}{
		VersionID: command.VersionID, Expected: command.ExpectedVersion,
		Transition: command.Transition, ReasonCode: command.ReasonCode,
	}
}

func optionalPGTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

func optionalPGUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != uuid.Nil}
}

func optionalInt32(value int32) *int32 {
	if value == 0 {
		return nil
	}
	return &value
}

func questionnaireQueryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return questionnaire.ErrNotFound
	}
	if err == nil {
		return nil
	}
	return questionnaire.ErrUnavailable
}

func questionnaireStoreError(err error) error {
	switch {
	case errors.Is(err, errIdempotencyConflict), isUniqueViolation(err):
		return questionnaire.ErrConflict
	case errors.Is(err, questionnaire.ErrInvalidInput),
		errors.Is(err, questionnaire.ErrNotFound),
		errors.Is(err, questionnaire.ErrConflict),
		errors.Is(err, questionnaire.ErrPreconditionFailed),
		errors.Is(err, questionnaire.ErrCapabilityInvalid),
		errors.Is(err, questionnaire.ErrRateLimited):
		return err
	default:
		return questionnaire.ErrUnavailable
	}
}

func (r *QuestionnaireRepository) versionCommandError(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	expected int64,
	err error,
) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return questionnaireStoreError(err)
	}
	current, getErr := q.GetQuestionnaireVersion(ctx, pgUUID(versionID))
	if errors.Is(getErr, pgx.ErrNoRows) {
		return questionnaire.ErrNotFound
	}
	if getErr != nil {
		return questionnaire.ErrUnavailable
	}
	if current.Revision != expected {
		return questionnaire.ErrPreconditionFailed
	}
	return questionnaire.ErrConflict
}

type questionnaireEvent struct {
	actor      access.Principal
	action     audit.Action
	entityType audit.EntityType
	entityID   uuid.UUID
	version    int64
	requestID  string
	fields     []audit.ChangedField
	aggregate  outbox.AggregateType
	eventType  outbox.EventType
	now        time.Time
}

func (s *Store) recordQuestionnaireEvent(
	ctx context.Context,
	q generated.Querier,
	spec questionnaireEvent,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: spec.actor.Issuer,
		actorSubject: spec.actor.Subject, action: spec.action,
		entityType: spec.entityType, entityID: spec.entityID,
		requestID: spec.requestID, changedFields: spec.fields,
		version: spec.version, aggregateType: spec.aggregate,
		eventType: spec.eventType, purpose: audit.PurposeQuestionnaireGovernance,
		now: spec.now,
	})
}
