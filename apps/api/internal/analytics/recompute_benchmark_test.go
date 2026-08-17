package analytics_test

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"math"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

const (
	benchmarkHorizonDays       = 1096
	benchmarkSourcesPerDay     = 10
	benchmarkVisitorsPerSource = 3
	benchmarkSourceCount       = benchmarkHorizonDays * benchmarkSourcesPerDay
	benchmarkFactCount         = benchmarkSourceCount * benchmarkVisitorsPerSource
	benchmarkDurationBudget    = 30 * time.Second
	benchmarkHeapBudget        = 512 * 1024 * 1024
)

type recomputePass struct {
	digest     [sha256.Size]byte
	duration   time.Duration
	heapGrowth uint64
	factCount  int
}

func TestAnalyticsThreeYearRecomputeBudget(t *testing.T) {
	sources := buildBenchmarkSources()
	if len(sources) != benchmarkSourceCount {
		t.Fatalf("sources = %d, want %d", len(sources), benchmarkSourceCount)
	}

	runtime.GC()
	previousGCPercent := debug.SetGCPercent(-1)
	first, firstErr := measureRecompute(sources)
	second, secondErr := measureRecompute(sources)
	debug.SetGCPercent(previousGCPercent)
	runtime.GC()

	requireSuccessfulRecompute(t, firstErr, secondErr)
	requireWithinRecomputeBudget(t, first, second)
	t.Logf(
		"ANALYTICS_RECOMPUTE_BENCHMARK=PASS horizon_days=%d sources=%d facts=%d "+
			"digest_sha256=%x duration_ms_first=%d duration_ms_second=%d heap_growth_peak_bytes=%d "+
			"goos=%s goarch=%s gomaxprocs=%d go_version=%s",
		benchmarkHorizonDays,
		len(sources),
		first.factCount,
		first.digest,
		first.duration.Milliseconds(),
		second.duration.Milliseconds(),
		maxUint64(first.heapGrowth, second.heapGrowth),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.GOMAXPROCS(0),
		runtime.Version(),
	)
}

func buildBenchmarkSources() []analytics.PresenceSource {
	start := stay.MustCivilDate("2023-07-29")
	sources := make([]analytics.PresenceSource, 0, benchmarkSourceCount)
	for dayIndex := 0; dayIndex < benchmarkHorizonDays; dayIndex++ {
		for sourceIndex := 0; sourceIndex < benchmarkSourcesPerDay; sourceIndex++ {
			sources = append(sources, benchmarkSource(start, dayIndex, sourceIndex))
		}
	}
	return sources
}

func benchmarkSource(
	start stay.CivilDate,
	dayIndex int,
	sourceIndex int,
) analytics.PresenceSource {
	arrival := start.AddDays(dayIndex)
	departure := arrival.AddDays(1)
	checkedInAt := arrival.Start().Add(15 * time.Hour)
	checkedOutAt := departure.Start().Add(10 * time.Hour)
	return analytics.PresenceSource{
		StayID:           benchmarkUUID(1, dayIndex, sourceIndex, 0),
		Status:           stay.StatusCheckedOut,
		Approval:         stay.ApprovalNotRequired,
		PlannedArrival:   arrival,
		PlannedDeparture: departure,
		CheckedInAt:      &checkedInAt,
		CheckedOutAt:     &checkedOutAt,
		Version:          1,
		VisitorIDs:       benchmarkVisitors(dayIndex, sourceIndex),
	}
}

func benchmarkVisitors(dayIndex int, sourceIndex int) []uuid.UUID {
	visitors := make([]uuid.UUID, benchmarkVisitorsPerSource)
	for visitorIndex := range visitors {
		visitors[visitorIndex] = benchmarkUUID(2, dayIndex, sourceIndex, visitorIndex)
	}
	return visitors
}

func benchmarkUUID(kind byte, dayIndex int, sourceIndex int, visitorIndex int) uuid.UUID {
	var input [13]byte
	input[0] = kind
	binary.LittleEndian.PutUint32(input[1:5], uint32(dayIndex))
	binary.LittleEndian.PutUint32(input[5:9], uint32(sourceIndex))
	binary.LittleEndian.PutUint32(input[9:13], uint32(visitorIndex))
	return uuid.NewSHA1(uuid.Nil, input[:])
}

func measureRecompute(sources []analytics.PresenceSource) (recomputePass, error) {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	startedAt := time.Now()
	facts, err := materializeBenchmark(sources)
	duration := time.Since(startedAt)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return recomputePass{
		digest:     digestPresenceFacts(facts),
		duration:   duration,
		heapGrowth: heapGrowth(before.HeapAlloc, after.HeapAlloc),
		factCount:  len(facts),
	}, err
}

func materializeBenchmark(
	sources []analytics.PresenceSource,
) ([]analytics.PresenceFact, error) {
	asOf := stay.MustCivilDate("2026-07-28")
	facts := make([]analytics.PresenceFact, 0, benchmarkFactCount)
	for _, source := range sources {
		materialized, err := analytics.MaterializePresence(source, asOf, 0.80)
		if err != nil {
			return nil, err
		}
		facts = append(facts, materialized...)
	}
	return facts, nil
}

func digestPresenceFacts(facts []analytics.PresenceFact) [sha256.Size]byte {
	digest := sha256.New()
	for _, fact := range facts {
		writePresenceFact(digest, fact)
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func writePresenceFact(digest hash.Hash, fact analytics.PresenceFact) {
	var number [8]byte
	_, _ = digest.Write(fact.StayID[:])
	_, _ = digest.Write(fact.VisitorID[:])
	_, _ = io.WriteString(digest, fact.PresenceOn.String())
	_, _ = io.WriteString(digest, string(fact.Kind))
	binary.LittleEndian.PutUint64(number[:], math.Float64bits(fact.Weight))
	_, _ = digest.Write(number[:])
	binary.LittleEndian.PutUint64(number[:], uint64(fact.SourceStayVersion))
	_, _ = digest.Write(number[:])
	_, _ = io.WriteString(digest, fact.AsOfOn.String())
}

func requireSuccessfulRecompute(t *testing.T, firstErr error, secondErr error) {
	t.Helper()
	if firstErr != nil {
		t.Fatalf("first recompute: %v", firstErr)
	}
	if secondErr != nil {
		t.Fatalf("second recompute: %v", secondErr)
	}
}

func requireWithinRecomputeBudget(t *testing.T, first recomputePass, second recomputePass) {
	t.Helper()
	if first.factCount != benchmarkFactCount || second.factCount != benchmarkFactCount {
		t.Fatalf("fact counts = %d/%d, want %d", first.factCount, second.factCount, benchmarkFactCount)
	}
	if first.digest != second.digest {
		t.Fatal("repeated recompute produced a different digest")
	}
	if first.duration > benchmarkDurationBudget || second.duration > benchmarkDurationBudget {
		t.Fatalf("durations = %s/%s, budget %s", first.duration, second.duration, benchmarkDurationBudget)
	}
	peakGrowth := maxUint64(first.heapGrowth, second.heapGrowth)
	if peakGrowth > benchmarkHeapBudget {
		t.Fatalf("heap growth = %d, budget %d", peakGrowth, benchmarkHeapBudget)
	}
}

func heapGrowth(before uint64, after uint64) uint64 {
	if after < before {
		return 0
	}
	return after - before
}

func maxUint64(first uint64, second uint64) uint64 {
	if first > second {
		return first
	}
	return second
}
