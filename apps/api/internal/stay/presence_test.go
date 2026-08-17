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
		Approval:         stay.ApprovalNotRequired,
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
		Approval:         stay.ApprovalNotRequired,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-14"),
		CheckedInAt:      &checkedInAt,
	}

	if _, err := stay.PresenceDaysAt(value, stay.MustCivilDate("2026-12-11")); err == nil {
		t.Fatal("PresenceDaysAt() error = nil")
	}
}

// The three points of the approval filter are the query projection, the
// repository choke point and the SQL function. This is the fourth, and the one
// no future query can bypass: a pending self-registration accrues nothing, even
// while it sits in pre_registered with dates inside the forecast window (N-35).
func TestPendingSelfRegistrationAccruesNoPresence(t *testing.T) {
	t.Parallel()

	pending := stay.Stay{
		Status:           stay.StatusPreRegistered,
		Approval:         stay.ApprovalPending,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-14"),
	}
	for _, state := range []stay.ApprovalState{
		stay.ApprovalPending, stay.ApprovalRejected, stay.ApprovalExpired,
	} {
		pending.Approval = state
		days, err := stay.PresenceDays(pending)
		if err != nil || len(days) != 0 {
			t.Fatalf("PresenceDays(%s) = %#v, %v; want no days", state, days, err)
		}
	}
	pending.Approval = stay.ApprovalApproved
	days, err := stay.PresenceDays(pending)
	if err != nil || len(days) != 4 {
		t.Fatalf("PresenceDays(approved) = %#v, %v; want four days", days, err)
	}
}

// A caller that never read approval_state must fail loudly instead of
// publishing a stay nobody approved.
func TestUndecidedApprovalRefusesToMaterializePresence(t *testing.T) {
	t.Parallel()

	undecided := stay.Stay{
		Status:           stay.StatusPreRegistered,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-14"),
	}
	if _, err := stay.PresenceDays(undecided); err == nil {
		t.Fatal("PresenceDays() accepted a stay with no approval decision")
	}
	if _, err := stay.PresenceDaysAt(undecided, stay.MustCivilDate("2026-12-11")); err == nil {
		t.Fatal("PresenceDaysAt() accepted a stay with no approval decision")
	}
}
