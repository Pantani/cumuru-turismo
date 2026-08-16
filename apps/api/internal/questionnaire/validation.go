package questionnaire

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	catalogKeyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)
	codePattern          = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	idempotencyPattern   = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
	requestIDPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	capabilityPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
	changeReasonCodes    = stringSet("privacy_metadata_incomplete", "excessive_collection", "unsafe_condition", "consent_mismatch")
	conditionOperators   = stringSet("equals", "not_equals", "in", "contains", "answered")
	validAnswerTypes     = answerTypeSet()
	validClassifications = classificationSet()
)

func validActor(actorIssuer, actorSubject string) bool {
	return strings.TrimSpace(actorIssuer) != "" && strings.TrimSpace(actorSubject) != ""
}

func validPage(cursorSet bool, cursorID uuid.UUID, limit int32) bool {
	return limit >= 1 && limit <= 100 && cursorSet == (cursorID != uuid.Nil)
}

func validCreate(command CreateCommand) bool {
	return validCreateIdentity(command) &&
		validCreateText(command) &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validCreateIdentity(command CreateCommand) bool {
	return validActor(command.Actor.Issuer, command.Actor.Subject) &&
		command.ID.Version() == 7 &&
		command.VersionID.Version() == 7 &&
		validCatalogKey(command.StableKey)
}

func validCreateText(command CreateCommand) bool {
	return validText(command.Name, 1, 160) &&
		validText(command.Title, 1, 200) &&
		validText(command.PrivacyNoticeVersion, 1, 100)
}

func validClone(command CloneCommand) bool {
	return validActor(command.Actor.Issuer, command.Actor.Subject) &&
		command.QuestionnaireID != uuid.Nil &&
		command.SourceVersionID != uuid.Nil &&
		command.NewVersionID.Version() == 7 &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validUpdate(command UpdateCommand) bool {
	return validActor(command.Actor.Issuer, command.Actor.Subject) &&
		command.VersionID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		requestIDPattern.MatchString(command.RequestID) &&
		ValidateDefinition(command.Definition) == nil
}

func validTransitionAuthority(command TransitionCommand) bool {
	return validActor(command.Actor.Issuer, command.Actor.Subject) &&
		command.VersionID != uuid.Nil &&
		command.ExpectedVersion >= 1 &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

// A change request carries a reason code from the closed catalogue; every other
// transition must carry none.
func validTransitionReason(command TransitionCommand) bool {
	if command.Transition == TransitionRequestChanges {
		return changeReasonCodes[command.ReasonCode]
	}
	return command.ReasonCode == "" && validTransitions()[command.Transition]
}

func validTransition(command TransitionCommand) bool {
	return validTransitionAuthority(command) && validTransitionReason(command)
}

func validSubmission(command SubmissionCommand) bool {
	if !validSubmissionAuthority(command) {
		return false
	}
	if command.Participation == ParticipationDeclined {
		return len(command.Answers) == 0 && len(command.Consents) == 0
	}
	return validSubmittedPayload(command)
}

func validSubmissionAuthority(command SubmissionCommand) bool {
	return capabilityPattern.MatchString(command.Capability) &&
		command.RateSubject != "" &&
		command.VersionID != uuid.Nil &&
		command.ClientSubmission.Version() == 7 &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validSubmittedPayload(command SubmissionCommand) bool {
	return command.Participation == ParticipationSubmitted &&
		len(command.Answers) <= 100 &&
		len(command.Consents) <= 50 &&
		validAnswerInputs(command.Answers) &&
		validConsentInputs(command.Consents)
}

func validDefinitionText(definition Definition) bool {
	return validText(definition.Title, 1, 200) &&
		validOptionalText(definition.Introduction, 2000) &&
		validText(definition.PrivacyNoticeVersion, 1, 100)
}

func validDefinitionCounts(definition Definition) bool {
	return len(definition.Questions) >= 1 &&
		len(definition.Questions) <= 100 &&
		len(definition.ConsentRequirements) <= 50
}

func ValidateDefinition(definition Definition) error {
	valid := validDefinitionText(definition) &&
		validDefinitionCounts(definition) &&
		validRequirements(definition.ConsentRequirements) &&
		validQuestions(definition.Questions, definition.ConsentRequirements)
	if !valid {
		return ErrInvalidInput
	}
	return nil
}

// publishableQuestion refuses to publish sensitive or secret classifications,
// and refuses free text while the free-text pipeline is disabled.
func publishableQuestion(question Question, freeTextEnabled bool) bool {
	if question.DataClassification == ClassificationSensitive ||
		question.DataClassification == ClassificationSecret {
		return false
	}
	return freeTextEnabled || !isFreeText(question.AnswerType)
}

func ValidatePublishable(definition Definition, freeTextEnabled bool) error {
	if err := ValidateDefinition(definition); err != nil {
		return err
	}
	for _, question := range definition.Questions {
		if !publishableQuestion(question, freeTextEnabled) {
			return ErrInvalidInput
		}
	}
	return nil
}

func validRequirements(requirements []ConsentRequirement) bool {
	purposes := make(map[string]bool, len(requirements))
	orders := make(map[int32]bool, len(requirements))
	for _, requirement := range requirements {
		if !validRequirement(requirement) ||
			purposes[requirement.PurposeCode] ||
			orders[requirement.DisplayOrder] {
			return false
		}
		purposes[requirement.PurposeCode] = true
		orders[requirement.DisplayOrder] = true
	}
	return true
}

func validRequirement(requirement ConsentRequirement) bool {
	return validCode(requirement.PurposeCode, 100) &&
		validText(requirement.NoticeVersion, 1, 100) &&
		validText(requirement.Prompt, 1, 500) &&
		within32(requirement.DisplayOrder, 1, 100)
}

func validQuestions(questions []Question, requirements []ConsentRequirement) bool {
	context := questionValidation{
		ids: make(map[uuid.UUID]bool), keys: make(map[string]Question),
		orders: make(map[int32]bool), purposes: requirementPurposes(requirements),
	}
	for _, question := range questions {
		if !context.add(question) {
			return false
		}
	}
	return true
}

type questionValidation struct {
	ids      map[uuid.UUID]bool
	keys     map[string]Question
	orders   map[int32]bool
	purposes map[string]bool
}

func (v *questionValidation) unique(question Question) bool {
	if v.ids[question.ID] || v.orders[question.DisplayOrder] {
		return false
	}
	_, duplicate := v.keys[question.StableKey]
	return !duplicate
}

func (v *questionValidation) accepts(question Question) bool {
	return validQuestionFields(question) &&
		v.purposes[question.PurposeCode] &&
		validVisibility(question.VisibilityRule, v.keys, question.DisplayOrder) &&
		v.unique(question)
}

func (v *questionValidation) add(question Question) bool {
	if !v.accepts(question) {
		return false
	}
	v.ids[question.ID] = true
	v.keys[question.StableKey] = question
	v.orders[question.DisplayOrder] = true
	return true
}

func validQuestionFields(question Question) bool {
	return validQuestionIdentity(question) &&
		validQuestionType(question) &&
		validPrivacyMetadata(question)
}

func validQuestionCodes(question Question) bool {
	return validCode(question.StableKey, 64) &&
		validCode(question.PurposeCode, 100) &&
		validCode(question.RetentionPolicyCode, 100)
}

func validQuestionIdentity(question Question) bool {
	return question.ID.Version() == 7 &&
		validQuestionCodes(question) &&
		validText(question.Prompt, 1, 500) &&
		validOptionalText(question.HelpText, 500) &&
		within32(question.DisplayOrder, 1, 100)
}

func validQuestionType(question Question) bool {
	return validAnswerTypes[question.AnswerType] &&
		validClassifications[question.DataClassification] &&
		validOptions(question.AnswerType, question.Options) &&
		validValidation(question.Validation)
}

func validPrivacyMetadata(question Question) bool {
	if isFreeText(question.AnswerType) {
		return validFreeTextMetadata(question)
	}
	if question.PublicAggregationAllowed {
		return validAggregationMetadata(question)
	}
	return question.AnalyticsKey == nil && question.MinimumPublicCell == nil
}

func validFreeTextMetadata(question Question) bool {
	return !question.Required &&
		question.DataClassification == ClassificationPersonal &&
		question.AnalyticsKey == nil &&
		!question.PublicAggregationAllowed &&
		question.MinimumPublicCell == nil
}

func validAggregationMetadata(question Question) bool {
	return validOptionalStableKey(question.AnalyticsKey) &&
		question.MinimumPublicCell != nil &&
		*question.MinimumPublicCell >= 10
}

type optionSeen struct {
	ids    map[uuid.UUID]bool
	values map[string]bool
	orders map[int32]bool
}

func newOptionSeen(size int) optionSeen {
	return optionSeen{
		ids:    make(map[uuid.UUID]bool, size),
		values: make(map[string]bool, size),
		orders: make(map[int32]bool, size),
	}
}

func (s optionSeen) accept(option Option) bool {
	if s.ids[option.ID] || s.values[option.Value] || s.orders[option.DisplayOrder] {
		return false
	}
	s.ids[option.ID] = true
	s.values[option.Value] = true
	s.orders[option.DisplayOrder] = true
	return true
}

// Only choice questions carry options, and a choice question must carry some.
func choiceAnswerType(answerType AnswerType) bool {
	return answerType == AnswerSingleChoice || answerType == AnswerMultipleChoice
}

func distinctValidOptions(options []Option) bool {
	seen := newOptionSeen(len(options))
	for _, option := range options {
		if !validOption(option) || !seen.accept(option) {
			return false
		}
	}
	return true
}

func validOptions(answerType AnswerType, options []Option) bool {
	if choiceAnswerType(answerType) != (len(options) > 0) {
		return false
	}
	return len(options) <= 100 && distinctValidOptions(options)
}

func validOption(option Option) bool {
	return option.ID.Version() == 7 &&
		validText(option.Value, 1, 100) &&
		validText(option.Label, 1, 200) &&
		within32(option.DisplayOrder, 1, 100)
}

func validValidation(validation *ValidationDefinition) bool {
	if validation == nil {
		return true
	}
	return ordered(validation.MinLength, validation.MaxLength, 0, 2000) &&
		ordered(validation.Minimum, validation.Maximum, -1_000_000, 1_000_000) &&
		within(validation.MaxSelections, 1, 50)
}

// ruleConditions returns the single populated branch of the rule, or nil when
// the rule sets both or neither — the DSL allows exactly one.
func ruleConditions(rule *VisibilityRule) []Condition {
	if (len(rule.All) > 0) == (len(rule.Any) > 0) {
		return nil
	}
	if len(rule.All) > 0 {
		return rule.All
	}
	return rule.Any
}

func validConditions(conditions []Condition, prior map[string]Question, currentOrder int32) bool {
	for _, condition := range conditions {
		if !validCondition(condition, prior, currentOrder) {
			return false
		}
	}
	return true
}

func validVisibility(rule *VisibilityRule, prior map[string]Question, currentOrder int32) bool {
	if rule == nil {
		return true
	}
	conditions := ruleConditions(rule)
	if len(conditions) == 0 || len(conditions) > 10 {
		return false
	}
	return validConditions(conditions, prior, currentOrder)
}

func validAnswerInputs(answers []AnswerInput) bool {
	seen := make(map[uuid.UUID]bool, len(answers))
	for _, answer := range answers {
		if answer.QuestionID == uuid.Nil || seen[answer.QuestionID] || !json.Valid(answer.Value) {
			return false
		}
		seen[answer.QuestionID] = true
	}
	return true
}

func validConsentInputs(consents []ConsentDecisionInput) bool {
	seen := make(map[string]bool, len(consents))
	for _, consent := range consents {
		if !validCode(consent.PurposeCode, 100) ||
			!validText(consent.NoticeVersion, 1, 100) ||
			seen[consent.PurposeCode] {
			return false
		}
		seen[consent.PurposeCode] = true
	}
	return true
}

func validMutationMeta(key, requestID string) bool {
	return idempotencyPattern.MatchString(key) && requestIDPattern.MatchString(requestID)
}

func validCatalogKey(value string) bool {
	return catalogKeyPattern.MatchString(value)
}

func validCode(value string, maximum int) bool {
	return len(value) <= maximum && codePattern.MatchString(value)
}

func validText(value string, minimum, maximum int) bool {
	length := utf8.RuneCountInString(strings.TrimSpace(value))
	return utf8.ValidString(value) && length >= minimum && length <= maximum
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, 1, maximum)
}

func validOptionalStableKey(value *string) bool {
	return value != nil && validCode(*value, 100)
}

func within32(value, minimum, maximum int32) bool {
	return value >= minimum && value <= maximum
}

func within(value *int32, minimum, maximum int32) bool {
	return value == nil || (*value >= minimum && *value <= maximum)
}

func ordered(minimum, maximum *int32, floor, ceiling int32) bool {
	if !within(minimum, floor, ceiling) || !within(maximum, floor, ceiling) {
		return false
	}
	return minimum == nil || maximum == nil || *minimum <= *maximum
}

func requirementPurposes(requirements []ConsentRequirement) map[string]bool {
	result := make(map[string]bool, len(requirements))
	for _, requirement := range requirements {
		result[requirement.PurposeCode] = true
	}
	return result
}

func isFreeText(answerType AnswerType) bool {
	return answerType == AnswerShortText || answerType == AnswerLongText
}

func validTransitions() map[Transition]bool {
	return map[Transition]bool{
		TransitionSubmitReview: true, TransitionRequestChanges: true,
		TransitionApprove: true, TransitionPublish: true, TransitionRetire: true,
	}
}

func answerTypeSet() map[AnswerType]bool {
	return map[AnswerType]bool{
		AnswerShortText: true, AnswerLongText: true, AnswerSingleChoice: true,
		AnswerMultipleChoice: true, AnswerBoolean: true, AnswerIntegerRange: true,
		AnswerRating: true, AnswerDate: true, AnswerStateCity: true,
	}
}

func classificationSet() map[DataClassification]bool {
	return map[DataClassification]bool{
		ClassificationPublic: true, ClassificationOperational: true,
		ClassificationPersonal: true, ClassificationSensitive: true,
		ClassificationSecret: true,
	}
}

func stringSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
