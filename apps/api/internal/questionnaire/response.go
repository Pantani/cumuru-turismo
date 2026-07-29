package questionnaire

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
)

func ValidateResponse(definition Definition, command SubmissionCommand) error {
	if command.Participation == ParticipationDeclined {
		return validateDeclinedResponse(command)
	}
	return validateSubmittedResponse(definition, command)
}

func validateDeclinedResponse(command SubmissionCommand) error {
	if len(command.Answers) != 0 || len(command.Consents) != 0 {
		return ErrInvalidInput
	}
	return nil
}

func validateSubmittedResponse(definition Definition, command SubmissionCommand) error {
	consents, ok := exactConsents(definition.ConsentRequirements, command.Consents)
	if !ok {
		return ErrInvalidInput
	}
	answers := answerMap(command.Answers)
	if len(answers) != len(command.Answers) {
		return ErrInvalidInput
	}
	stableAnswers := make(map[string]json.RawMessage, len(answers))
	for _, question := range definition.Questions {
		if !validResponseQuestion(question, answers, stableAnswers, consents) {
			return ErrInvalidInput
		}
	}
	if len(answers) != 0 {
		return ErrInvalidInput
	}
	return nil
}

func exactConsents(
	requirements []ConsentRequirement,
	inputs []ConsentDecisionInput,
) (map[string]bool, bool) {
	if len(requirements) != len(inputs) {
		return nil, false
	}
	inputByPurpose := make(map[string]ConsentDecisionInput, len(inputs))
	for _, input := range inputs {
		if _, duplicate := inputByPurpose[input.PurposeCode]; duplicate {
			return nil, false
		}
		inputByPurpose[input.PurposeCode] = input
	}
	result := make(map[string]bool, len(requirements))
	for _, requirement := range requirements {
		input, ok := inputByPurpose[requirement.PurposeCode]
		if !ok || input.NoticeVersion != requirement.NoticeVersion {
			return nil, false
		}
		result[requirement.PurposeCode] = input.Granted
	}
	return result, true
}

func answerMap(inputs []AnswerInput) map[uuid.UUID]json.RawMessage {
	result := make(map[uuid.UUID]json.RawMessage, len(inputs))
	for _, input := range inputs {
		if _, duplicate := result[input.QuestionID]; duplicate {
			return nil
		}
		result[input.QuestionID] = input.Value
	}
	return result
}

func validResponseQuestion(
	question Question,
	answers map[uuid.UUID]json.RawMessage,
	stableAnswers map[string]json.RawMessage,
	consents map[string]bool,
) bool {
	value, answered := answers[question.ID]
	visible := responseVisible(question.VisibilityRule, stableAnswers)
	allowed := consents[question.PurposeCode] && visible
	if answered && !allowed {
		return false
	}
	if question.Required && allowed && !answered {
		return false
	}
	if !answered {
		return true
	}
	delete(answers, question.ID)
	if !validAnswerValue(question, value) {
		return false
	}
	stableAnswers[question.StableKey] = value
	return true
}

func validAnswerValue(question Question, value json.RawMessage) bool {
	switch question.AnswerType {
	case AnswerShortText, AnswerLongText:
		return validTextAnswer(question, value)
	case AnswerSingleChoice:
		return validSingleChoice(question.Options, value)
	case AnswerMultipleChoice:
		return validMultipleChoice(question, value)
	case AnswerBoolean:
		var answer bool
		return json.Unmarshal(value, &answer) == nil
	case AnswerIntegerRange, AnswerRating:
		return validIntegerAnswer(question.Validation, value)
	case AnswerDate:
		return validDateAnswer(value)
	case AnswerStateCity:
		return validStateCityAnswer(value)
	default:
		return false
	}
}

func validTextAnswer(question Question, value json.RawMessage) bool {
	var answer string
	if json.Unmarshal(value, &answer) != nil {
		return false
	}
	minimum, maximum := int32(0), int32(2000)
	if question.Validation != nil {
		minimum = valueOr(question.Validation.MinLength, minimum)
		maximum = valueOr(question.Validation.MaxLength, maximum)
	}
	length := int32(len([]rune(answer)))
	byteLength := int32(len([]byte(answer)))
	return length >= minimum &&
		length <= maximum &&
		byteLength <= maximum
}

func validSingleChoice(options []Option, value json.RawMessage) bool {
	var answer string
	if json.Unmarshal(value, &answer) != nil {
		return false
	}
	return slices.Contains(optionValues(options), answer)
}

func validMultipleChoice(question Question, value json.RawMessage) bool {
	var answers []string
	if json.Unmarshal(value, &answers) != nil || len(answers) == 0 {
		return false
	}
	maximum := int32(50)
	if question.Validation != nil {
		maximum = valueOr(question.Validation.MaxSelections, maximum)
	}
	if len(answers) > int(maximum) || duplicateStrings(answers) {
		return false
	}
	allowed := optionValues(question.Options)
	for _, answer := range answers {
		if !slices.Contains(allowed, answer) {
			return false
		}
	}
	return true
}

func validIntegerAnswer(validation *ValidationDefinition, value json.RawMessage) bool {
	var answer int32
	if json.Unmarshal(value, &answer) != nil {
		return false
	}
	if validation == nil {
		return true
	}
	minimum := valueOr(validation.Minimum, int32(-1_000_000))
	maximum := valueOr(validation.Maximum, int32(1_000_000))
	return answer >= minimum && answer <= maximum
}

func validDateAnswer(value json.RawMessage) bool {
	var answer string
	if json.Unmarshal(value, &answer) != nil {
		return false
	}
	parsed, err := time.Parse("2006-01-02", answer)
	return err == nil && parsed.Format("2006-01-02") == answer
}

func validStateCityAnswer(value json.RawMessage) bool {
	var answer struct {
		State    string `json:"state"`
		CityCode string `json:"city_code"`
	}
	if json.Unmarshal(value, &answer) != nil {
		return false
	}
	return len(answer.State) == 2 && len(answer.CityCode) == 7
}

func responseVisible(
	rule *VisibilityRule,
	answers map[string]json.RawMessage,
) bool {
	if rule == nil {
		return true
	}
	conditions := rule.All
	matchAll := true
	if len(conditions) == 0 {
		conditions = rule.Any
		matchAll = false
	}
	matches := 0
	for _, condition := range conditions {
		if conditionMatches(condition, answers) {
			matches++
		}
	}
	if matchAll {
		return matches == len(conditions)
	}
	return matches > 0
}

func conditionMatches(
	condition Condition,
	answers map[string]json.RawMessage,
) bool {
	value, answered := answers[condition.Question]
	switch condition.Operator {
	case "answered":
		return answered
	case "equals":
		return matchEquals(answered, value, condition.Value)
	case "not_equals":
		return matchNotEquals(answered, value, condition.Value)
	case "in":
		return matchIn(answered, condition.Value, value)
	case "contains":
		return matchIn(answered, value, condition.Value)
	default:
		return false
	}
}

func matchEquals(answered bool, left, right json.RawMessage) bool {
	return answered && jsonEqual(left, right)
}

func matchNotEquals(answered bool, left, right json.RawMessage) bool {
	return answered && !jsonEqual(left, right)
}

func matchIn(answered bool, arrayValue, expected json.RawMessage) bool {
	return answered && rawArrayContains(arrayValue, expected)
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return bytes.Equal(mustJSON(leftValue), mustJSON(rightValue))
}

func rawArrayContains(arrayValue, expected json.RawMessage) bool {
	var values []json.RawMessage
	if json.Unmarshal(arrayValue, &values) != nil {
		return false
	}
	for _, value := range values {
		if jsonEqual(value, expected) {
			return true
		}
	}
	return false
}

func mustJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func optionValues(options []Option) []string {
	result := make([]string, 0, len(options))
	for _, option := range options {
		result = append(result, option.Value)
	}
	return result
}

func duplicateStrings(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func valueOr(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	return *value
}
