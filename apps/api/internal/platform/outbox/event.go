package outbox

import (
	"errors"

	"github.com/google/uuid"
)

var ErrInvalidEvent = errors.New("invalid outbox event")

type AggregateType string

const (
	AggregateAccommodation        AggregateType = "accommodation"
	AggregateMembership           AggregateType = "membership"
	AggregateStay                 AggregateType = "stay"
	AggregateQuestionnaire        AggregateType = "questionnaire"
	AggregateQuestionnaireVersion AggregateType = "questionnaire_version"
	AggregateSurveyResponse       AggregateType = "survey_response"
	// A poster and an activation capability are aggregates of their own. Filing
	// their events under the accommodation would reuse
	// (aggregate_type, aggregate_id, aggregate_version, event_type) on every
	// rotation, because issuing one does not bump the accommodation version.
	AggregateAccommodationInvite     AggregateType = "accommodation_invite"
	AggregateAccommodationActivation AggregateType = "accommodation_activation"
	// The access request is an aggregate of its own: it exists before the
	// accommodation and may never become one. Filing it under the accommodation
	// would force inventing a registration identifier for the rejected request,
	// which has none.
	AggregateAccessRequest AggregateType = "accommodation_access_request"

	// O feed é agregado próprio: a reserva importada muda de versão a cada
	// sincronização, e pendurá-la na estadia faria a estadia versionar por
	// causa de um arquivo remoto (ADR-044).
	AggregateCalendarFeed        AggregateType = "calendar_feed"
	AggregateCalendarReservation AggregateType = "calendar_reservation"
)

type EventType string

const (
	EventAccommodationCreated               EventType = "accommodation.created"
	EventAccommodationUpdated               EventType = "accommodation.updated"
	EventMembershipCreated                  EventType = "membership.created"
	EventMembershipUpdated                  EventType = "membership.updated"
	EventStayCreated                        EventType = "stay.created"
	EventStayUpdated                        EventType = "stay.updated"
	EventStayInvited                        EventType = "stay.invited"
	EventStayGroupSubmitted                 EventType = "stay.group_submitted"
	EventStayCheckedIn                      EventType = "stay.checked_in"
	EventStayCheckedOut                     EventType = "stay.checked_out"
	EventStayCancelled                      EventType = "stay.cancelled"
	EventStayNoShow                         EventType = "stay.no_show"
	EventStayApproved                       EventType = "stay.approved"
	EventStayRejected                       EventType = "stay.rejected"
	EventStayApprovalExpired                EventType = "stay.approval_expired"
	EventAccommodationInvited               EventType = "accommodation.invited"
	EventAccommodationInviteRevoked         EventType = "accommodation.invite_revoked"
	EventAccommodationActivationIssued      EventType = "accommodation.activation_issued"
	EventAccommodationActivationCompleted   EventType = "accommodation.activation_completed"
	EventStayPresenceRecalculationRequested EventType = "stay.presence_recalculation_requested"
	EventQuestionnaireCreated               EventType = "questionnaire.created"
	EventQuestionnaireCloned                EventType = "questionnaire.cloned"
	EventQuestionnaireUpdated               EventType = "questionnaire.updated"
	EventQuestionnaireReviewSubmitted       EventType = "questionnaire.review_submitted"
	EventQuestionnaireChangesRequested      EventType = "questionnaire.changes_requested"
	EventQuestionnaireApproved              EventType = "questionnaire.approved"
	EventQuestionnairePublished             EventType = "questionnaire.published"
	EventQuestionnaireRetired               EventType = "questionnaire.retired"
	EventSurveyResponseRecorded             EventType = "survey_response.recorded"
	EventAccessRequestCreated               EventType = "accommodation_access_request.created"
	EventAccessRequestApproved              EventType = "accommodation_access_request.approved"
	EventAccessRequestRejected              EventType = "accommodation_access_request.rejected"
	EventCalendarFeedRegistered             EventType = "calendar_feed.registered"
	EventCalendarFeedRemoved                EventType = "calendar_feed.removed"
	EventCalendarReservationConfirmed       EventType = "calendar_reservation.confirmed"
	EventCalendarReservationDismissed       EventType = "calendar_reservation.dismissed"
)

var eventAggregates = map[EventType]AggregateType{
	EventAccommodationCreated:               AggregateAccommodation,
	EventAccommodationUpdated:               AggregateAccommodation,
	EventMembershipCreated:                  AggregateMembership,
	EventMembershipUpdated:                  AggregateMembership,
	EventStayCreated:                        AggregateStay,
	EventStayUpdated:                        AggregateStay,
	EventStayInvited:                        AggregateStay,
	EventStayGroupSubmitted:                 AggregateStay,
	EventStayCheckedIn:                      AggregateStay,
	EventStayCheckedOut:                     AggregateStay,
	EventStayCancelled:                      AggregateStay,
	EventStayApproved:                       AggregateStay,
	EventStayRejected:                       AggregateStay,
	EventStayApprovalExpired:                AggregateStay,
	EventAccommodationInvited:               AggregateAccommodationInvite,
	EventAccommodationInviteRevoked:         AggregateAccommodationInvite,
	EventAccommodationActivationIssued:      AggregateAccommodationActivation,
	EventAccommodationActivationCompleted:   AggregateAccommodationActivation,
	EventStayNoShow:                         AggregateStay,
	EventStayPresenceRecalculationRequested: AggregateStay,
	EventQuestionnaireCreated:               AggregateQuestionnaire,
	EventQuestionnaireCloned:                AggregateQuestionnaireVersion,
	EventQuestionnaireUpdated:               AggregateQuestionnaireVersion,
	EventQuestionnaireReviewSubmitted:       AggregateQuestionnaireVersion,
	EventQuestionnaireChangesRequested:      AggregateQuestionnaireVersion,
	EventQuestionnaireApproved:              AggregateQuestionnaireVersion,
	EventQuestionnairePublished:             AggregateQuestionnaireVersion,
	EventQuestionnaireRetired:               AggregateQuestionnaireVersion,
	EventSurveyResponseRecorded:             AggregateSurveyResponse,
	EventAccessRequestCreated:               AggregateAccessRequest,
	EventAccessRequestApproved:              AggregateAccessRequest,
	EventAccessRequestRejected:              AggregateAccessRequest,
	EventCalendarFeedRegistered:             AggregateCalendarFeed,
	EventCalendarFeedRemoved:                AggregateCalendarFeed,
	EventCalendarReservationConfirmed:       AggregateCalendarReservation,
	EventCalendarReservationDismissed:       AggregateCalendarReservation,
}

type Event struct {
	ID               uuid.UUID
	AggregateType    AggregateType
	AggregateID      uuid.UUID
	AggregateVersion int64
	Type             EventType
}

func (e Event) Validate() error {
	if e.ID == uuid.Nil || e.AggregateID == uuid.Nil || e.AggregateVersion < 1 {
		return ErrInvalidEvent
	}
	if eventAggregates[e.Type] != e.AggregateType {
		return ErrInvalidEvent
	}
	return nil
}
