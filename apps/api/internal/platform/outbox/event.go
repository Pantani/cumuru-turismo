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
)

type EventType string

const (
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
)

var eventAggregates = map[EventType]AggregateType{
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
