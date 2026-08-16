package analytics

import "time"

type CoverageStatus string

const (
	CoveragePublished   CoverageStatus = "published"
	CoverageProtected   CoverageStatus = "protected"
	CoverageUnavailable CoverageStatus = "unavailable"
)

type AccommodationCoverage struct {
	Active         bool
	Capacity       int
	LastReportedAt time.Time
}

type Coverage struct {
	Status CoverageStatus
	Ratio  *int
}

func ComputeCoverage(
	accommodations []AccommodationCoverage,
	asOf time.Time,
	freshness time.Duration,
	minimumReporting int,
) Coverage {
	totals := tallyCoverage(accommodations, asOf.Add(-freshness))
	if totals.denominator == 0 || totals.reporting < minimumReporting {
		return Coverage{Status: CoverageUnavailable}
	}
	ratio := roundHalfUp(
		100*float64(totals.numerator)/float64(totals.denominator), 5,
	)
	return Coverage{Status: CoveragePublished, Ratio: &ratio}
}

type coverageTotals struct {
	denominator int
	numerator   int
	reporting   int
}

// Capacity of every active accommodation forms the denominator; only those that
// reported inside the freshness window count toward the numerator.
func tallyCoverage(
	accommodations []AccommodationCoverage,
	cutoff time.Time,
) coverageTotals {
	var totals coverageTotals
	for _, accommodation := range accommodations {
		if !accommodation.Active || accommodation.Capacity <= 0 {
			continue
		}
		totals.denominator += accommodation.Capacity
		if !reportedSince(accommodation, cutoff) {
			continue
		}
		totals.numerator += accommodation.Capacity
		totals.reporting++
	}
	return totals
}

func reportedSince(accommodation AccommodationCoverage, cutoff time.Time) bool {
	return !accommodation.LastReportedAt.IsZero() &&
		!accommodation.LastReportedAt.Before(cutoff)
}
