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
	if len(condition.Value) == 0 || !json.Valid(condition.Value) {
		return false
	}
	switch condition.Operator {
	case "equals", "not_equals":
		return validEqualsCondition(source, condition.Value)
	case "in":
		return validInCondition(source, condition.Value)
	case "contains":
		return validContainsCondition(source, condition.Value)
	default:
		return false
	}
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

func validInCondition(source Question, raw json.RawMessage) bool {
	var values []any
	if json.Unmarshal(raw, &values) != nil ||
		len(values) == 0 ||
		len(values) > 20 {
		return false
	}
	for _, value := range values {
		if !validScalarCondition(source, value) {
			return false
		}
	}
	return true
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

func validMultipleCondition(source Question, values []string) bool {
	if len(values) == 0 || len(values) > 20 || duplicateStrings(values) {
		return false
	}
	for _, value := range values {
		if !validOptionCondition(source.Options, value) {
			return false
		}
	}
	return true
}

func validConditionDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validSafeConditionString(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	return utf8.ValidString(value) &&
		trimmed == value &&
		len(value) >= 1 &&
		len(value) <= 100 &&
		!strings.ContainsAny(value, "<>/\\") &&
		!strings.Contains(lower, "javascript:") &&
		!strings.Contains(lower, "data:")
}
