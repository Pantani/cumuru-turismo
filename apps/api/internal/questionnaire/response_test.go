package questionnaire

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateResponseRequiresExactConsent(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	command := validSubmissionFixture(definition)
	if err := ValidateResponse(definition, command); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	command.Consents[0].NoticeVersion = "other"
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("mismatched notice accepted")
	}
}

func TestValidateResponseRejectsAnswerForDeniedPurpose(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	command := validSubmissionFixture(definition)
	command.Consents[0].Granted = false
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("answer for denied purpose accepted")
	}
}

func TestValidateResponseAcceptsDeclineWithoutConsent(t *testing.T) {
	t.Parallel()
	command := SubmissionCommand{Participation: ParticipationDeclined}
	if err := ValidateResponse(Definition{}, command); err != nil {
		t.Fatalf("decline rejected: %v", err)
	}
	command.Answers = []AnswerInput{{QuestionID: mustV7(t), Value: json.RawMessage(`true`)}}
	if err := ValidateResponse(Definition{}, command); err == nil {
		t.Fatal("decline with answer accepted")
	}
}

func TestVisibilityUsesPriorStableKeyOnly(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	second := Question{
		ID: mustV7(t), StableKey: "follow_up", Prompt: "Conte mais",
		AnswerType: AnswerBoolean, Required: true,
		DataClassification: ClassificationPersonal,
		PurposeCode:        "tourism_planning", RetentionPolicyCode: "survey_prototype_v1",
		DisplayOrder: 2, Options: []Option{},
		VisibilityRule: &VisibilityRule{All: []Condition{{
			Question: "first_visit", Operator: "equals", Value: json.RawMessage(`"yes"`),
		}}},
	}
	definition.Questions = append(definition.Questions, second)
	command := validSubmissionFixture(definition)
	command.Answers = append(command.Answers, AnswerInput{
		QuestionID: second.ID, Value: json.RawMessage(`true`),
	})
	if err := ValidateResponse(definition, command); err != nil {
		t.Fatalf("visible answer rejected: %v", err)
	}
	command.Answers[0].Value = json.RawMessage(`"no"`)
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("hidden answer accepted")
	}
}

func TestFreeTextLimitAppliesBeforeEncryptionInUTF8Bytes(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	definition.Questions[0].AnswerType = AnswerShortText
	definition.Questions[0].Options = nil
	definition.Questions[0].Required = false
	definition.Questions[0].AnalyticsKey = nil
	definition.Questions[0].MinimumPublicCell = nil
	definition.Questions[0].PublicAggregationAllowed = false
	command := validSubmissionFixture(definition)
	command.Answers[0].Value = json.RawMessage(`"` + strings.Repeat("😀", 600) + `"`)
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("free text larger than the UTF-8 byte budget was accepted")
	}
}

func TestValidateResponseAcceptsEveryAnswerType(t *testing.T) {
	t.Parallel()
	minimum, maximum, selections := int32(1), int32(5), int32(2)
	tests := []struct {
		name       string
		answerType AnswerType
		value      string
		options    []Option
		validation *ValidationDefinition
	}{
		{name: "short text", answerType: AnswerShortText, value: `"sol"`},
		{name: "long text", answerType: AnswerLongText, value: `"praia tranquila"`},
		{name: "single choice", answerType: AnswerSingleChoice, value: `"yes"`, options: responseOptions(t)},
		{name: "multiple choice", answerType: AnswerMultipleChoice, value: `["yes","no"]`, options: responseOptions(t), validation: &ValidationDefinition{MaxSelections: &selections}},
		{name: "boolean", answerType: AnswerBoolean, value: `true`},
		{name: "integer range", answerType: AnswerIntegerRange, value: `3`, validation: &ValidationDefinition{Minimum: &minimum, Maximum: &maximum}},
		{name: "rating", answerType: AnswerRating, value: `5`, validation: &ValidationDefinition{Minimum: &minimum, Maximum: &maximum}},
		{name: "date", answerType: AnswerDate, value: `"2026-07-28"`},
		{name: "state city", answerType: AnswerStateCity, value: `{"state":"BA","city_code":"2925501"}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			definition := responseDefinition(t, test.answerType, test.options, test.validation)
			command := validSubmissionFixture(definition)
			command.Answers[0].Value = json.RawMessage(test.value)
			if err := ValidateResponse(definition, command); err != nil {
				t.Fatalf("%s response rejected: %v", test.answerType, err)
			}
		})
	}
}

func TestValidateResponseRejectsUnknownOptionAndDuplicateQuestion(t *testing.T) {
	t.Parallel()
	definition := responseDefinition(t, AnswerSingleChoice, responseOptions(t), nil)
	command := validSubmissionFixture(definition)
	command.Answers[0].Value = json.RawMessage(`"not-published"`)
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("unknown option accepted")
	}
	command = validSubmissionFixture(definition)
	command.Answers = append(command.Answers, command.Answers[0])
	if err := ValidateResponse(definition, command); err == nil {
		t.Fatal("duplicate question accepted")
	}
}

func responseDefinition(
	t *testing.T,
	answerType AnswerType,
	options []Option,
	validation *ValidationDefinition,
) Definition {
	t.Helper()
	definition := validFixtureDefinition(t)
	question := &definition.Questions[0]
	question.AnswerType = answerType
	question.Options = options
	question.Validation = validation
	if answerType == AnswerShortText || answerType == AnswerLongText {
		question.Required = false
		question.AnalyticsKey = nil
		question.MinimumPublicCell = nil
		question.PublicAggregationAllowed = false
	}
	return definition
}

func responseOptions(t *testing.T) []Option {
	t.Helper()
	return []Option{
		{ID: mustV7(t), Value: "yes", Label: "Sim", DisplayOrder: 1},
		{ID: mustV7(t), Value: "no", Label: "Não", DisplayOrder: 2},
	}
}

func validSubmissionFixture(definition Definition) SubmissionCommand {
	return SubmissionCommand{
		Participation: ParticipationSubmitted,
		Answers: []AnswerInput{{
			QuestionID: definition.Questions[0].ID, Value: json.RawMessage(`"yes"`),
		}},
		Consents: []ConsentDecisionInput{{
			PurposeCode: "tourism_planning", NoticeVersion: "notice-v1", Granted: true,
		}},
	}
}
