package questionnaire

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput       = errors.New("invalid questionnaire input")
	ErrNotFound           = errors.New("questionnaire resource not found")
	ErrConflict           = errors.New("questionnaire resource conflict")
	ErrPreconditionFailed = errors.New("questionnaire precondition failed")
	ErrCapabilityInvalid  = errors.New("survey capability invalid")
	ErrRateLimited        = errors.New("survey rate limited")
	ErrUnavailable        = errors.New("questionnaire repository unavailable")
)

type VersionStatus string

const (
	StatusDraft         VersionStatus = "draft"
	StatusPrivacyReview VersionStatus = "privacy_review"
	StatusApproved      VersionStatus = "approved"
	StatusPublished     VersionStatus = "published"
	StatusRetired       VersionStatus = "retired"
)

type AnswerType string

const (
	AnswerShortText      AnswerType = "short_text"
	AnswerLongText       AnswerType = "long_text"
	AnswerSingleChoice   AnswerType = "single_choice"
	AnswerMultipleChoice AnswerType = "multiple_choice"
	AnswerBoolean        AnswerType = "boolean"
	AnswerIntegerRange   AnswerType = "integer_range"
	AnswerRating         AnswerType = "rating"
	AnswerDate           AnswerType = "date"
	AnswerStateCity      AnswerType = "state_city"
)

type DataClassification string

const (
	ClassificationPublic      DataClassification = "public"
	ClassificationOperational DataClassification = "operational"
	ClassificationPersonal    DataClassification = "personal"
	ClassificationSensitive   DataClassification = "sensitive"
	ClassificationSecret      DataClassification = "secret"
)

type Questionnaire struct {
	ID        uuid.UUID `json:"id"`
	StableKey string    `json:"stable_key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type PageRequest struct {
	CursorCreatedAt time.Time
	CursorID        uuid.UUID
	Limit           int32
}

type Page struct {
	Items      []Questionnaire
	NextCursor *PageCursor
}

type PageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type VersionPageRequest struct {
	CursorVersionNumber int32
	CursorID            uuid.UUID
	Limit               int32
}

type VersionPage struct {
	Items      []VersionSummary
	NextCursor *VersionCursor
}

type VersionCursor struct {
	VersionNumber int32
	ID            uuid.UUID
}

type VersionSummary struct {
	ID                   uuid.UUID     `json:"id"`
	QuestionnaireID      uuid.UUID     `json:"questionnaire_id"`
	VersionNumber        int32         `json:"version_number"`
	Revision             int64         `json:"revision"`
	Status               VersionStatus `json:"status"`
	Title                string        `json:"title"`
	PrivacyNoticeVersion string        `json:"privacy_notice_version"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

type VersionMutation struct {
	ID              uuid.UUID     `json:"id"`
	QuestionnaireID uuid.UUID     `json:"questionnaire_id"`
	VersionNumber   int32         `json:"version_number"`
	Revision        int64         `json:"revision"`
	Status          VersionStatus `json:"status"`
}

type ValidationDefinition struct {
	MinLength     *int32 `json:"min_length,omitempty"`
	MaxLength     *int32 `json:"max_length,omitempty"`
	Minimum       *int32 `json:"minimum,omitempty"`
	Maximum       *int32 `json:"maximum,omitempty"`
	MaxSelections *int32 `json:"max_selections,omitempty"`
}

type Condition struct {
	Question string          `json:"question"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type VisibilityRule struct {
	All []Condition `json:"all,omitempty"`
	Any []Condition `json:"any,omitempty"`
}

type Option struct {
	ID           uuid.UUID `json:"id"`
	Value        string    `json:"value"`
	Label        string    `json:"label"`
	DisplayOrder int32     `json:"display_order"`
}

type Question struct {
	ID                       uuid.UUID             `json:"id"`
	StableKey                string                `json:"stable_key"`
	Prompt                   string                `json:"prompt"`
	HelpText                 *string               `json:"help_text,omitempty"`
	AnswerType               AnswerType            `json:"answer_type"`
	Required                 bool                  `json:"required"`
	DataClassification       DataClassification    `json:"data_classification"`
	PurposeCode              string                `json:"purpose_code"`
	RetentionPolicyCode      string                `json:"retention_policy_code"`
	AnalyticsKey             *string               `json:"analytics_key,omitempty"`
	PublicAggregationAllowed bool                  `json:"public_aggregation_allowed"`
	MinimumPublicCell        *int32                `json:"minimum_public_cell,omitempty"`
	Validation               *ValidationDefinition `json:"validation,omitempty"`
	VisibilityRule           *VisibilityRule       `json:"visibility_rule,omitempty"`
	DisplayOrder             int32                 `json:"display_order"`
	Options                  []Option              `json:"options"`
}

type ConsentRequirement struct {
	PurposeCode       string `json:"purpose_code"`
	NoticeVersion     string `json:"notice_version"`
	Prompt            string `json:"prompt"`
	RequiredForAnswer bool   `json:"required_for_answers"`
	DisplayOrder      int32  `json:"display_order"`
}

type Definition struct {
	Title                string               `json:"title"`
	Introduction         *string              `json:"introduction,omitempty"`
	PrivacyNoticeVersion string               `json:"privacy_notice_version"`
	Questions            []Question           `json:"questions"`
	ConsentRequirements  []ConsentRequirement `json:"consent_requirements"`
}

type Version struct {
	VersionSummary
	Introduction         *string              `json:"introduction"`
	Questions            []Question           `json:"questions"`
	ConsentRequirements  []ConsentRequirement `json:"consent_requirements"`
	SubmittedForReviewAt *time.Time           `json:"submitted_for_review_at"`
	PrivacyReviewedAt    *time.Time           `json:"privacy_reviewed_at"`
	PublishedAt          *time.Time           `json:"published_at"`
	RetiredAt            *time.Time           `json:"retired_at"`
	LastEditorHMAC       []byte               `json:"-"`
	LastEditorKeyVersion string               `json:"-"`
}

func (v Version) Definition() Definition {
	return Definition{
		Title: v.Title, Introduction: v.Introduction,
		PrivacyNoticeVersion: v.PrivacyNoticeVersion,
		Questions:            v.Questions, ConsentRequirements: v.ConsentRequirements,
	}
}

type PublicQuestion struct {
	ID             uuid.UUID             `json:"id"`
	StableKey      string                `json:"stable_key"`
	Prompt         string                `json:"prompt"`
	HelpText       *string               `json:"help_text,omitempty"`
	AnswerType     AnswerType            `json:"answer_type"`
	Required       bool                  `json:"required"`
	PurposeCode    string                `json:"purpose_code"`
	Validation     *ValidationDefinition `json:"validation,omitempty"`
	VisibilityRule *VisibilityRule       `json:"visibility_rule,omitempty"`
	DisplayOrder   int32                 `json:"display_order"`
	Options        []Option              `json:"options"`
}

func ValidCapabilitySyntax(value string) bool {
	return capabilityPattern.MatchString(value)
}

type Published struct {
	ID                   uuid.UUID            `json:"id"`
	QuestionnaireID      uuid.UUID            `json:"questionnaire_id"`
	StableKey            string               `json:"stable_key"`
	VersionNumber        int32                `json:"version_number"`
	Revision             int64                `json:"revision"`
	Title                string               `json:"title"`
	Introduction         *string              `json:"introduction,omitempty"`
	PrivacyNoticeVersion string               `json:"privacy_notice_version"`
	Questions            []PublicQuestion     `json:"questions"`
	ConsentRequirements  []ConsentRequirement `json:"consent_requirements"`
}

type CreateCommand struct {
	Actor                access.Principal
	ID                   uuid.UUID
	VersionID            uuid.UUID
	StableKey            string
	Name                 string
	Title                string
	PrivacyNoticeVersion string
	IdempotencyKey       string
	RequestID            string
}

type UpdateCommand struct {
	Actor           access.Principal
	VersionID       uuid.UUID
	ExpectedVersion int64
	Definition      Definition
	RequestID       string
}

type CloneCommand struct {
	Actor           access.Principal
	QuestionnaireID uuid.UUID
	SourceVersionID uuid.UUID
	NewVersionID    uuid.UUID
	IdempotencyKey  string
	RequestID       string
}

type Transition string

const (
	TransitionSubmitReview   Transition = "submit_review"
	TransitionRequestChanges Transition = "request_changes"
	TransitionApprove        Transition = "approve"
	TransitionPublish        Transition = "publish"
	TransitionRetire         Transition = "retire"
)

type TransitionCommand struct {
	Actor           access.Principal
	VersionID       uuid.UUID
	ExpectedVersion int64
	Transition      Transition
	ReasonCode      string
	IdempotencyKey  string
	RequestID       string
}

type Participation string

const (
	ParticipationSubmitted Participation = "submitted"
	ParticipationDeclined  Participation = "declined"
)

type AnswerInput struct {
	QuestionID uuid.UUID       `json:"question_id"`
	Value      json.RawMessage `json:"value"`
}

type ConsentDecisionInput struct {
	PurposeCode   string `json:"purpose_code"`
	NoticeVersion string `json:"notice_version"`
	Granted       bool   `json:"granted"`
}

type SubmissionCommand struct {
	Capability       string
	RateSubject      string
	VersionID        uuid.UUID
	ClientSubmission uuid.UUID
	Participation    Participation
	Answers          []AnswerInput
	Consents         []ConsentDecisionInput
	IdempotencyKey   string
	RequestID        string
}

type SubmissionAccepted struct {
	ResponseID    uuid.UUID     `json:"submission_id"`
	Participation Participation `json:"participation"`
	Status        string        `json:"status"`
}

type Repository interface {
	List(context.Context, access.Principal, PageRequest) (Page, error)
	Create(context.Context, CreateCommand) (VersionMutation, bool, error)
	Get(context.Context, access.Principal, uuid.UUID) (Questionnaire, error)
	ListVersions(context.Context, access.Principal, uuid.UUID, VersionPageRequest) (VersionPage, error)
	Clone(context.Context, CloneCommand) (VersionMutation, bool, error)
	GetVersion(context.Context, access.Principal, uuid.UUID) (Version, error)
	UpdateVersion(context.Context, UpdateCommand) (Version, error)
	Transition(context.Context, TransitionCommand) (VersionMutation, bool, error)
	GetPublished(context.Context, string) (Published, error)
	Submit(context.Context, SubmissionCommand) (SubmissionAccepted, bool, error)
	EraseExpiredFreeText(context.Context, time.Time) (int32, error)
}
