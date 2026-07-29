package analytics_test

import (
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

func TestMaterializePresenceUsesSourceVersionAndWeights(t *testing.T) {
	t.Parallel()

	stayID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	visitors := []uuid.UUID{
		uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		uuid.MustParse("019f0000-0000-7000-8000-000000000003"),
	}
	source := analytics.PresenceSource{
		StayID:           stayID,
		Status:           stay.StatusPreRegistered,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-12"),
		Version:          7,
		VisitorIDs:       visitors,
	}

	got, err := analytics.MaterializePresence(
		source,
		stay.MustCivilDate("2026-12-10"),
		0.80,
	)
	if err != nil {
		t.Fatalf("MaterializePresence() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("facts = %d, want 4", len(got))
	}
	for _, fact := range got {
		if fact.StayID != stayID || fact.SourceStayVersion != 7 ||
			fact.Kind != stay.PresenceForecast || fact.Weight != 0.80 {
			t.Fatalf("fact = %#v", fact)
		}
	}
}

func TestMaterializePresenceSplitsCheckedInAtCutoff(t *testing.T) {
	t.Parallel()

	checkedInAt := time.Date(2026, 12, 10, 15, 0, 0, 0, time.UTC)
	source := analytics.PresenceSource{
		StayID:           uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		Status:           stay.StatusCheckedIn,
		PlannedArrival:   stay.MustCivilDate("2026-12-10"),
		PlannedDeparture: stay.MustCivilDate("2026-12-13"),
		CheckedInAt:      &checkedInAt,
		Version:          2,
		VisitorIDs: []uuid.UUID{
			uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		},
	}

	got, err := analytics.MaterializePresence(
		source,
		stay.MustCivilDate("2026-12-11"),
		0.80,
	)
	if err != nil {
		t.Fatalf("MaterializePresence() error = %v", err)
	}
	if len(got) != 3 ||
		got[0].Kind != stay.PresenceObserved ||
		got[1].Kind != stay.PresenceObserved ||
		got[2].Kind != stay.PresenceForecast ||
		got[2].Weight != 1 {
		t.Fatalf("facts = %#v", got)
	}
}
