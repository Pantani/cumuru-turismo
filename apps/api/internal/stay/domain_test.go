package stay_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

func TestStateMachineAcceptsDocumentedTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from  stay.Status
		event stay.Event
		to    stay.Status
	}{
		{stay.StatusDraft, stay.EventInvite, stay.StatusInvited},
		{stay.StatusInvited, stay.EventInvite, stay.StatusInvited},
		{stay.StatusDraft, stay.EventSubmitGroup, stay.StatusPreRegistered},
		{stay.StatusInvited, stay.EventSubmitGroup, stay.StatusPreRegistered},
		{stay.StatusPreRegistered, stay.EventCheckIn, stay.StatusCheckedIn},
		{stay.StatusCheckedIn, stay.EventCheckOut, stay.StatusCheckedOut},
		{stay.StatusDraft, stay.EventCancel, stay.StatusCancelled},
		{stay.StatusInvited, stay.EventCancel, stay.StatusCancelled},
		{stay.StatusPreRegistered, stay.EventCancel, stay.StatusCancelled},
		{stay.StatusInvited, stay.EventNoShow, stay.StatusNoShow},
		{stay.StatusPreRegistered, stay.EventNoShow, stay.StatusNoShow},
	}
	for _, tt := range tests {
		got, err := tt.from.Transition(tt.event)
		if err != nil || got != tt.to {
			t.Errorf("%s.Transition(%s) = %s, %v; want %s", tt.from, tt.event, got, err, tt.to)
		}
	}
}

func TestStateMachineRejectsUndocumentedTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from  stay.Status
		event stay.Event
	}{
		{stay.StatusDraft, stay.EventCheckIn},
		{stay.StatusInvited, stay.EventCheckOut},
		{stay.StatusPreRegistered, stay.EventInvite},
		{stay.StatusCheckedIn, stay.EventNoShow},
		{stay.StatusCheckedOut, stay.EventCancel},
		{stay.StatusCancelled, stay.EventSubmitGroup},
		{stay.StatusNoShow, stay.EventCheckIn},
	}
	for _, tt := range tests {
		if _, err := tt.from.Transition(tt.event); !errors.Is(err, stay.ErrInvalidTransition) {
			t.Errorf("%s.Transition(%s) error = %v", tt.from, tt.event, err)
		}
	}
}

func TestCheckedInCancellationRequiresManagerCorrection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role       accommodation.Role
		correction bool
		allowed    bool
	}{
		{accommodation.RoleOperator, true, false},
		{accommodation.RoleManager, false, false},
		{accommodation.RoleManager, true, true},
	}
	for _, tt := range tests {
		command := stay.CancelCommand{
			Role: tt.role, Correction: tt.correction, Reason: stay.CancelReasonCorrection,
		}
		err := command.Validate(stay.StatusCheckedIn)
		if (err == nil) != tt.allowed {
			t.Errorf("Validate(%s, correction=%t) error = %v", tt.role, tt.correction, err)
		}
	}
}

func TestNoShowUsesBahiaCivilArrivalDay(t *testing.T) {
	t.Parallel()

	arrival := stay.MustCivilDate("2026-07-28")
	before := time.Date(2026, 7, 28, 2, 59, 59, 0, time.UTC)
	atStart := time.Date(2026, 7, 28, 3, 0, 0, 0, time.UTC)
	if err := stay.ValidateNoShowTime(arrival, before); !errors.Is(err, stay.ErrNoShowBeforeArrival) {
		t.Fatalf("before arrival error = %v", err)
	}
	if err := stay.ValidateNoShowTime(arrival, atStart); err != nil {
		t.Fatalf("arrival start error = %v", err)
	}
}

func TestPresenceDaysFollowStatusAndHalfOpenInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value stay.Stay
		want  []string
		kind  stay.PresenceKind
	}{
		{
			name:  "pre registered forecast",
			value: fixtureStay(stay.StatusPreRegistered),
			want:  []string{"2026-12-10", "2026-12-11"},
			kind:  stay.PresenceForecast,
		},
		{
			name:  "checked in uses Bahia civil day",
			value: withCheckIn(fixtureStay(stay.StatusCheckedIn), time.Date(2026, 12, 11, 2, 30, 0, 0, time.UTC)),
			want:  []string{"2026-12-10", "2026-12-11"},
			kind:  stay.PresenceObserved,
		},
		{
			name: "checked out uses actual departure",
			value: withCheckOut(
				withCheckIn(fixtureStay(stay.StatusCheckedOut), time.Date(2026, 12, 10, 15, 0, 0, 0, time.UTC)),
				time.Date(2026, 12, 12, 12, 0, 0, 0, time.UTC),
			),
			want: []string{"2026-12-10", "2026-12-11"},
			kind: stay.PresenceObserved,
		},
		{name: "cancelled empty", value: fixtureStay(stay.StatusCancelled)},
		{name: "no show empty", value: fixtureStay(stay.StatusNoShow)},
		{name: "draft empty", value: fixtureStay(stay.StatusDraft)},
		{name: "invited empty", value: fixtureStay(stay.StatusInvited)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertPresence(t, tt.value, tt.want, tt.kind)
		})
	}
}

func TestGroupValidationRequiresOneResponsibleAndUniqueClients(t *testing.T) {
	t.Parallel()

	valid := []stay.Visitor{
		{
			ClientID: "019f0000-0000-7000-8000-000000000001",
			Role:     stay.VisitorResponsible, AgeBand: stay.Age25To34,
			ResidenceCountry: "BR", ResidenceState: "BA", ResidenceCityCode: "2925509",
		},
		{
			ClientID: "019f0000-0000-7000-8000-000000000002",
			Role:     stay.VisitorCompanion, AgeBand: stay.Age35To44,
			ResidenceCountry: "AR",
		},
	}
	if err := stay.ValidateGroup(valid); err != nil {
		t.Fatalf("ValidateGroup(valid) error = %v", err)
	}
	duplicate := append([]stay.Visitor(nil), valid...)
	duplicate[1].ClientID = duplicate[0].ClientID
	if err := stay.ValidateGroup(duplicate); !errors.Is(err, stay.ErrDuplicateClient) {
		t.Fatalf("ValidateGroup(duplicate) error = %v", err)
	}
	noResponsible := append([]stay.Visitor(nil), valid...)
	noResponsible[0].Role = stay.VisitorCompanion
	if err := stay.ValidateGroup(noResponsible); !errors.Is(err, stay.ErrResponsibleCount) {
		t.Fatalf("ValidateGroup(no responsible) error = %v", err)
	}
}

func TestGroupValidationRejectsInvalidResidenceShapes(t *testing.T) {
	t.Parallel()

	base := stay.Visitor{
		ClientID: "019f0000-0000-7000-8000-000000000001",
		Role:     stay.VisitorResponsible, AgeBand: stay.Age25To34,
		ResidenceCountry: "BR", ResidenceState: "BA", ResidenceCityCode: "2925509",
	}
	tests := []struct {
		name   string
		mutate func(*stay.Visitor)
	}{
		{name: "lowercase country", mutate: func(value *stay.Visitor) { value.ResidenceCountry = "br" }},
		{name: "Brazil without state", mutate: func(value *stay.Visitor) { value.ResidenceState = "" }},
		{name: "Brazil without city", mutate: func(value *stay.Visitor) { value.ResidenceCityCode = "" }},
		{name: "Brazil non numeric city", mutate: func(value *stay.Visitor) { value.ResidenceCityCode = "abcdefg" }},
		{name: "foreign country with state", mutate: func(value *stay.Visitor) {
			value.ResidenceCountry = "AR"
			value.ResidenceCityCode = ""
		}},
		{name: "foreign country with city", mutate: func(value *stay.Visitor) {
			value.ResidenceCountry = "AR"
			value.ResidenceState = ""
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			visitor := base
			test.mutate(&visitor)
			if err := stay.ValidateGroup([]stay.Visitor{visitor}); !errors.Is(err, stay.ErrInvalidGroup) {
				t.Fatalf("ValidateGroup() error = %v, want %v", err, stay.ErrInvalidGroup)
			}
		})
	}
}

func fixtureStay(status stay.Status) stay.Stay {
	return stay.Stay{
		Status:           status,
		Approval:         stay.ApprovalNotRequired,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-12"),
	}
}

func withCheckIn(value stay.Stay, instant time.Time) stay.Stay {
	value.CheckedInAt = &instant
	return value
}

func withCheckOut(value stay.Stay, instant time.Time) stay.Stay {
	value.CheckedOutAt = &instant
	return value
}

func assertPresence(t *testing.T, value stay.Stay, want []string, kind stay.PresenceKind) {
	t.Helper()
	got, err := stay.PresenceDays(value)
	if err != nil {
		t.Fatalf("PresenceDays() error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("PresenceDays() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index].Date.String() != want[index] || got[index].Kind != kind {
			t.Fatalf("PresenceDays()[%d] = %#v, want %s/%s", index, got[index], want[index], kind)
		}
	}
}
