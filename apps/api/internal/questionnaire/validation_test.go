package questionnaire

import (
	"strings"
	"testing"
)

func TestValidateDefinitionAcceptsStructuredQuestion(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	if err := ValidateDefinition(definition); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
}

func TestValidateDefinitionRejectsFutureConditionAndFreeTextAnalytics(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	definition.Questions[0].VisibilityRule = &VisibilityRule{
		All: []Condition{{Question: "future", Operator: "answered"}},
	}
	if err := ValidateDefinition(definition); err == nil {
		t.Fatal("future reference accepted")
	}

	definition = validFixtureDefinition(t)
	key := "unsafe"
	cell := int32(10)
	definition.Questions[0].AnswerType = AnswerShortText
	definition.Questions[0].Options = nil
	definition.Questions[0].AnalyticsKey = &key
	definition.Questions[0].MinimumPublicCell = &cell
	definition.Questions[0].PublicAggregationAllowed = true
	if err := ValidateDefinition(definition); err == nil {
		t.Fatal("free text analytics accepted")
	}
}

func TestValidatePublishableRejectsSensitiveAndDisabledFreeText(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	definition.Questions[0].DataClassification = ClassificationSensitive
	if err := ValidatePublishable(definition, true); err == nil {
		t.Fatal("sensitive question publishable")
	}

	definition = validFixtureDefinition(t)
	definition.Questions[0].AnswerType = AnswerShortText
	definition.Questions[0].Options = nil
	definition.Questions[0].Required = false
	definition.Questions[0].DataClassification = ClassificationPersonal
	definition.Questions[0].AnalyticsKey = nil
	definition.Questions[0].MinimumPublicCell = nil
	definition.Questions[0].PublicAggregationAllowed = false
	if err := ValidatePublishable(definition, false); err == nil {
		t.Fatal("free text publishable without cipher")
	}
}

func TestCatalogKeyRequiresThreeToSixtyFourCharacters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		valid bool
	}{
		{value: "a", valid: false},
		{value: "ab", valid: false},
		{value: "abc", valid: true},
		{value: strings.Repeat("a", 64), valid: true},
		{value: strings.Repeat("a", 65), valid: false},
	}
	for _, test := range tests {
		if validCatalogKey(test.value) != test.valid {
			t.Errorf("validCatalogKey(%q) = %v, want %v", test.value, !test.valid, test.valid)
		}
	}
}

func TestDefinitionCodesFollowContractLengths(t *testing.T) {
	t.Parallel()
	definition := validFixtureDefinition(t)
	purpose := "p" + strings.Repeat("a", 99)
	retention := "r" + strings.Repeat("b", 99)
	analytics := "k" + strings.Repeat("c", 99)
	definition.ConsentRequirements[0].PurposeCode = purpose
	definition.Questions[0].StableKey = "q"
	definition.Questions[0].PurposeCode = purpose
	definition.Questions[0].RetentionPolicyCode = retention
	definition.Questions[0].AnalyticsKey = &analytics
	if err := ValidateDefinition(definition); err != nil {
		t.Fatalf("contract-valid one and 100 character codes rejected: %v", err)
	}
}

func TestConditionDSLRejectsNestedUnsafeAndIncompatibleValues(t *testing.T) {
	t.Parallel()
	source := validFixtureDefinition(t).Questions[0]
	prior := map[string]Question{source.StableKey: source}
	tests := []Condition{
		{Question: source.StableKey, Operator: "equals", Value: []byte(`{"nested":true}`)},
		{Question: source.StableKey, Operator: "equals", Value: []byte(`"javascript:alert(1)"`)},
		{Question: source.StableKey, Operator: "equals", Value: []byte(`"https://example.invalid"`)},
		{Question: source.StableKey, Operator: "equals", Value: []byte(`true`)},
		{Question: source.StableKey, Operator: "equals", Value: []byte(`"unknown-option"`)},
		{Question: source.StableKey, Operator: "contains", Value: []byte(`"yes"`)},
	}
	for _, condition := range tests {
		if validCondition(condition, prior, 2) {
			t.Errorf("unsafe condition accepted: %s", condition.Value)
		}
	}
	valid := Condition{
		Question: source.StableKey, Operator: "equals", Value: []byte(`"yes"`),
	}
	if !validCondition(valid, prior, 2) {
		t.Fatal("valid option condition rejected")
	}
}

func validFixtureDefinition(t *testing.T) Definition {
	t.Helper()
	key := "first_visit"
	cell := int32(10)
	return Definition{
		Title: "Pesquisa turística", PrivacyNoticeVersion: "survey-v1",
		ConsentRequirements: []ConsentRequirement{{
			PurposeCode: "tourism_planning", NoticeVersion: "notice-v1",
			Prompt: "Aceita responder?", RequiredForAnswer: true, DisplayOrder: 1,
		}},
		Questions: []Question{{
			ID: mustV7(t), StableKey: key, Prompt: "É sua primeira visita?",
			AnswerType: AnswerSingleChoice, Required: false,
			DataClassification: ClassificationPersonal,
			PurposeCode:        "tourism_planning", RetentionPolicyCode: "survey_prototype_v1",
			AnalyticsKey: &key, PublicAggregationAllowed: true, MinimumPublicCell: &cell,
			DisplayOrder: 1,
			Options: []Option{
				{ID: mustV7(t), Value: "yes", Label: "Sim", DisplayOrder: 1},
				{ID: mustV7(t), Value: "no", Label: "Não", DisplayOrder: 2},
			},
			VisibilityRule: nil,
			Validation:     &ValidationDefinition{},
		}},
	}
}
