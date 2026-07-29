package outbox_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/google/uuid"
)

func TestEventContainsOnlyAggregateIdentityVersionAndType(t *testing.T) {
	t.Parallel()

	event := outbox.Event{
		ID:               uuid.MustParse("019f0000-0000-7000-8000-000000000041"),
		AggregateType:    outbox.AggregateStay,
		AggregateID:      uuid.MustParse("019f0000-0000-7000-8000-000000000042"),
		AggregateVersion: 2,
		Type:             outbox.EventStayPresenceRecalculationRequested,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	event.AggregateType = outbox.AggregateMembership
	if err := event.Validate(); !errors.Is(err, outbox.ErrInvalidEvent) {
		t.Fatalf("Validate(mismatched aggregate) error = %v", err)
	}
	forbidden := map[string]bool{
		"Payload": true, "Metadata": true, "Token": true, "Visitors": true, "Subject": true,
	}
	eventType := reflect.TypeFor[outbox.Event]()
	for index := range eventType.NumField() {
		if forbidden[eventType.Field(index).Name] {
			t.Fatalf("Event contains forbidden field %s", eventType.Field(index).Name)
		}
	}
}

func TestPhase3EventsContainNoPayload(t *testing.T) {
	t.Parallel()
	cases := []outbox.Event{
		{
			ID: uuid.New(), AggregateType: outbox.AggregateQuestionnaireVersion,
			AggregateID: uuid.New(), AggregateVersion: 2,
			Type: outbox.EventQuestionnairePublished,
		},
		{
			ID: uuid.New(), AggregateType: outbox.AggregateSurveyResponse,
			AggregateID: uuid.New(), AggregateVersion: 1,
			Type: outbox.EventSurveyResponseRecorded,
		},
	}
	for _, event := range cases {
		if err := event.Validate(); err != nil {
			t.Fatalf("phase 3 event rejected: %v", err)
		}
	}
}
