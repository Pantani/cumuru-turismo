package external

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Two identical cycles write the same rows. Idempotence is decided by the
// unique index on (source, series, period, digest), so a repeated fact is a
// no-op and the second run reports `unchanged` rather than claiming writes.
func TestIngestionIsIdempotentAcrossCycles(t *testing.T) {
	repository, ingestion := forecastIngestion(t, "open_meteo_forecast.json")
	ingestion.RunCycle(context.Background())
	first := repository.count()
	firstOutcome := repository.lastRun().Outcome

	ingestion.RunCycle(context.Background())

	if repository.count() != first {
		t.Fatalf("second cycle changed the row count: %d then %d",
			first, repository.count())
	}
	if firstOutcome != OutcomeOK {
		t.Fatalf("first cycle outcome = %q, want %q", firstOutcome, OutcomeOK)
	}
	if got := repository.lastRun().Outcome; got != OutcomeUnchanged {
		t.Fatalf("second cycle outcome = %q, want %q", got, OutcomeUnchanged)
	}
	if got := repository.lastRun().ObservationsWritten; got != 0 {
		t.Fatalf("second cycle wrote %d observations, want 0", got)
	}
}

// A different digest for the same period enters as revision max+1. ERA5
// backfills data that was already published, so a revision is a recorded fact
// and never an in-place overwrite that erases what was served before.
func TestIngestionRecordsRevisionForChangedDigest(t *testing.T) {
	body := fixture(t, "open_meteo_forecast.json")
	revised := fixture(t, "open_meteo_forecast_revised.json")
	handler := newSwitchableHandler(bodyHandler(body))
	stub := newStubUpstream(t, handler.serve)
	repository, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())
	before := repository.count()
	handler.switchTo(bodyHandler(revised))
	ingestion.RunCycle(context.Background())

	if repository.count() != before+1 {
		t.Fatalf("revision did not add exactly one row: %d then %d",
			before, repository.count())
	}
	target := stubTarget(t, stub)
	changed := digestOf(target, ObservedPoint{
		PeriodStart: civilDay(t, "2026-08-12"), Value: 27.9,
	})
	if got := repository.revisionOf(changed); got != 2 {
		t.Fatalf("revised observation revision = %d, want 2", got)
	}
	original := digestOf(target, ObservedPoint{
		PeriodStart: civilDay(t, "2026-08-12"), Value: 27.1,
	})
	if got := repository.revisionOf(original); got != 1 {
		t.Fatalf("original observation revision = %d, want 1", got)
	}
}

// Every failure ends the cycle, records the run, leaves the worker running and
// never erases the last valid observation. The card degrades; the layer does
// not.
func TestIngestionFailuresNeverAbortTheCycle(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		outcome string
	}{
		{"upstream 500", statusHandler(http.StatusInternalServerError), OutcomeHTTPError},
		{"rate limited", statusHandler(http.StatusTooManyRequests), OutcomeRateLimited},
		{"timeout", slowHandler(2 * time.Second), OutcomeHTTPError},
		{"truncated body", truncatedHandler(), OutcomeHTTPError},
		{"oversized body", oversizedHandler(), OutcomeHTTPError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertFailureIsContained(t, testCase.handler, testCase.outcome)
		})
	}
}

// An unparseable payload is parse_error, never silence: a missing required
// field or an unexpected type is a fact about the upstream and it is recorded
// as one.
func TestIngestionRejectsInvalidPayloads(t *testing.T) {
	for _, name := range []string{
		"open_meteo_missing_daily.json",
		"open_meteo_wrong_type.json",
	} {
		t.Run(name, func(t *testing.T) {
			assertFailureIsContained(t, fixtureHandler(t, name), OutcomeParseError)
		})
	}
}

// The previous observation survives the failure. "Unavailable" is a statement
// about the last run, not a reason to delete what was already true.
func assertFailureIsContained(
	t *testing.T,
	failing http.HandlerFunc,
	want string,
) {
	t.Helper()
	handler := newSwitchableHandler(fixtureHandler(t, "open_meteo_forecast.json"))
	stub := newStubUpstream(t, handler.serve)
	repository, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())
	stored := repository.count()
	handler.switchTo(failing)
	ingestion.RunCycle(context.Background())

	if got := repository.lastRun().Outcome; got != want {
		t.Fatalf("outcome = %q, want %q", got, want)
	}
	if repository.count() != stored {
		t.Fatalf("failed cycle changed stored observations: %d then %d",
			stored, repository.count())
	}
	if len(repository.runs) != 2 {
		t.Fatalf("fetch_runs recorded = %d, want 2", len(repository.runs))
	}
}

// A null inside the daily array is the source saying it has no number for that
// day. It is skipped rather than stored as a half fact, and it does not turn a
// well-formed payload into a parse error.
func TestIngestionSkipsNullDaysWithoutFailing(t *testing.T) {
	repository, ingestion := forecastIngestion(
		t, "open_meteo_forecast_with_gap.json",
	)
	ingestion.RunCycle(context.Background())

	if repository.count() != 1 {
		t.Fatalf("stored %d observations, want 1", repository.count())
	}
	if got := repository.lastRun().Outcome; got != OutcomeOK {
		t.Fatalf("outcome = %q, want %q", got, OutcomeOK)
	}
}

// The cycle is bounded by its own budget. DATABASE_TIMEOUT sizes a request and
// stays at its 3 s default here precisely to show it plays no part.
func TestCycleRespectsItsOwnBatchBudget(t *testing.T) {
	settings := testSettings()
	settings.RequestTimeout = 3 * time.Second
	settings.BatchBudget = 150 * time.Millisecond
	stub := newStubUpstream(t, slowHandler(2*time.Second))
	logger := testLogger(&bytes.Buffer{})
	fetcher := newTestFetcher(t, stub, settings, logger)
	repository := &fakeRepository{}
	ingestion := newTestIngestion(
		repository, fetcher, settings, logger,
		[]Target{stubTarget(t, stub)},
	)

	started := time.Now()
	ingestion.RunCycle(context.Background())
	elapsed := time.Since(started)

	if elapsed > time.Second {
		t.Fatalf("cycle ran for %s, beyond its own budget", elapsed)
	}
	if len(repository.runs) != 1 {
		t.Fatalf("fetch_runs recorded = %d, want 1", len(repository.runs))
	}
}

// Three consecutive failures open the breaker, and while it is open no run is
// fabricated: the previous failing run stays the latest, so the card keeps the
// reason that is actually true.
func TestBreakerStopsAskingAndFabricatesNoRun(t *testing.T) {
	stub := newStubUpstream(t, statusHandler(http.StatusInternalServerError))
	repository, ingestion := ingestionOver(t, stub)

	for range breakerFailureThreshold {
		ingestion.RunCycle(context.Background())
	}
	recorded := len(repository.runs)
	ingestion.RunCycle(context.Background())

	if len(repository.runs) != recorded {
		t.Fatalf("breaker fabricated a run: %d then %d",
			recorded, len(repository.runs))
	}
	if got := len(stub.recorded()); got != breakerFailureThreshold {
		t.Fatalf("upstream called %d times, want %d",
			got, breakerFailureThreshold)
	}
}

// A database failure is `write_error` and never `http_error`. The distinction is
// the whole reason the value exists: recording a storage failure as a network
// one sends whoever debugs it to the wrong subsystem, and `fetch_runs` earns its
// place by naming causes rather than by counting failures.
func TestStorageFailureIsWriteErrorNotHTTPError(t *testing.T) {
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	repository, ingestion := ingestionOver(t, stub)

	repository.failWrites = true
	ingestion.RunCycle(context.Background())

	run := repository.lastRun()
	if run.Outcome != OutcomeWriteError {
		t.Fatalf("outcome = %q, want %q", run.Outcome, OutcomeWriteError)
	}
	if run.Outcome == OutcomeHTTPError {
		t.Fatal("a database failure was recorded as a network failure")
	}
	// The source did respond and was read: the HTTP status belongs in the row,
	// and it is what separates this from a fetch that never got a body.
	if run.HTTPStatus == nil || *run.HTTPStatus != http.StatusOK {
		t.Fatalf("http_status = %v, want 200", run.HTTPStatus)
	}
	if run.BatchBudgetExhausted {
		t.Fatal("a storage failure was reported as budget exhaustion")
	}
	if repository.count() != 0 {
		t.Fatalf("stored %d observations, want 0", repository.count())
	}
}

// A fetch that never got a response has no HTTP status to record. Persisting 0
// would invent a code the protocol does not have and send whoever debugs it
// hunting for a server that answered, when none did. The column stays nullable
// precisely so that "never answered" and "answered 500" remain distinguishable
// in the trail — the same reason write_error exists apart from http_error.
func TestFetchWithoutResponseRecordsNoHTTPStatus(t *testing.T) {
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	repository, ingestion := ingestionOver(t, stub)
	// Upstream goes away before the cycle: no connection, no body, no status.
	stub.server.Close()

	ingestion.RunCycle(context.Background())

	run := repository.lastRun()
	if run.Outcome != OutcomeHTTPError {
		t.Fatalf("outcome = %q, want %q", run.Outcome, OutcomeHTTPError)
	}
	if run.HTTPStatus != nil {
		t.Fatalf(
			"http_status = %d, want nil when the upstream never responded",
			*run.HTTPStatus,
		)
	}
}

// A write that fails before the run row exists is still a write failure. It
// never reaches `fetch_runs` — there is no row to carry it — so the documented
// meaning of `write_error`, "the source responded and was read", stays exactly
// true of every persisted row, while the metric stops blaming the network for a
// database that was unreachable.
func TestUnstartableRunIsWriteErrorInTheMetricOnly(t *testing.T) {
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	repository, ingestion := ingestionOver(t, stub)

	repository.failStart = true
	ingestion.RunCycle(context.Background())

	if len(repository.runs) != 0 {
		t.Fatalf("fetch_runs recorded = %d, want 0", len(repository.runs))
	}
	if got := len(stub.recorded()); got != 0 {
		t.Fatalf("upstream called %d times before a run existed, want 0", got)
	}
}

// The budget still outranks the storage label: a cycle cut short by its own
// deadline is `skipped_budget`, because the write did not fail — it was never
// given time to run.
func TestExhaustedBudgetOutranksTheStorageLabel(t *testing.T) {
	expired, cancel := context.WithCancel(context.Background())
	cancel()

	if got := storageOutcome(expired); got != OutcomeSkippedBudget {
		t.Fatalf("expired budget outcome = %q, want %q",
			got, OutcomeSkippedBudget)
	}
	if got := storageOutcome(context.Background()); got != OutcomeWriteError {
		t.Fatalf("storage failure outcome = %q, want %q",
			got, OutcomeWriteError)
	}
}

func forecastIngestion(
	t *testing.T,
	name string,
) (*fakeRepository, *Ingestion) {
	t.Helper()
	return ingestionOver(t, newStubUpstream(t, fixtureHandler(t, name)))
}

func ingestionOver(
	t *testing.T,
	stub *stubUpstream,
) (*fakeRepository, *Ingestion) {
	t.Helper()
	settings := testSettings()
	logger := testLogger(&bytes.Buffer{})
	fetcher := newTestFetcher(t, stub, settings, logger)
	repository := &fakeRepository{}
	ingestion := newTestIngestion(
		repository, fetcher, settings, logger,
		[]Target{stubTarget(t, stub)},
	)
	return repository, ingestion
}

func civilDay(t *testing.T, day string) time.Time {
	t.Helper()
	value, err := time.ParseInLocation(time.DateOnly, day, bahia)
	if err != nil {
		t.Fatalf("civil day %s invalid: %v", day, err)
	}
	return value
}

func bodyHandler(body []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(body)
	}
}

func statusHandler(status int) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
	}
}

func slowHandler(delay time.Duration) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(delay):
			writer.WriteHeader(http.StatusOK)
		case <-request.Context().Done():
		}
	}
}

func truncatedHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "512")
		_, _ = writer.Write([]byte(`{"daily":{"time":["2026-08-11"]`))
		panic(http.ErrAbortHandler)
	}
}

func oversizedHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(
			`{"daily":{"time":[],"temperature_2m_max":[]},"pad":"` +
				strings.Repeat("x", 128*1024) + `"}`,
		))
	}
}
