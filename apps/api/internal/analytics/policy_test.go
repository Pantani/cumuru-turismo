package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestProtectCellsAppliesPrimaryAndComplementarySuppression(t *testing.T) {
	t.Parallel()

	cells := []analytics.Cell{
		{Key: "total", Family: "visit", RawValue: 20, SampleSize: 20, AccommodationCount: 3, Total: true},
		{Key: "first_visit", Family: "visit", RawValue: 12, SampleSize: 12, AccommodationCount: 3},
		{Key: "returning", Family: "visit", RawValue: 8, SampleSize: 8, AccommodationCount: 3},
	}

	got, err := analytics.ProtectCells(cells, analytics.Policy{
		PrimaryThreshold:               10,
		MinimumReportingAccommodations: 3,
		RoundingBase:                   10,
	})
	if err != nil {
		t.Fatalf("ProtectCells() error = %v", err)
	}
	status := make(map[string]analytics.CellStatus, len(got))
	for _, cell := range got {
		status[cell.Key] = cell.Status
	}
	if status["returning"] != analytics.CellProtected {
		t.Fatalf("returning status = %s", status["returning"])
	}
	if status["first_visit"] != analytics.CellProtected && status["total"] != analytics.CellProtected {
		t.Fatalf("no complementary cell protected: %#v", status)
	}
}

func TestProtectCellsRequiresThreeAccommodationsAndRoundsAfterSuppression(t *testing.T) {
	t.Parallel()

	got, err := analytics.ProtectCells([]analytics.Cell{
		{Key: "small-participation", Family: "a", RawValue: 14, SampleSize: 14, AccommodationCount: 2},
		{Key: "publishable", Family: "b", RawValue: 15, SampleSize: 15, AccommodationCount: 3},
	}, analytics.Policy{
		PrimaryThreshold:               10,
		MinimumReportingAccommodations: 3,
		RoundingBase:                   10,
	})
	if err != nil {
		t.Fatalf("ProtectCells() error = %v", err)
	}
	if got[0].Status != analytics.CellProtected || got[0].PublishedValue != nil {
		t.Fatalf("protected cell = %#v", got[0])
	}
	if got[1].Status != analytics.CellPublished || got[1].PublishedValue == nil ||
		*got[1].PublishedValue != 20 {
		t.Fatalf("published cell = %#v", got[1])
	}
}

func TestRoundForecastPreservesOrderedBounds(t *testing.T) {
	t.Parallel()

	got, err := analytics.RoundForecast(14, 15, 16, 10)
	if err != nil {
		t.Fatalf("RoundForecast() error = %v", err)
	}
	if got.Lower != 10 || got.Central != 20 || got.Upper != 20 {
		t.Fatalf("RoundForecast() = %#v", got)
	}
}
