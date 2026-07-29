package stay_test

import (
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

func TestPresenceDaysAtSplitsCheckedInObservedAndFutureForecast(t *testing.T) {
	t.Parallel()

	checkedInAt := time.Date(2026, 12, 10, 15, 0, 0, 0, time.UTC)
	value := stay.Stay{
		Status:           stay.StatusCheckedIn,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-14"),
		CheckedInAt:      &checkedInAt,
	}

	got, err := stay.PresenceDaysAt(value, stay.MustCivilDate("2026-12-11"))
	if err != nil {
		t.Fatalf("PresenceDaysAt() error = %v", err)
	}
	want := []struct {
		date string
		kind stay.PresenceKind
	}{
		{"2026-12-10", stay.PresenceObserved},
		{"2026-12-11", stay.PresenceObserved},
		{"2026-12-12", stay.PresenceForecast},
		{"2026-12-13", stay.PresenceForecast},
	}
	if len(got) != len(want) {
		t.Fatalf("PresenceDaysAt() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index].Date.String() != want[index].date || got[index].Kind != want[index].kind {
			t.Fatalf("PresenceDaysAt()[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func TestPresenceDaysAtFailsClosedWhenCutoffPrecedesCheckIn(t *testing.T) {
	t.Parallel()

	checkedInAt := time.Date(2026, 12, 12, 15, 0, 0, 0, time.UTC)
	value := stay.Stay{
		Status:           stay.StatusCheckedIn,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-14"),
		CheckedInAt:      &checkedInAt,
	}

	if _, err := stay.PresenceDaysAt(value, stay.MustCivilDate("2026-12-11")); err == nil {
		t.Fatal("PresenceDaysAt() error = nil")
	}
}
