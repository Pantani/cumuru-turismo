package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type transitionSpec struct {
	operation idempotency.Operation
	action    audit.Action
	eventType outbox.EventType
}

func (r *QuestionnaireRepository) Transition(
	ctx context.Context,
	command questionnaire.TransitionCommand,
) (result questionnaire.VersionMutation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec, specErr := questionnaireTransitionSpec(command.Transition)
		if specErr != nil {
			return questionnaire.ErrInvalidInput
		}
		idempotent := questionnaireIdempotency(
			command.Actor, spec.operation, command.VersionID,
			command.IdempotencyKey, transitionQuestionnaireHash(command), now,
		)
		value, runErr := r.store.runIdempotent(ctx, q, idempotent, func() (storedMutation, error) {
			return r.transition(ctx, q, command, spec, now)
		})
		if runErr != nil {
			return questionnaireStoreError(runErr)
		}
		if decodeErr := json.Unmarshal(value.response.body, &result); decodeErr != nil {
			return questionnaire.ErrUnavailable
		}
		replayed = value.replayed
		return nil
	})
	return result, replayed, err
}

func (r *QuestionnaireRepository) transition(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.TransitionCommand,
	spec transitionSpec,
	now time.Time,
) (storedMutation, error) {
	current, err := r.transitionableVersion(ctx, q, command)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := r.applyTransition(ctx, q, current, command, now)
	if err != nil {
		return storedMutation{}, r.versionCommandError(
			ctx, q, command.VersionID, command.ExpectedVersion, err,
		)
	}
	result := versionMutationFromRow(row)
	if err := r.store.recordQuestionnaireEvent(ctx, q, questionnaireEvent{
		actor: command.Actor, action: spec.action,
		entityType: audit.EntityQuestionnaireVersion, entityID: command.VersionID,
		version: result.Revision, requestID: command.RequestID,
		fields:    []audit.ChangedField{audit.FieldStatus},
		aggregate: outbox.AggregateQuestionnaireVersion,
		eventType: spec.eventType, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, command.VersionID, result, map[string]string{
		"ETag": entityTag(result.Revision),
	})
}

// A review transition must come from someone other than the last editor, and
// the version content itself must still satisfy the target state.
func (r *QuestionnaireRepository) transitionableVersion(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.TransitionCommand,
) (questionnaire.Version, error) {
	current, err := r.readVersion(ctx, q, command.VersionID)
	if err != nil {
		return current, err
	}
	if current.Revision != command.ExpectedVersion {
		return current, questionnaire.ErrPreconditionFailed
	}
	if requiresDistinctReviewer(command.Transition) &&
		!reviewerDistinctFromEditor(r.store, current, command.Actor) {
		return current, questionnaire.ErrConflict
	}
	err = validateTransitionContent(current, command.Transition, r.store.phase3.Enabled)
	return current, err
}

func requiresDistinctReviewer(transition questionnaire.Transition) bool {
	return transition == questionnaire.TransitionRequestChanges ||
		transition == questionnaire.TransitionApprove
}

// Publishing retires the current published version first, so a questionnaire
// never has two published versions at once.
func publishQuestionnaireVersion(
	ctx context.Context,
	q generated.Querier,
	current questionnaire.Version,
	command questionnaire.TransitionCommand,
	now time.Time,
) (generated.SurveyQuestionnaireVersion, error) {
	if err := q.RetireCurrentPublishedVersion(ctx, generated.RetireCurrentPublishedVersionParams{
		TransitionedAt: pgTime(now), QuestionnaireID: pgUUID(current.QuestionnaireID),
	}); err != nil {
		return generated.SurveyQuestionnaireVersion{}, err
	}
	return q.PublishQuestionnaireVersion(ctx, generated.PublishQuestionnaireVersionParams{
		TransitionedAt: pgTime(now), ID: pgUUID(command.VersionID),
		ExpectedRevision: command.ExpectedVersion,
	})
}

func (r *QuestionnaireRepository) applyTransition(
	ctx context.Context,
	q generated.Querier,
	current questionnaire.Version,
	command questionnaire.TransitionCommand,
	now time.Time,
) (generated.SurveyQuestionnaireVersion, error) {
	actor := currentActorDigest(r.store, command.Actor)
	if command.Transition == questionnaire.TransitionPublish {
		return publishQuestionnaireVersion(ctx, q, current, command, now)
	}
	return applyReviewTransition(ctx, q, command, actor, now)
}

func applyReviewTransition(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.TransitionCommand,
	actor actorDigest,
	now time.Time,
) (generated.SurveyQuestionnaireVersion, error) {
	version := actor.version
	switch command.Transition {
	case questionnaire.TransitionSubmitReview:
		return q.SubmitQuestionnaireVersionReview(ctx, generated.SubmitQuestionnaireVersionReviewParams{
			SubmittedByHmac: actor.sum, SubmittedByKeyVersion: &version,
			TransitionedAt: pgTime(now), ID: pgUUID(command.VersionID),
			ExpectedRevision: command.ExpectedVersion,
		})
	case questionnaire.TransitionRequestChanges:
		reason := command.ReasonCode
		return q.RequestQuestionnaireVersionChanges(ctx, generated.RequestQuestionnaireVersionChangesParams{
			ReviewedByHmac: actor.sum, ReviewedByKeyVersion: &version,
			TransitionedAt: pgTime(now), ChangeReasonCode: &reason,
			ID: pgUUID(command.VersionID), ExpectedRevision: command.ExpectedVersion,
		})
	case questionnaire.TransitionApprove:
		return q.ApproveQuestionnaireVersion(ctx, generated.ApproveQuestionnaireVersionParams{
			ReviewedByHmac: actor.sum, ReviewedByKeyVersion: &version,
			TransitionedAt: pgTime(now), ID: pgUUID(command.VersionID),
			ExpectedRevision: command.ExpectedVersion,
		})
	case questionnaire.TransitionRetire:
		return q.RetireQuestionnaireVersion(ctx, generated.RetireQuestionnaireVersionParams{
			TransitionedAt: pgTime(now), ID: pgUUID(command.VersionID),
			ExpectedRevision: command.ExpectedVersion,
		})
	default:
		return generated.SurveyQuestionnaireVersion{}, questionnaire.ErrInvalidInput
	}
}

func validateTransitionContent(
	current questionnaire.Version,
	transition questionnaire.Transition,
	freeTextEnabled bool,
) error {
	switch transition {
	case questionnaire.TransitionSubmitReview:
		return questionnaire.ValidateDefinition(current.Definition())
	case questionnaire.TransitionApprove, questionnaire.TransitionPublish:
		return questionnaire.ValidatePublishable(current.Definition(), freeTextEnabled)
	default:
		return nil
	}
}

func questionnaireTransitionSpec(transition questionnaire.Transition) (transitionSpec, error) {
	specs := map[questionnaire.Transition]transitionSpec{
		questionnaire.TransitionSubmitReview: {
			operation: idempotency.OperationSubmitQuestionnaireReview,
			action:    audit.ActionQuestionnaireReview,
			eventType: outbox.EventQuestionnaireReviewSubmitted,
		},
		questionnaire.TransitionRequestChanges: {
			operation: idempotency.OperationRequestQuestionnaireChanges,
			action:    audit.ActionQuestionnaireChanges,
			eventType: outbox.EventQuestionnaireChangesRequested,
		},
		questionnaire.TransitionApprove: {
			operation: idempotency.OperationApproveQuestionnaire,
			action:    audit.ActionQuestionnaireApproved,
			eventType: outbox.EventQuestionnaireApproved,
		},
		questionnaire.TransitionPublish: {
			operation: idempotency.OperationPublishQuestionnaire,
			action:    audit.ActionQuestionnairePublished,
			eventType: outbox.EventQuestionnairePublished,
		},
		questionnaire.TransitionRetire: {
			operation: idempotency.OperationRetireQuestionnaire,
			action:    audit.ActionQuestionnaireRetired,
			eventType: outbox.EventQuestionnaireRetired,
		},
	}
	spec, ok := specs[transition]
	if !ok {
		return transitionSpec{}, errors.New("unknown questionnaire transition")
	}
	return spec, nil
}

func (r *QuestionnaireRepository) EraseExpiredFreeText(
	ctx context.Context,
	cutoff time.Time,
) (int32, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	count, err := r.store.queries.EraseExpiredSurveyFreeText(ctx, pgTime(cutoff))
	if err != nil {
		return 0, questionnaire.ErrUnavailable
	}
	return count, nil
}

var _ questionnaire.Repository = (*QuestionnaireRepository)(nil)

var _ = uuid.Nil
var _ = pgx.ErrNoRows
