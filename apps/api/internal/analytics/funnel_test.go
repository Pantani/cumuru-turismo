package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestLatencyMedianHoldsBackSmallSamples(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		hours  float64
		sample int32
		want   *int32
	}{
		{name: "a single submission is a person, not a median", hours: 3.4, sample: 1},
		{name: "one below the floor stays closed", hours: 3.4, sample: 9},
		{name: "no submission at all", hours: 0, sample: 0},
		{name: "negative duration is a corrupt row", hours: -2, sample: 40},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := analytics.LatencyMedian(testCase.hours, testCase.sample); got != nil {
				t.Fatalf("median = %d, want absent", *got)
			}
		})
	}
}

func TestLatencyMedianRoundsToWholeHoursAtTheFloor(t *testing.T) {
	t.Parallel()

	got := analytics.LatencyMedian(3.6, analytics.FunnelLatencyMinimum)
	if got == nil || *got != 4 {
		t.Fatalf("median = %v, want 4", got)
	}
}

func TestValidFunnelWindowKeepsTheSelectorClosed(t *testing.T) {
	t.Parallel()

	if !analytics.ValidFunnelWindow("last_30_days") {
		t.Fatal("the contract window must be accepted")
	}
	for _, window := range []string{"", "last_90_days", "recent_30_days", "month"} {
		if analytics.ValidFunnelWindow(window) {
			t.Fatalf("%q must be refused, not defaulted", window)
		}
	}
}
