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
	denominator := 0
	numerator := 0
	reporting := 0
	cutoff := asOf.Add(-freshness)
	for _, accommodation := range accommodations {
		if !accommodation.Active || accommodation.Capacity <= 0 {
			continue
		}
		denominator += accommodation.Capacity
		if accommodation.LastReportedAt.IsZero() ||
			accommodation.LastReportedAt.Before(cutoff) {
			continue
		}
		numerator += accommodation.Capacity
		reporting++
	}
	if denominator == 0 || reporting < minimumReporting {
		return Coverage{Status: CoverageUnavailable}
	}
	ratio := roundHalfUp(100*float64(numerator)/float64(denominator), 5)
	return Coverage{Status: CoveragePublished, Ratio: &ratio}
}
