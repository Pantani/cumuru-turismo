package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestPublicationFingerprintIsStableAcrossCellOrder(t *testing.T) {
	t.Parallel()

	first := publicationFixture()
	second := publicationFixture()
	second.Cells[0], second.Cells[1] = second.Cells[1], second.Cells[0]

	firstFingerprint, err := analytics.Fingerprint(first)
	if err != nil {
		t.Fatalf("Fingerprint(first) error = %v", err)
	}
	secondFingerprint, err := analytics.Fingerprint(second)
	if err != nil {
		t.Fatalf("Fingerprint(second) error = %v", err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("fingerprints differ: %q != %q", firstFingerprint, secondFingerprint)
	}
}

func publicationFixture() analytics.Publication {
	return analytics.Publication{
		AsOfOn: "2026-07-28", DataMode: "prototype_fixtures",
		PrivacyPolicyVersion: "prototype-v1",
		MethodologyVersion:   "explainable-baseline-v1",
		CoverageStatus:       "protected",
		Cells: []analytics.PublicationCell{
			{
				CellKey: "b", MetricCode: "presence", PeriodSelector: "next_30_days",
				PeriodStart: "2026-07-29", PeriodEnd: "2026-07-30",
				Unit: "person_day", DimensionCode: "none", CategoryCode: "none",
				Kind: "forecast", Status: analytics.CellProtected,
			},
			{
				CellKey: "a", MetricCode: "presence", PeriodSelector: "recent_30_days",
				PeriodStart: "2026-07-28", PeriodEnd: "2026-07-29",
				Unit: "person_day", DimensionCode: "none", CategoryCode: "none",
				Kind: "observed", Status: analytics.CellProtected,
			},
		},
	}
}
