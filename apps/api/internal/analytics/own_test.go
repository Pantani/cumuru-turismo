package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestEvaluateComparison(t *testing.T) {
	policy := analytics.DefaultComparisonPolicy()
	cases := []struct {
		name        string
		ownCapacity int32
		reporting   analytics.VillageReporting
		wantStatus  string
		wantReason  string
	}{
		{
			name:        "below the reporting floor stays closed",
			ownCapacity: 10,
			reporting:   analytics.VillageReporting{Accommodations: 4, Capacity: 200},
			wantStatus:  analytics.ComparisonUnavailable,
			wantReason:  analytics.ComparisonReasonFewAccommodations,
		},
		{
			name:        "empty denominator stays closed",
			ownCapacity: 10,
			reporting:   analytics.VillageReporting{Accommodations: 9, Capacity: 0},
			wantStatus:  analytics.ComparisonUnavailable,
			wantReason:  analytics.ComparisonReasonFewAccommodations,
		},
		{
			name:        "own capacity above a quarter of the village stays closed",
			ownCapacity: 51,
			reporting:   analytics.VillageReporting{Accommodations: 6, Capacity: 200},
			wantStatus:  analytics.ComparisonUnavailable,
			wantReason:  analytics.ComparisonReasonOwnShareTooHigh,
		},
		{
			name:        "own capacity exactly at the ceiling is still allowed",
			ownCapacity: 50,
			reporting:   analytics.VillageReporting{Accommodations: 6, Capacity: 200},
			wantStatus:  analytics.ComparisonAvailable,
		},
		{
			name:        "a small share of a wide village opens",
			ownCapacity: 12,
			reporting:   analytics.VillageReporting{Accommodations: 11, Capacity: 320},
			wantStatus:  analytics.ComparisonAvailable,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := analytics.EvaluateComparison(
				testCase.ownCapacity, testCase.reporting, policy,
			)
			if got.Status != testCase.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, testCase.wantStatus)
			}
			if got.Reason != testCase.wantReason {
				t.Fatalf("reason = %q, want %q", got.Reason, testCase.wantReason)
			}
		})
	}
}

func TestEvaluateComparisonRejectsWeakerPolicyThanTheFloor(t *testing.T) {
	weaker := analytics.ComparisonPolicy{
		MinimumReportingAccommodations: 3,
		MaximumOwnCapacitySharePercent: 25,
	}
	got := analytics.EvaluateComparison(
		1, analytics.VillageReporting{Accommodations: 40, Capacity: 900}, weaker,
	)
	if got.Status != analytics.ComparisonUnavailable {
		t.Fatalf("status = %q, want the comparison closed", got.Status)
	}
}
