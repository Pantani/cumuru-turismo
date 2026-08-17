package proofofwork_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/proofofwork"
)

// N-19: the cost rises with the bucket counter instead of the bucket getting
// tighter, so the tenth guest behind the pousada Wi-Fi pays a few more seconds
// rather than receiving a 429.
func TestDifficultyRisesWithTheBucketCounter(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		requestCount int32
		want         uint8
	}{
		"first request of the window": {0, 12},
		"below the first step":        {1, 12},
		"one step":                    {2, 13},
		"four steps":                  {8, 16},
		"beyond the ceiling":          {200, 18},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := proofofwork.Difficulty(12, 18, 2, test.requestCount)
			if got != test.want {
				t.Fatalf("Difficulty(%d) = %d, want %d", test.requestCount, got, test.want)
			}
		})
	}
}

// A counter large enough to wrap the addition would otherwise hand the abuser
// the cheapest challenge in the system.
func TestDifficultySaturatesInsteadOfWrapping(t *testing.T) {
	t.Parallel()

	const maxCount = int32(1<<31 - 1)
	if got := proofofwork.Difficulty(12, 18, 1, maxCount); got != 18 {
		t.Fatalf("Difficulty(max) = %d, want the ceiling", got)
	}
	if got := proofofwork.Difficulty(1, 32, 1, maxCount); got != 32 {
		t.Fatalf("Difficulty(max) = %d, want the ceiling", got)
	}
}

// A misconfiguration must not silently produce a challenge nobody has to solve.
func TestDifficultyRefusesAnInvalidRange(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ base, ceiling uint8 }{
		"zero base":            {0, 18},
		"ceiling below base":   {18, 12},
		"ceiling out of range": {12, proofofwork.MaxDifficultyBits + 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := proofofwork.Difficulty(test.base, test.ceiling, 2, 4); got != 0 {
				t.Fatalf("Difficulty() = %d, want 0 for an invalid range", got)
			}
		})
	}
}

// A step of zero or a negative counter must not lower the base.
func TestDifficultyNeverFallsBelowTheBase(t *testing.T) {
	t.Parallel()

	for _, step := range []int32{0, -1} {
		if got := proofofwork.Difficulty(12, 18, step, 100); got != 12 {
			t.Fatalf("Difficulty(step=%d) = %d, want the base", step, got)
		}
	}
	if got := proofofwork.Difficulty(12, 18, 2, -5); got != 12 {
		t.Fatalf("Difficulty(negative count) = %d, want the base", got)
	}
}
