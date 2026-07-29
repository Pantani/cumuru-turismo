package store

import (
	"context"
	"encoding/json"
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
)

type QuestionnaireRepository struct {
	store *Store
}

func NewQuestionnaireRepository(store *Store) *QuestionnaireRepository {
	return &QuestionnaireRepository{store: store}
}

func (r *QuestionnaireRepository) List(
	ctx context.Context,
	_ access.Principal,
	page questionnaire.PageRequest,
) (questionnaire.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListQuestionnaires(ctx, generated.ListQuestionnairesParams{
		CursorCreatedAt: optionalPGTime(page.CursorCreatedAt),
		CursorID:        optionalPGUUID(page.CursorID), PageLimit: page.Limit + 1,
	})
	if err != nil {
		return questionnaire.Page{}, questionnaire.ErrUnavailable
	}
	return questionnairePage(rows, page.Limit), nil
}

func (r *QuestionnaireRepository) Create(
	ctx context.Context,
	command questionnaire.CreateCommand,
) (result questionnaire.VersionMutation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := questionnaireIdempotency(
			command.Actor, idempotency.OperationCreateQuestionnaire,
			command.ID, command.IdempotencyKey, createQuestionnaireHash(command), now,
		)
		value, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.create(ctx, q, command, now)
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

func (r *QuestionnaireRepository) create(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.CreateCommand,
	now time.Time,
) (storedMutation, error) {
	_, err := q.CreateQuestionnaire(ctx, generated.CreateQuestionnaireParams{
		ID: pgUUID(command.ID), StableKey: command.StableKey,
		Name: command.Name, CreatedAt: pgTime(now),
	})
	if err != nil {
		return storedMutation{}, questionnaireStoreError(err)
	}
	actor := currentActorDigest(r.store, command.Actor)
	row, err := q.CreateQuestionnaireVersion(ctx, generated.CreateQuestionnaireVersionParams{
		ID: pgUUID(command.VersionID), QuestionnaireID: pgUUID(command.ID),
		VersionNumber: 1, Title: command.Title,
		PrivacyNoticeVersion: command.PrivacyNoticeVersion,
		LastEditorHmac:       actor.sum, LastEditorKeyVersion: actor.version,
		CreatedAt: pgTime(now),
	})
	if err != nil {
		return storedMutation{}, questionnaireStoreError(err)
	}
	result := createVersionMutation(row)
	if err := r.store.recordQuestionnaireEvent(ctx, q, questionnaireEvent{
		actor: command.Actor, action: audit.ActionQuestionnaireCreated,
		entityType: audit.EntityQuestionnaire, entityID: command.ID,
		version: 1, requestID: command.RequestID,
		fields:    []audit.ChangedField{audit.FieldDefinition},
		aggregate: outbox.AggregateQuestionnaire,
		eventType: outbox.EventQuestionnaireCreated, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(201, command.VersionID, result, map[string]string{
		"Location": "/api/v1/questionnaire-versions/" + command.VersionID.String(),
		"ETag":     entityTag(result.Revision),
	})
}

func (r *QuestionnaireRepository) Get(
	ctx context.Context,
	_ access.Principal,
	id uuid.UUID,
) (questionnaire.Questionnaire, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.GetQuestionnaire(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return questionnaire.Questionnaire{}, questionnaire.ErrNotFound
	}
	if err != nil {
		return questionnaire.Questionnaire{}, questionnaire.ErrUnavailable
	}
	return questionnaireFromRow(row), nil
}

func (r *QuestionnaireRepository) ListVersions(
	ctx context.Context,
	_ access.Principal,
	id uuid.UUID,
	page questionnaire.VersionPageRequest,
) (questionnaire.VersionPage, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListQuestionnaireVersions(ctx, generated.ListQuestionnaireVersionsParams{
		QuestionnaireID: pgUUID(id), CursorVersionNumber: optionalInt32(page.CursorVersionNumber),
		CursorID: optionalPGUUID(page.CursorID), PageLimit: page.Limit + 1,
	})
	if err != nil {
		return questionnaire.VersionPage{}, questionnaire.ErrUnavailable
	}
	if len(rows) == 0 {
		if _, getErr := r.store.queries.GetQuestionnaire(ctx, pgUUID(id)); errors.Is(getErr, pgx.ErrNoRows) {
			return questionnaire.VersionPage{}, questionnaire.ErrNotFound
		}
	}
	return questionnaireVersionPage(rows, page.Limit), nil
}

func (r *QuestionnaireRepository) Clone(
	ctx context.Context,
	command questionnaire.CloneCommand,
) (result questionnaire.VersionMutation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := questionnaireIdempotency(
			command.Actor, idempotency.OperationCloneQuestionnaire,
			command.QuestionnaireID, command.IdempotencyKey, cloneQuestionnaireHash(command), now,
		)
		value, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.clone(ctx, q, command, now)
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

func (r *QuestionnaireRepository) clone(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.CloneCommand,
	now time.Time,
) (storedMutation, error) {
	if _, err := q.LockQuestionnaire(ctx, pgUUID(command.QuestionnaireID)); err != nil {
		return storedMutation{}, questionnaireQueryError(err)
	}
	source, err := r.readVersion(ctx, q, command.SourceVersionID)
	if err != nil || source.QuestionnaireID != command.QuestionnaireID {
		return storedMutation{}, questionnaireQueryError(err)
	}
	number, err := q.GetNextQuestionnaireVersionNumber(ctx, pgUUID(command.QuestionnaireID))
	if err != nil {
		return storedMutation{}, questionnaire.ErrUnavailable
	}
	actor := currentActorDigest(r.store, command.Actor)
	row, err := q.CreateQuestionnaireVersion(ctx, generated.CreateQuestionnaireVersionParams{
		ID: pgUUID(command.NewVersionID), QuestionnaireID: pgUUID(command.QuestionnaireID),
		VersionNumber: number, Title: source.Title, Introduction: source.Introduction,
		PrivacyNoticeVersion: source.PrivacyNoticeVersion,
		LastEditorHmac:       actor.sum, LastEditorKeyVersion: actor.version, CreatedAt: pgTime(now),
	})
	if err != nil {
		return storedMutation{}, questionnaireStoreError(err)
	}
	if err := r.cloneContent(ctx, q, command.NewVersionID, source); err != nil {
		return storedMutation{}, err
	}
	result := createVersionMutation(row)
	if err := r.store.recordQuestionnaireEvent(ctx, q, questionnaireEvent{
		actor: command.Actor, action: audit.ActionQuestionnaireCloned,
		entityType: audit.EntityQuestionnaireVersion, entityID: command.NewVersionID,
		version: result.Revision, requestID: command.RequestID,
		fields:    []audit.ChangedField{audit.FieldDefinition},
		aggregate: outbox.AggregateQuestionnaireVersion,
		eventType: outbox.EventQuestionnaireCloned, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(201, command.NewVersionID, result, map[string]string{
		"Location": "/api/v1/questionnaire-versions/" + command.NewVersionID.String(),
		"ETag":     entityTag(result.Revision),
	})
}

func (r *QuestionnaireRepository) GetVersion(
	ctx context.Context,
	_ access.Principal,
	id uuid.UUID,
) (questionnaire.Version, error) {
	var result questionnaire.Version
	err := r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		var readErr error
		result, readErr = r.readVersion(ctx, q, id)
		return readErr
	})
	return result, err
}

func (r *QuestionnaireRepository) UpdateVersion(
	ctx context.Context,
	command questionnaire.UpdateCommand,
) (result questionnaire.Version, err error) {
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		actor := currentActorDigest(r.store, command.Actor)
		row, updateErr := q.UpdateDraftQuestionnaireVersion(ctx, generated.UpdateDraftQuestionnaireVersionParams{
			Title: command.Definition.Title, Introduction: command.Definition.Introduction,
			PrivacyNoticeVersion: command.Definition.PrivacyNoticeVersion,
			LastEditorHmac:       actor.sum, LastEditorKeyVersion: actor.version,
			UpdatedAt: pgTime(r.store.currentTime()), ID: pgUUID(command.VersionID),
			ExpectedRevision: command.ExpectedVersion,
		})
		if updateErr != nil {
			return r.versionCommandError(ctx, q, command.VersionID, command.ExpectedVersion, updateErr)
		}
		if updateErr = r.replaceContent(ctx, q, command.VersionID, command.Definition); updateErr != nil {
			return updateErr
		}
		result, updateErr = r.readVersion(ctx, q, command.VersionID)
		if updateErr != nil {
			return updateErr
		}
		return r.store.recordQuestionnaireEvent(ctx, q, questionnaireEvent{
			actor: command.Actor, action: audit.ActionQuestionnaireUpdated,
			entityType: audit.EntityQuestionnaireVersion, entityID: command.VersionID,
			version: row.Revision, requestID: command.RequestID,
			fields:    []audit.ChangedField{audit.FieldDefinition},
			aggregate: outbox.AggregateQuestionnaireVersion,
			eventType: outbox.EventQuestionnaireUpdated, now: row.UpdatedAt.Time.UTC(),
		})
	})
	return result, err
}

func (r *QuestionnaireRepository) GetPublished(
	ctx context.Context,
	stableKey string,
) (result questionnaire.Published, err error) {
	err = r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		row, readErr := q.GetPublishedQuestionnaireVersionByStableKey(ctx, stableKey)
		if readErr != nil {
			return questionnaireQueryError(readErr)
		}
		version, readErr := r.readVersion(ctx, q, uuid.UUID(row.ID.Bytes))
		if readErr != nil {
			return readErr
		}
		catalog, readErr := q.GetQuestionnaire(ctx, row.QuestionnaireID)
		if readErr != nil {
			return questionnaireQueryError(readErr)
		}
		result = publishedFrom(catalog, version)
		return nil
	})
	return result, err
}

func (r *QuestionnaireRepository) readVersion(
	ctx context.Context,
	q generated.Querier,
	id uuid.UUID,
) (questionnaire.Version, error) {
	row, err := q.GetQuestionnaireVersion(ctx, pgUUID(id))
	if err != nil {
		return questionnaire.Version{}, questionnaireQueryError(err)
	}
	questions, err := q.ListQuestionsForVersion(ctx, pgUUID(id))
	if err != nil {
		return questionnaire.Version{}, questionnaire.ErrUnavailable
	}
	options, err := q.ListQuestionOptionsForVersion(ctx, pgUUID(id))
	if err != nil {
		return questionnaire.Version{}, questionnaire.ErrUnavailable
	}
	mapped, err := questionsFromRows(questions, options)
	if err != nil {
		return questionnaire.Version{}, questionnaire.ErrUnavailable
	}
	requirements, err := q.ListConsentRequirementsForVersion(ctx, pgUUID(id))
	if err != nil {
		return questionnaire.Version{}, questionnaire.ErrUnavailable
	}
	return versionFromParts(row, mapped, requirementsFromRows(requirements)), nil
}

func (r *QuestionnaireRepository) replaceContent(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	definition questionnaire.Definition,
) error {
	if err := q.DeleteDraftQuestionnaireContent(ctx, pgUUID(versionID)); err != nil {
		return questionnaire.ErrUnavailable
	}
	if err := q.DeleteDraftConsentRequirements(ctx, pgUUID(versionID)); err != nil {
		return questionnaire.ErrUnavailable
	}
	if err := r.insertQuestions(ctx, q, versionID, definition.Questions); err != nil {
		return err
	}
	return insertRequirements(ctx, q, versionID, definition.ConsentRequirements)
}

func (r *QuestionnaireRepository) insertQuestions(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	questions []questionnaire.Question,
) error {
	for _, question := range questions {
		if err := insertQuestion(ctx, q, versionID, question); err != nil {
			return err
		}
		for _, option := range question.Options {
			if err := q.InsertQuestionOption(ctx, generated.InsertQuestionOptionParams{
				ID: pgUUID(option.ID), QuestionID: pgUUID(question.ID),
				Value: option.Value, Label: option.Label, DisplayOrder: option.DisplayOrder,
			}); err != nil {
				return questionnaire.ErrUnavailable
			}
		}
	}
	return nil
}

func insertQuestion(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	question questionnaire.Question,
) error {
	validation, err := encodeJSON(question.Validation, true)
	if err != nil {
		return questionnaire.ErrUnavailable
	}
	visibility, err := encodeJSON(question.VisibilityRule, true)
	if err != nil {
		return questionnaire.ErrUnavailable
	}
	err = q.InsertQuestion(ctx, generated.InsertQuestionParams{
		ID: pgUUID(question.ID), QuestionnaireVersionID: pgUUID(versionID),
		StableKey: question.StableKey, Prompt: question.Prompt, HelpText: question.HelpText,
		AnswerType: generated.SurveyAnswerType(question.AnswerType), Required: question.Required,
		DataClassification: generated.SurveyDataClassification(question.DataClassification),
		PurposeCode:        question.PurposeCode, RetentionPolicyCode: question.RetentionPolicyCode,
		AnalyticsKey:             question.AnalyticsKey,
		PublicAggregationAllowed: question.PublicAggregationAllowed,
		MinimumPublicCell:        question.MinimumPublicCell, Validation: validation,
		VisibilityRule: visibility, DisplayOrder: question.DisplayOrder,
	})
	if err != nil {
		return questionnaire.ErrUnavailable
	}
	return nil
}

func insertRequirements(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	requirements []questionnaire.ConsentRequirement,
) error {
	for _, requirement := range requirements {
		err := q.InsertConsentRequirement(ctx, generated.InsertConsentRequirementParams{
			QuestionnaireVersionID: pgUUID(versionID),
			PurposeCode:            requirement.PurposeCode, NoticeVersion: requirement.NoticeVersion,
			Prompt: requirement.Prompt, RequiredForAnswers: requirement.RequiredForAnswer,
			DisplayOrder: requirement.DisplayOrder,
		})
		if err != nil {
			return questionnaire.ErrUnavailable
		}
	}
	return nil
}

func (r *QuestionnaireRepository) cloneContent(
	ctx context.Context,
	q generated.Querier,
	versionID uuid.UUID,
	source questionnaire.Version,
) error {
	cloned := source.Definition()
	for index := range cloned.Questions {
		id, err := uuid.NewV7()
		if err != nil {
			return questionnaire.ErrUnavailable
		}
		cloned.Questions[index].ID = id
		for optionIndex := range cloned.Questions[index].Options {
			optionID, optionErr := uuid.NewV7()
			if optionErr != nil {
				return questionnaire.ErrUnavailable
			}
			cloned.Questions[index].Options[optionIndex].ID = optionID
		}
	}
	return r.replaceContent(ctx, q, versionID, cloned)
}

func questionnairePage(rows []generated.SurveyQuestionnaire, limit int32) questionnaire.Page {
	items := rows
	var next *questionnaire.PageCursor
	if len(items) > int(limit) {
		cursorRow := items[limit-1]
		next = &questionnaire.PageCursor{
			CreatedAt: cursorRow.CreatedAt.Time.UTC(), ID: uuid.UUID(cursorRow.ID.Bytes),
		}
		items = items[:limit]
	}
	result := make([]questionnaire.Questionnaire, 0, len(items))
	for _, row := range items {
		result = append(result, questionnaireFromRow(row))
	}
	return questionnaire.Page{Items: result, NextCursor: next}
}

func questionnaireVersionPage(
	rows []generated.ListQuestionnaireVersionsRow,
	limit int32,
) questionnaire.VersionPage {
	items := rows
	var next *questionnaire.VersionCursor
	if len(items) > int(limit) {
		cursorRow := items[limit-1]
		next = &questionnaire.VersionCursor{
			VersionNumber: cursorRow.VersionNumber, ID: uuid.UUID(cursorRow.ID.Bytes),
		}
		items = items[:limit]
	}
	result := make([]questionnaire.VersionSummary, 0, len(items))
	for _, row := range items {
		result = append(result, listVersionSummary(row))
	}
	return questionnaire.VersionPage{Items: result, NextCursor: next}
}
