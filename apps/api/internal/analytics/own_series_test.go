package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func villageValue(value int32) *int32 { return &value }

func TestBuildOwnSeriesIndexesBothSidesOnTheSameDay(t *testing.T) {
	days := []analytics.OwnSeriesDay{
		{Date: "2026-07-01", OwnPersonDays: 0, VillageValue: villageValue(0)},
		{Date: "2026-07-02", OwnPersonDays: 10, VillageValue: villageValue(200)},
		{Date: "2026-07-03", OwnPersonDays: 12, VillageValue: villageValue(300)},
	}
	points := analytics.BuildOwnSeries(days, true)
	if *points[0].OwnIndex != 0 || *points[0].VillageIndex != 0 {
		t.Fatal("an empty day indexes to zero against the base, not to nothing")
	}
	if *points[1].OwnIndex != 100 || *points[1].VillageIndex != 100 {
		t.Fatalf("base day = %d/%d, want 100/100", *points[1].OwnIndex, *points[1].VillageIndex)
	}
	if *points[2].OwnIndex != 120 {
		t.Fatalf("own index = %d, want 120", *points[2].OwnIndex)
	}
	if *points[2].VillageIndex != 150 {
		t.Fatalf("village index = %d, want 150", *points[2].VillageIndex)
	}
}

func TestBuildOwnSeriesKeepsOwnValuesWhenTheComparisonIsClosed(t *testing.T) {
	days := []analytics.OwnSeriesDay{
		{Date: "2026-07-02", OwnPersonDays: 10, VillageValue: villageValue(200)},
	}
	points := analytics.BuildOwnSeries(days, false)
	if points[0].OwnPersonDays != 10 {
		t.Fatalf("own person days = %d, want 10", points[0].OwnPersonDays)
	}
	if points[0].OwnIndex != nil || points[0].VillageIndex != nil {
		t.Fatal("a closed comparison publishes no index at all")
	}
}

func TestBuildOwnSeriesSkipsProtectedVillageDays(t *testing.T) {
	days := []analytics.OwnSeriesDay{
		{Date: "2026-07-02", OwnPersonDays: 10, VillageValue: villageValue(200)},
		{Date: "2026-07-03", OwnPersonDays: 15, VillageValue: nil},
	}
	points := analytics.BuildOwnSeries(days, true)
	if *points[1].OwnIndex != 150 {
		t.Fatalf("own index = %d, want 150", *points[1].OwnIndex)
	}
	if points[1].VillageIndex != nil {
		t.Fatal("a suppressed village cell stays absent from the comparison")
	}
}

func TestBuildOwnSeriesWithoutAnyEligibleBase(t *testing.T) {
	days := []analytics.OwnSeriesDay{
		{Date: "2026-07-02", OwnPersonDays: 4, VillageValue: nil},
	}
	points := analytics.BuildOwnSeries(days, true)
	if points[0].OwnIndex != nil || points[0].VillageIndex != nil {
		t.Fatal("no common base means no index on either side")
	}
}
