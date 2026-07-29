package analytics_test

import (
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestCoverageIsUnavailableWithoutThreeReportingAccommodations(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	got := analytics.ComputeCoverage([]analytics.AccommodationCoverage{
		{Active: true, Capacity: 10, LastReportedAt: asOf},
		{Active: true, Capacity: 20, LastReportedAt: asOf},
		{Active: true, Capacity: 30},
	}, asOf, 30*24*time.Hour, 3)
	if got.Status != analytics.CoverageUnavailable || got.Ratio != nil {
		t.Fatalf("ComputeCoverage() = %#v", got)
	}
}

func TestCoverageRoundsRatioToFivePercentagePoints(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	got := analytics.ComputeCoverage([]analytics.AccommodationCoverage{
		{Active: true, Capacity: 20, LastReportedAt: asOf},
		{Active: true, Capacity: 20, LastReportedAt: asOf},
		{Active: true, Capacity: 20, LastReportedAt: asOf},
		{Active: true, Capacity: 40},
	}, asOf, 30*24*time.Hour, 3)
	if got.Status != analytics.CoveragePublished || got.Ratio == nil || *got.Ratio != 60 {
		t.Fatalf("ComputeCoverage() = %#v", got)
	}
}
