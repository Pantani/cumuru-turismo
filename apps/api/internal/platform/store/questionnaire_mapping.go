package store

import (
	"encoding/json"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func questionnaireFromRow(row generated.SurveyQuestionnaire) questionnaire.Questionnaire {
	return questionnaire.Questionnaire{
		ID: uuid.UUID(row.ID.Bytes), StableKey: row.StableKey,
		Name: row.Name, CreatedAt: row.CreatedAt.Time.UTC(),
	}
}

func versionSummaryFromRow(row generated.SurveyQuestionnaireVersion) questionnaire.VersionSummary {
	return questionnaire.VersionSummary{
		ID: uuid.UUID(row.ID.Bytes), QuestionnaireID: uuid.UUID(row.QuestionnaireID.Bytes),
		VersionNumber: row.VersionNumber, Revision: row.Revision,
		Status: questionnaire.VersionStatus(row.Status), Title: row.Title,
		PrivacyNoticeVersion: row.PrivacyNoticeVersion,
		CreatedAt:            row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func versionMutationFromRow(row generated.SurveyQuestionnaireVersion) questionnaire.VersionMutation {
	return questionnaire.VersionMutation{
		ID: uuid.UUID(row.ID.Bytes), QuestionnaireID: uuid.UUID(row.QuestionnaireID.Bytes),
		VersionNumber: row.VersionNumber, Revision: row.Revision,
		Status: questionnaire.VersionStatus(row.Status),
	}
}

func versionFromParts(
	row generated.SurveyQuestionnaireVersion,
	questions []questionnaire.Question,
	requirements []questionnaire.ConsentRequirement,
) questionnaire.Version {
	return questionnaire.Version{
		VersionSummary: versionSummaryFromRow(row),
		Introduction:   row.Introduction, Questions: questions,
		ConsentRequirements:  requirements,
		SubmittedForReviewAt: nullableTime(row.SubmittedForReviewAt),
		PrivacyReviewedAt:    nullableTime(row.PrivacyReviewedAt),
		PublishedAt:          nullableTime(row.PublishedAt), RetiredAt: nullableTime(row.RetiredAt),
		LastEditorHMAC:       append([]byte(nil), row.LastEditorHmac...),
		LastEditorKeyVersion: row.LastEditorKeyVersion,
	}
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func questionsFromRows(
	rows []generated.SurveyQuestion,
	options []generated.SurveyQuestionOption,
) ([]questionnaire.Question, error) {
	byQuestion := make(map[uuid.UUID][]questionnaire.Option)
	for _, row := range options {
		id := uuid.UUID(row.QuestionID.Bytes)
		byQuestion[id] = append(byQuestion[id], optionFromRow(row))
	}
	result := make([]questionnaire.Question, 0, len(rows))
	for _, row := range rows {
		question, err := questionFromRow(row, byQuestion[uuid.UUID(row.ID.Bytes)])
		if err != nil {
			return nil, err
		}
		result = append(result, question)
	}
	return result, nil
}

func questionFromRow(
	row generated.SurveyQuestion,
	options []questionnaire.Option,
) (questionnaire.Question, error) {
	validation, err := decodeValidation(row.Validation)
	if err != nil {
		return questionnaire.Question{}, err
	}
	visibility, err := decodeVisibility(row.VisibilityRule)
	if err != nil {
		return questionnaire.Question{}, err
	}
	return questionnaire.Question{
		ID: uuid.UUID(row.ID.Bytes), StableKey: row.StableKey,
		Prompt: row.Prompt, HelpText: row.HelpText,
		AnswerType: questionnaire.AnswerType(row.AnswerType), Required: row.Required,
		DataClassification: questionnaire.DataClassification(row.DataClassification),
		PurposeCode:        row.PurposeCode, RetentionPolicyCode: row.RetentionPolicyCode,
		AnalyticsKey: row.AnalyticsKey, PublicAggregationAllowed: row.PublicAggregationAllowed,
		MinimumPublicCell: row.MinimumPublicCell, Validation: validation,
		VisibilityRule: visibility, DisplayOrder: row.DisplayOrder, Options: options,
	}, nil
}

func optionFromRow(row generated.SurveyQuestionOption) questionnaire.Option {
	return questionnaire.Option{
		ID: uuid.UUID(row.ID.Bytes), Value: row.Value,
		Label: row.Label, DisplayOrder: row.DisplayOrder,
	}
}

func requirementsFromRows(
	rows []generated.SurveyConsentRequirement,
) []questionnaire.ConsentRequirement {
	result := make([]questionnaire.ConsentRequirement, 0, len(rows))
	for _, row := range rows {
		result = append(result, questionnaire.ConsentRequirement{
			PurposeCode: row.PurposeCode, NoticeVersion: row.NoticeVersion,
			Prompt: row.Prompt, RequiredForAnswer: row.RequiredForAnswers,
			DisplayOrder: row.DisplayOrder,
		})
	}
	return result
}

func decodeValidation(content []byte) (*questionnaire.ValidationDefinition, error) {
	if len(content) == 0 || string(content) == "{}" {
		return nil, nil
	}
	var value questionnaire.ValidationDefinition
	if err := json.Unmarshal(content, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func decodeVisibility(content []byte) (*questionnaire.VisibilityRule, error) {
	if len(content) == 0 || string(content) == "null" {
		return nil, nil
	}
	var value questionnaire.VisibilityRule
	if err := json.Unmarshal(content, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func encodeJSON(value any, nullable bool) ([]byte, error) {
	if nullable && value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func createVersionMutation(row generated.CreateQuestionnaireVersionRow) questionnaire.VersionMutation {
	return questionnaire.VersionMutation{
		ID: uuid.UUID(row.ID.Bytes), QuestionnaireID: uuid.UUID(row.QuestionnaireID.Bytes),
		VersionNumber: row.VersionNumber, Revision: row.Revision,
		Status: questionnaire.VersionStatus(row.Status),
	}
}

func listVersionSummary(row generated.ListQuestionnaireVersionsRow) questionnaire.VersionSummary {
	return questionnaire.VersionSummary{
		ID: uuid.UUID(row.ID.Bytes), QuestionnaireID: uuid.UUID(row.QuestionnaireID.Bytes),
		VersionNumber: row.VersionNumber, Revision: row.Revision,
		Status: questionnaire.VersionStatus(row.Status), Title: row.Title,
		PrivacyNoticeVersion: row.PrivacyNoticeVersion,
		CreatedAt:            row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func publicQuestion(question questionnaire.Question) questionnaire.PublicQuestion {
	return questionnaire.PublicQuestion{
		ID: question.ID, StableKey: question.StableKey, Prompt: question.Prompt,
		HelpText: question.HelpText, AnswerType: question.AnswerType,
		Required: question.Required, PurposeCode: question.PurposeCode,
		Validation:     question.Validation,
		VisibilityRule: question.VisibilityRule, DisplayOrder: question.DisplayOrder,
		Options: question.Options,
	}
}

func publishedFrom(
	catalog generated.SurveyQuestionnaire,
	version questionnaire.Version,
) questionnaire.Published {
	questions := make([]questionnaire.PublicQuestion, 0, len(version.Questions))
	for _, question := range version.Questions {
		questions = append(questions, publicQuestion(question))
	}
	return questionnaire.Published{
		ID: version.ID, QuestionnaireID: version.QuestionnaireID,
		StableKey: catalog.StableKey, VersionNumber: version.VersionNumber,
		Revision: version.Revision, Title: version.Title,
		Introduction: version.Introduction, PrivacyNoticeVersion: version.PrivacyNoticeVersion,
		Questions: questions, ConsentRequirements: version.ConsentRequirements,
	}
}
