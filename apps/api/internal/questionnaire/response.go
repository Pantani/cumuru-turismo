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

// consumeAnswers walks the definition in order; each accepted answer is removed
// from the map, so a leftover entry means the payload answered something the
// definition does not declare.
func consumeAnswers(
	definition Definition,
	answers map[uuid.UUID]json.RawMessage,
	consents map[string]bool,
) bool {
	stableAnswers := make(map[string]json.RawMessage, len(answers))
	for _, question := range definition.Questions {
		if !validResponseQuestion(question, answers, stableAnswers, consents) {
			return false
		}
	}
	return len(answers) == 0
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
	if !consumeAnswers(definition, answers, consents) {
		return ErrInvalidInput
	}
	return nil
}

func consentsByPurpose(inputs []ConsentDecisionInput) (map[string]ConsentDecisionInput, bool) {
	result := make(map[string]ConsentDecisionInput, len(inputs))
	for _, input := range inputs {
		if _, duplicate := result[input.PurposeCode]; duplicate {
			return nil, false
		}
		result[input.PurposeCode] = input
	}
	return result, true
}

// A submission must decide every declared purpose exactly once, and under the
// notice version the requirement was published with.
func exactConsents(
	requirements []ConsentRequirement,
	inputs []ConsentDecisionInput,
) (map[string]bool, bool) {
	if len(requirements) != len(inputs) {
		return nil, false
	}
	inputByPurpose, ok := consentsByPurpose(inputs)
	if !ok {
		return nil, false
	}
	return matchRequirements(requirements, inputByPurpose)
}

func matchRequirements(
	requirements []ConsentRequirement,
	inputByPurpose map[string]ConsentDecisionInput,
) (map[string]bool, bool) {
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
	allowed := consents[question.PurposeCode] &&
		responseVisible(question.VisibilityRule, stableAnswers)
	if !answerPresenceAllowed(question, answered, allowed) {
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

// An answer may only appear where consent and visibility allow it, and a
// required question that is allowed must be answered.
func answerPresenceAllowed(question Question, answered, allowed bool) bool {
	if answered {
		return allowed
	}
	return !(question.Required && allowed)
}

func validBooleanAnswer(value json.RawMessage) bool {
	var answer bool
	return json.Unmarshal(value, &answer) == nil
}

// One validator per answer type. A missing entry rejects the answer, so a new
// type in the contract fails closed instead of accepting anything.
var answerValidators = map[AnswerType]func(Question, json.RawMessage) bool{
	AnswerShortText:      validTextAnswer,
	AnswerLongText:       validTextAnswer,
	AnswerSingleChoice:   func(q Question, v json.RawMessage) bool { return validSingleChoice(q.Options, v) },
	AnswerMultipleChoice: validMultipleChoice,
	AnswerBoolean:        func(_ Question, v json.RawMessage) bool { return validBooleanAnswer(v) },
	AnswerIntegerRange:   func(q Question, v json.RawMessage) bool { return validIntegerAnswer(q.Validation, v) },
	AnswerRating:         func(q Question, v json.RawMessage) bool { return validIntegerAnswer(q.Validation, v) },
	AnswerDate:           func(_ Question, v json.RawMessage) bool { return validDateAnswer(v) },
	AnswerStateCity:      func(_ Question, v json.RawMessage) bool { return validStateCityAnswer(v) },
}

func validAnswerValue(question Question, value json.RawMessage) bool {
	validate, ok := answerValidators[question.AnswerType]
	return ok && validate(question, value)
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

func maxSelections(validation *ValidationDefinition) int32 {
	if validation == nil {
		return 50
	}
	return valueOr(validation.MaxSelections, 50)
}

func allWithinOptions(answers []string, options []Option) bool {
	allowed := optionValues(options)
	for _, answer := range answers {
		if !slices.Contains(allowed, answer) {
			return false
		}
	}
	return true
}

func validMultipleChoice(question Question, value json.RawMessage) bool {
	var answers []string
	if json.Unmarshal(value, &answers) != nil || len(answers) == 0 {
		return false
	}
	if len(answers) > int(maxSelections(question.Validation)) || duplicateStrings(answers) {
		return false
	}
	return allWithinOptions(answers, question.Options)
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
	conditions, matchAll := rule.All, true
	if len(conditions) == 0 {
		conditions, matchAll = rule.Any, false
	}
	matches := countMatches(conditions, answers)
	if matchAll {
		return matches == len(conditions)
	}
	return matches > 0
}

func countMatches(conditions []Condition, answers map[string]json.RawMessage) int {
	matches := 0
	for _, condition := range conditions {
		if conditionMatches(condition, answers) {
			matches++
		}
	}
	return matches
}

// One matcher per operator of the allowed DSL, keyed so an unknown operator
// fails closed instead of falling through a switch default.
var conditionMatchers = map[string]func(bool, json.RawMessage, json.RawMessage) bool{
	"answered": func(answered bool, _, _ json.RawMessage) bool { return answered },
	"equals": func(answered bool, value, expected json.RawMessage) bool {
		return matchEquals(answered, value, expected)
	},
	"not_equals": func(answered bool, value, expected json.RawMessage) bool {
		return matchNotEquals(answered, value, expected)
	},
	"in":       func(answered bool, value, expected json.RawMessage) bool { return matchIn(answered, expected, value) },
	"contains": func(answered bool, value, expected json.RawMessage) bool { return matchIn(answered, value, expected) },
}

func conditionMatches(
	condition Condition,
	answers map[string]json.RawMessage,
) bool {
	match, ok := conditionMatchers[condition.Operator]
	if !ok {
		return false
	}
	value, answered := answers[condition.Question]
	return match(answered, value, condition.Value)
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
