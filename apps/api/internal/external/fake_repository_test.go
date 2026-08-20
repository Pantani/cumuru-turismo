package external

import (
	"context"
	"errors"
	"sync"
	"time"
)

// fakeRepository reproduces the two invariants the migration enforces: the
// unique index on (source, series, period, digest), which makes an identical
// fact a no-op, and max(revision)+1, which makes a changed fact a new revision.
// Everything else is bookkeeping the assertions read.
type fakeRepository struct {
	mutex        sync.Mutex
	observations []ObservationRecord
	runs         []RunResult
	started      []RunStart
	sources      []SourceRecord
	series       []SeriesRecord
	failWrites   bool
	failStart    bool
	// failSeeds names the sources whose catalogue write fails, so a test can
	// break the seeding of one source without breaking the catalogue write the
	// fetched target does for itself.
	failSeeds map[string]bool
}

var errFakeStorage = errors.New("storage unavailable")

// The two Ensure methods reproduce ON CONFLICT DO UPDATE by key, so a repeated
// cycle overwrites the row in place instead of adding another one — which is
// what makes "idempotent" assertable here rather than assumed.
func (r *fakeRepository) EnsureSource(_ context.Context, source SourceRecord) error {
	if r.failSeeds[source.SourceCode] {
		return errFakeStorage
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for index, existing := range r.sources {
		if existing.SourceCode == source.SourceCode {
			r.sources[index] = source
			return nil
		}
	}
	r.sources = append(r.sources, source)
	return nil
}

func (r *fakeRepository) EnsureSeries(_ context.Context, series SeriesRecord) error {
	if r.failSeeds[series.SourceCode] {
		return errFakeStorage
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for index, existing := range r.series {
		if existing.SourceCode == series.SourceCode &&
			existing.SeriesCode == series.SeriesCode {
			r.series[index] = series
			return nil
		}
	}
	r.series = append(r.series, series)
	return nil
}

func (r *fakeRepository) sourceByCode(code string) (SourceRecord, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, source := range r.sources {
		if source.SourceCode == code {
			return source, true
		}
	}
	return SourceRecord{}, false
}

func (r *fakeRepository) seriesByCode(source, code string) (SeriesRecord, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, series := range r.series {
		if series.SourceCode == source && series.SeriesCode == code {
			return series, true
		}
	}
	return SeriesRecord{}, false
}

func (r *fakeRepository) seriesCountFor(source string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	total := 0
	for _, series := range r.series {
		if series.SourceCode == source {
			total++
		}
	}
	return total
}

// observationsFor and runsFor exist so a test can assert that a series has
// neither — the state the tide card lives in permanently, and the one the view
// represents with its three null columns.
func (r *fakeRepository) observationsFor(source string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	total := 0
	for _, record := range r.observations {
		if record.Key.SourceCode == source {
			total++
		}
	}
	return total
}

func (r *fakeRepository) runsFor(source string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	total := 0
	for _, run := range r.started {
		if run.SourceCode == source {
			total++
		}
	}
	return total
}

func (r *fakeRepository) StartRun(_ context.Context, run RunStart) error {
	if r.failStart {
		return errFakeStorage
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.started = append(r.started, run)
	return nil
}

func (r *fakeRepository) FinishRun(_ context.Context, result RunResult) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.runs = append(r.runs, result)
	return nil
}

func (r *fakeRepository) NextRevision(
	_ context.Context,
	key ObservationKey,
) (int32, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	highest := int32(0)
	for _, record := range r.observations {
		if sameKey(record.Key, key) && record.Revision > highest {
			highest = record.Revision
		}
	}
	return highest + 1, nil
}

// Espelha o índice único do banco: digest repetido é no-op e devolve zero
// linha, digest novo devolve uma. O fake mente se devolver 1 no conflito.
func (r *fakeRepository) InsertObservation(
	_ context.Context,
	record ObservationRecord,
) (int64, error) {
	if r.failWrites {
		return 0, errFakeStorage
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.duplicate(record) {
		return 0, nil
	}
	r.observations = append(r.observations, record)
	return 1, nil
}

func (r *fakeRepository) duplicate(candidate ObservationRecord) bool {
	for _, record := range r.observations {
		if sameKey(record.Key, candidate.Key) &&
			record.PayloadDigest == candidate.PayloadDigest {
			return true
		}
	}
	return false
}

func (r *fakeRepository) DeleteExpiredObservations(
	_ context.Context,
	_ time.Time,
	_ int32,
) (int64, error) {
	return 0, nil
}

func sameKey(left, right ObservationKey) bool {
	return left.SourceCode == right.SourceCode &&
		left.SeriesCode == right.SeriesCode &&
		left.PeriodStart.Equal(right.PeriodStart)
}

func (r *fakeRepository) lastRun() RunResult {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if len(r.runs) == 0 {
		return RunResult{}
	}
	return r.runs[len(r.runs)-1]
}

func (r *fakeRepository) revisionOf(digest string) int32 {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, record := range r.observations {
		if record.PayloadDigest == digest {
			return record.Revision
		}
	}
	return 0
}

func (r *fakeRepository) count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return len(r.observations)
}

var _ Repository = (*fakeRepository)(nil)
