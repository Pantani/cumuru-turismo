package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestExplainableBaselineUsesMedianAndLeadFactor(t *testing.T) {
	t.Parallel()

	got, err := analytics.ExplainableBaseline(
		40,
		[]float64{80, 100, 90, 110, 100, 105, 85, 115},
		15,
	)
	if err != nil {
		t.Fatalf("ExplainableBaseline() error = %v", err)
	}
	if got.Central != 70 || got.Lower != 59.5 || got.Upper != 80.5 || got.Fallback {
		t.Fatalf("ExplainableBaseline() = %#v", got)
	}
}

func TestExplainableBaselineFallsBackToOnBooksWithoutEightWeeks(t *testing.T) {
	t.Parallel()

	got, err := analytics.ExplainableBaseline(20, []float64{30, 40}, 30)
	if err != nil {
		t.Fatalf("ExplainableBaseline() error = %v", err)
	}
	if got.Central != 20 || got.Lower != 14 || got.Upper != 26 || !got.Fallback {
		t.Fatalf("ExplainableBaseline() = %#v", got)
	}
}
