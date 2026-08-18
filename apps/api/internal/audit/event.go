package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidActor = errors.New("invalid audit actor")
	ErrInvalidEvent = errors.New("invalid audit event")
	auditVersion    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	auditRequestID  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
)

type ActorDigest struct {
	Version string
	Sum     [sha256.Size]byte
}

type ActorHasher struct {
	version string
	key     []byte
}

func NewActorHasher(version string, key []byte) (*ActorHasher, error) {
	if !auditVersion.MatchString(version) || len(key) < 32 {
		return nil, ErrInvalidActor
	}
	return &ActorHasher{version: version, key: append([]byte(nil), key...)}, nil
}

func (h *ActorHasher) Pseudonymize(issuer, subject string) (ActorDigest, error) {
	issuer = strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	subject = strings.TrimSpace(subject)
	if issuer == "" || subject == "" {
		return ActorDigest{}, ErrInvalidActor
	}
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte("audit-actor\x00"))
	_, _ = mac.Write([]byte(issuer))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(subject))
	var sum [sha256.Size]byte
	copy(sum[:], mac.Sum(nil))
	return ActorDigest{Version: h.version, Sum: sum}, nil
}

type ActorType string

const (
	ActorUser             ActorType = "user"
	ActorInvite           ActorType = "invite"
	ActorSurveyCapability ActorType = "survey_capability"
	ActorSystem           ActorType = "system"
)

type Action string

const (
	ActionAccommodationCreated   Action = "accommodation.created"
	ActionAccommodationUpdated   Action = "accommodation.updated"
	ActionMembershipCreated      Action = "membership.created"
	ActionMembershipUpdated      Action = "membership.updated"
	ActionStayCreated            Action = "stay.created"
	ActionStayUpdated            Action = "stay.updated"
	ActionStayInvited            Action = "stay.invited"
	ActionStayGroupSubmitted     Action = "stay.group_submitted"
	ActionStayCheckedIn          Action = "stay.checked_in"
	ActionStayCheckedOut         Action = "stay.checked_out"
	ActionStayCancelled          Action = "stay.cancelled"
	ActionStayNoShow             Action = "stay.no_show"
	ActionStayApproved           Action = "stay.approved"
	ActionStayRejected           Action = "stay.rejected"
	ActionStayApprovalExpired    Action = "stay.approval_expired"
	ActionAccommodationInvited   Action = "accommodation.invited"
	ActionAccommodationRevoked   Action = "accommodation.invite_revoked"
	ActionActivationIssued       Action = "accommodation.activation_issued"
	ActionActivationCompleted    Action = "accommodation.activation_completed"
	ActionQuestionnaireCreated   Action = "questionnaire.created"
	ActionQuestionnaireCloned    Action = "questionnaire.cloned"
	ActionQuestionnaireUpdated   Action = "questionnaire.updated"
	ActionQuestionnaireReview    Action = "questionnaire.review_submitted"
	ActionQuestionnaireChanges   Action = "questionnaire.changes_requested"
	ActionQuestionnaireApproved  Action = "questionnaire.approved"
	ActionQuestionnairePublished Action = "questionnaire.published"
	ActionQuestionnaireRetired   Action = "questionnaire.retired"
	ActionSurveyRecorded         Action = "survey_response.recorded"
	// The access request has actions of its own because it is not an
	// accommodation: it is born from an anonymous origin and may be refused.
	// Reusing accommodation.created would put every refused attempt into the
	// trail the rest of the system reads as a real registration (ADR-042).
	ActionAccessRequestCreated  Action = "accommodation_access_request.created"
	ActionAccessRequestApproved Action = "accommodation_access_request.approved"
	ActionAccessRequestRejected Action = "accommodation_access_request.rejected"
	ActionAccessRequestExpired  Action = "accommodation_access_request.expired"
)

type EntityType string

const (
	EntityAccommodation        EntityType = "accommodation"
	EntityMembership           EntityType = "membership"
	EntityStay                 EntityType = "stay"
	EntityQuestionnaire        EntityType = "questionnaire"
	EntityQuestionnaireVersion EntityType = "questionnaire_version"
	EntitySurveyResponse       EntityType = "survey_response"
	// EntityAccessRequest exists before any organization does, and a rejected or
	// expired request never has one. That is why it joins the organization-less
	// group of validOrganization.
	EntityAccessRequest EntityType = "accommodation_access_request"
)

var actionEntities = map[Action]EntityType{
	ActionAccommodationCreated:   EntityAccommodation,
	ActionAccommodationUpdated:   EntityAccommodation,
	ActionMembershipCreated:      EntityMembership,
	ActionMembershipUpdated:      EntityMembership,
	ActionStayCreated:            EntityStay,
	ActionStayUpdated:            EntityStay,
	ActionStayInvited:            EntityStay,
	ActionStayGroupSubmitted:     EntityStay,
	ActionStayCheckedIn:          EntityStay,
	ActionStayCheckedOut:         EntityStay,
	ActionStayCancelled:          EntityStay,
	ActionStayApproved:           EntityStay,
	ActionStayRejected:           EntityStay,
	ActionStayApprovalExpired:    EntityStay,
	ActionAccommodationInvited:   EntityAccommodation,
	ActionAccommodationRevoked:   EntityAccommodation,
	ActionActivationIssued:       EntityAccommodation,
	ActionActivationCompleted:    EntityAccommodation,
	ActionStayNoShow:             EntityStay,
	ActionQuestionnaireCreated:   EntityQuestionnaire,
	ActionQuestionnaireCloned:    EntityQuestionnaireVersion,
	ActionQuestionnaireUpdated:   EntityQuestionnaireVersion,
	ActionQuestionnaireReview:    EntityQuestionnaireVersion,
	ActionQuestionnaireChanges:   EntityQuestionnaireVersion,
	ActionQuestionnaireApproved:  EntityQuestionnaireVersion,
	ActionQuestionnairePublished: EntityQuestionnaireVersion,
	ActionQuestionnaireRetired:   EntityQuestionnaireVersion,
	ActionSurveyRecorded:         EntitySurveyResponse,
	ActionAccessRequestCreated:   EntityAccessRequest,
	ActionAccessRequestApproved:  EntityAccessRequest,
	ActionAccessRequestRejected:  EntityAccessRequest,
	ActionAccessRequestExpired:   EntityAccessRequest,
}

type PurposeCode string

const (
	PurposeStayOperation           PurposeCode = "stay_operation"
	PurposeQuestionnaireGovernance PurposeCode = "questionnaire_governance"
	PurposeTourismSurvey           PurposeCode = "tourism_survey"
)

type ChangedField string

const (
	FieldName             ChangedField = "name"
	FieldCategory         ChangedField = "category"
	FieldCadasturID       ChangedField = "cadastur_id"
	FieldCapacity         ChangedField = "capacity"
	FieldPublicAreaCode   ChangedField = "public_area_code"
	FieldRole             ChangedField = "role"
	FieldActive           ChangedField = "active"
	FieldPlannedArrival   ChangedField = "planned_arrival_on"
	FieldPlannedDeparture ChangedField = "planned_departure_on"
	FieldExpectedGuests   ChangedField = "expected_guest_count"
	FieldStatus           ChangedField = "status"
	FieldDefinition       ChangedField = "definition"
	FieldParticipation    ChangedField = "participation"
)

var validChangedFields = map[ChangedField]bool{
	FieldName: true, FieldCategory: true, FieldCadasturID: true,
	FieldCapacity: true, FieldPublicAreaCode: true, FieldRole: true,
	FieldActive: true, FieldPlannedArrival: true, FieldPlannedDeparture: true,
	FieldExpectedGuests: true, FieldStatus: true,
	FieldDefinition: true, FieldParticipation: true,
}

type Outcome string

const OutcomeSucceeded Outcome = "succeeded"

type Event struct {
	ID             uuid.UUID
	Actor          ActorDigest
	ActorType      ActorType
	OrganizationID uuid.UUID
	Action         Action
	EntityType     EntityType
	EntityID       uuid.UUID
	PurposeCode    PurposeCode
	RequestID      string
	ChangedFields  []ChangedField
	Outcome        Outcome
}

func (e Event) Validate() error {
	validators := []bool{
		e.ID != uuid.Nil,
		auditVersion.MatchString(e.Actor.Version),
		validActorType(e.ActorType),
		validOrganization(e.EntityType, e.OrganizationID),
		actionEntities[e.Action] == e.EntityType,
		e.EntityID != uuid.Nil,
		validPurpose(e.EntityType, e.PurposeCode),
		auditRequestID.MatchString(e.RequestID),
		e.Outcome == OutcomeSucceeded,
		fieldsValid(e.ChangedFields),
	}
	for _, valid := range validators {
		if !valid {
			return ErrInvalidEvent
		}
	}
	return nil
}

func validActorType(actorType ActorType) bool {
	return actorType == ActorUser ||
		actorType == ActorInvite ||
		actorType == ActorSurveyCapability ||
		actorType == ActorSystem
}

func validOrganization(entity EntityType, organizationID uuid.UUID) bool {
	global := entity == EntityQuestionnaire ||
		entity == EntityQuestionnaireVersion ||
		entity == EntitySurveyResponse ||
		entity == EntityAccessRequest
	return global == (organizationID == uuid.Nil)
}

func validPurpose(entity EntityType, purpose PurposeCode) bool {
	switch entity {
	case EntityQuestionnaire, EntityQuestionnaireVersion:
		return purpose == PurposeQuestionnaireGovernance
	case EntitySurveyResponse:
		return purpose == PurposeTourismSurvey
	default:
		return purpose == PurposeStayOperation
	}
}

func fieldsValid(fields []ChangedField) bool {
	for _, field := range fields {
		if !validChangedFields[field] {
			return false
		}
	}
	return true
}
