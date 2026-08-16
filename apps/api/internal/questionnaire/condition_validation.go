package questionnaire

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func validCondition(
	condition Condition,
	prior map[string]Question,
	currentOrder int32,
) bool {
	source, ok := conditionSource(condition, prior, currentOrder)
	if !ok {
		return false
	}
	if condition.Operator == "answered" {
		return len(condition.Value) == 0
	}
	return validConditionValue(source, condition)
}

func validConditionValue(source Question, condition Condition) bool {
	if len(condition.Value) == 0 || !json.Valid(condition.Value) {
		return false
	}
	validate, known := conditionValueValidators[condition.Operator]
	return known && validate(source, condition.Value)
}

// One value validator per operator that carries a value. "answered" is absent
// on purpose: it must arrive with no value at all.
var conditionValueValidators = map[string]func(Question, json.RawMessage) bool{
	"equals":     validEqualsCondition,
	"not_equals": validEqualsCondition,
	"in":         validInCondition,
	"contains":   validContainsCondition,
}

func conditionSource(
	condition Condition,
	prior map[string]Question,
	currentOrder int32,
) (Question, bool) {
	source, exists := prior[condition.Question]
	if !exists || source.DisplayOrder >= currentOrder {
		return Question{}, false
	}
	return source, conditionOperators[condition.Operator]
}

func validEqualsCondition(source Question, raw json.RawMessage) bool {
	if source.AnswerType == AnswerMultipleChoice {
		var values []string
		return json.Unmarshal(raw, &values) == nil &&
			validMultipleCondition(source, values)
	}
	var value any
	return json.Unmarshal(raw, &value) == nil &&
		validScalarCondition(source, value)
}

func allValidScalarConditions(source Question, values []any) bool {
	for _, value := range values {
		if !validScalarCondition(source, value) {
			return false
		}
	}
	return true
}

func validInCondition(source Question, raw json.RawMessage) bool {
	var values []any
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	if len(values) == 0 || len(values) > 20 {
		return false
	}
	return allValidScalarConditions(source, values)
}

func validContainsCondition(source Question, raw json.RawMessage) bool {
	if source.AnswerType != AnswerMultipleChoice {
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil &&
		validSafeConditionString(value) &&
		slices.Contains(optionValues(source.Options), value)
}

func validScalarCondition(source Question, value any) bool {
	if source.AnswerType == AnswerBoolean {
		_, ok := value.(bool)
		return ok
	}
	if source.AnswerType == AnswerIntegerRange ||
		source.AnswerType == AnswerRating {
		return validConditionNumber(value)
	}
	return validConditionText(source, value)
}

func validConditionNumber(value any) bool {
	number, ok := value.(float64)
	return ok && math.Trunc(number) == number
}

func validConditionText(source Question, value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	switch source.AnswerType {
	case AnswerSingleChoice:
		return validOptionCondition(source.Options, text)
	case AnswerShortText, AnswerLongText:
		return validSafeConditionString(text)
	case AnswerDate:
		return validConditionDate(text)
	default:
		return false
	}
}

func validOptionCondition(options []Option, value string) bool {
	return validSafeConditionString(value) &&
		slices.Contains(optionValues(options), value)
}

func allValidOptionConditions(options []Option, values []string) bool {
	for _, value := range values {
		if !validOptionCondition(options, value) {
			return false
		}
	}
	return true
}

func validMultipleCondition(source Question, values []string) bool {
	if len(values) == 0 || len(values) > 20 || duplicateStrings(values) {
		return false
	}
	return allValidOptionConditions(source.Options, values)
}

func validConditionDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

// A condition literal reaches a rendered questionnaire, so it must not be able
// to carry markup or a scheme that a renderer might act on.
func inertConditionString(value string) bool {
	lower := strings.ToLower(value)
	return !strings.ContainsAny(value, "<>/\\") &&
		!strings.Contains(lower, "javascript:") &&
		!strings.Contains(lower, "data:")
}

func validSafeConditionString(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	if len(value) < 1 || len(value) > 100 {
		return false
	}
	return inertConditionString(value)
}
