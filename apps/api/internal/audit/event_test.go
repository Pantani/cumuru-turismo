package audit_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/google/uuid"
)

func TestActorHasherPseudonymizesIssuerAndSubject(t *testing.T) {
	t.Parallel()

	hasher, err := audit.NewActorHasher("actor-v1", []byte("actor-key-material-is-at-least-32-bytes"))
	if err != nil {
		t.Fatalf("NewActorHasher() error = %v", err)
	}
	first, err := hasher.Pseudonymize("https://issuer.invalid", "fixture-operator")
	if err != nil {
		t.Fatalf("Pseudonymize() error = %v", err)
	}
	second, _ := hasher.Pseudonymize("https://issuer.invalid", "fixture-operator")
	other, _ := hasher.Pseudonymize("https://issuer.invalid", "fixture-manager")
	if first.Version != "actor-v1" || first.Sum != second.Sum || first.Sum == other.Sum {
		t.Fatalf("pseudonyms are not deterministic and scoped")
	}
	if bytes.Contains(first.Sum[:], []byte("fixture-operator")) {
		t.Fatal("pseudonym contains subject")
	}
}

func TestEventIsClosedAndRejectsUnapprovedFields(t *testing.T) {
	t.Parallel()

	event := audit.Event{
		ID:             uuid.MustParse("019f0000-0000-7000-8000-000000000031"),
		Actor:          audit.ActorDigest{Version: "actor-v1", Sum: [32]byte{1}},
		ActorType:      audit.ActorUser,
		OrganizationID: uuid.MustParse("019f0000-0000-7000-8000-000000000032"),
		Action:         audit.ActionStayUpdated,
		EntityType:     audit.EntityStay,
		EntityID:       uuid.MustParse("019f0000-0000-7000-8000-000000000033"),
		PurposeCode:    audit.PurposeStayOperation,
		RequestID:      "019f0000-0000-7000-8000-000000000034",
		ChangedFields:  []audit.ChangedField{audit.FieldPlannedDeparture},
		Outcome:        audit.OutcomeSucceeded,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	event.ChangedFields = []audit.ChangedField{"visitor_name"}
	if err := event.Validate(); !errors.Is(err, audit.ErrInvalidEvent) {
		t.Fatalf("Validate(visitor_name) error = %v", err)
	}
	forbidden := map[string]bool{"Metadata": true, "Body": true, "Token": true, "Subject": true}
	eventType := reflect.TypeFor[audit.Event]()
	for index := range eventType.NumField() {
		if forbidden[eventType.Field(index).Name] {
			t.Fatalf("Event contains forbidden field %s", eventType.Field(index).Name)
		}
	}
}

func TestQuestionnaireEventAllowsGlobalOrganization(t *testing.T) {
	t.Parallel()
	event := questionnaireEvent(
		audit.ActorUser,
		audit.ActionQuestionnairePublished,
		audit.EntityQuestionnaireVersion,
		audit.PurposeQuestionnaireGovernance,
		audit.FieldStatus,
	)
	if err := event.Validate(); err != nil {
		t.Fatalf("global questionnaire event rejected: %v", err)
	}
}

func TestSurveyEventRequiresSurveyPurpose(t *testing.T) {
	t.Parallel()
	event := questionnaireEvent(
		audit.ActorSurveyCapability,
		audit.ActionSurveyRecorded,
		audit.EntitySurveyResponse,
		audit.PurposeTourismSurvey,
		audit.FieldParticipation,
	)
	if err := event.Validate(); err != nil {
		t.Fatalf("survey event rejected: %v", err)
	}
	event.PurposeCode = audit.PurposeStayOperation
	if err := event.Validate(); !errors.Is(err, audit.ErrInvalidEvent) {
		t.Fatalf("survey event with stay purpose error = %v", err)
	}
}

func TestAccommodationCreatedEventUsesOnlyAllowlistedFields(t *testing.T) {
	t.Parallel()
	event := audit.Event{
		ID:             uuid.MustParse("019f0000-0000-7000-8000-000000000081"),
		Actor:          audit.ActorDigest{Version: "actor-v1", Sum: [32]byte{1}},
		ActorType:      audit.ActorUser,
		OrganizationID: uuid.MustParse("019f0000-0000-7000-8000-000000000082"),
		Action:         audit.ActionAccommodationCreated,
		EntityType:     audit.EntityAccommodation,
		EntityID:       uuid.MustParse("019f0000-0000-7000-8000-000000000083"),
		PurposeCode:    audit.PurposeStayOperation,
		RequestID:      "019f0000-0000-7000-8000-000000000084",
		ChangedFields: []audit.ChangedField{
			audit.FieldName,
			audit.FieldCategory,
			audit.FieldCapacity,
			audit.FieldStatus,
		},
		Outcome: audit.OutcomeSucceeded,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func questionnaireEvent(
	actorType audit.ActorType,
	action audit.Action,
	entity audit.EntityType,
	purpose audit.PurposeCode,
	field audit.ChangedField,
) audit.Event {
	return audit.Event{
		ID:        uuid.MustParse("019f0000-0000-7000-8000-000000000071"),
		Actor:     audit.ActorDigest{Version: "actor-v1", Sum: [32]byte{1}},
		ActorType: actorType, OrganizationID: uuid.Nil,
		Action: action, EntityType: entity,
		EntityID:    uuid.MustParse("019f0000-0000-7000-8000-000000000072"),
		PurposeCode: purpose, RequestID: "019f0000-0000-7000-8000-000000000073",
		ChangedFields: []audit.ChangedField{field}, Outcome: audit.OutcomeSucceeded,
	}
}
