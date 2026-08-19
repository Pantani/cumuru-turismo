package external

import (
	"bytes"
	"context"
	"net/http"
	"testing"
)

// This test exists because its absence was the defect. Every earlier assertion
// about the tide card and about Cadastur called the catalogue functions
// directly, so the suite was green while `TideSeries` and `CreditedOnlySources`
// had no production caller at all: after a real cycle the database held one
// source and one series, the tide card did not exist and Cadastur was credited
// nowhere. What has to be asserted is the state of the database after the
// cycle, not the return value of a function the cycle never calls.
func TestCycleSeedsTideAndCreditedSources(t *testing.T) {
	repository, ingestion := forecastIngestion(t, "open_meteo_forecast.json")

	ingestion.RunCycle(context.Background())

	assertTideSeeded(t, repository)
	assertCreditedSeeded(t, repository)
	if _, found := repository.sourceByCode(SourceOpenMeteoForecast); !found {
		t.Fatal("the fetched source is missing from the catalogue")
	}
}

// The tide will never have a fetch, and Cadastur will never have an
// observation. If the catalogue only appeared as a consequence of collection,
// an upstream that is down would take both out of the product entirely.
func TestCatalogueSeedingSurvivesADeadUpstream(t *testing.T) {
	stub := newStubUpstream(t, statusHandler(http.StatusInternalServerError))
	repository, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())

	assertTideSeeded(t, repository)
	assertCreditedSeeded(t, repository)
	if repository.count() != 0 {
		t.Fatalf("a failed cycle stored %d observations", repository.count())
	}
}

// Running the cycle again must not duplicate a row nor rewrite one with a
// different value.
func TestCatalogueSeedingIsIdempotent(t *testing.T) {
	repository, ingestion := forecastIngestion(t, "open_meteo_forecast.json")

	ingestion.RunCycle(context.Background())
	sources, series := len(repository.sources), len(repository.series)
	before, _ := repository.seriesByCode(SourceCHMHarmonics, "tide_extremes")

	ingestion.RunCycle(context.Background())

	if len(repository.sources) != sources || len(repository.series) != series {
		t.Fatalf("second cycle changed the catalogue size: %d/%d then %d/%d",
			sources, series, len(repository.sources), len(repository.series))
	}
	after, _ := repository.seriesByCode(SourceCHMHarmonics, "tide_extremes")
	if after != before {
		t.Fatalf("second cycle rewrote the tide series: %+v then %+v",
			before, after)
	}
}

// A catalogue row that cannot be written does not stop the collection that can.
// The failure is scoped to the seeded sources, because the fetched target
// writes its own catalogue row and a broken database would stop that too — the
// question here is only whether seeding failure is contained.
func TestCatalogueSeedingFailureDoesNotAbortTheCycle(t *testing.T) {
	settings := testSettings()
	logger := testLogger(&bytes.Buffer{})
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	fetcher := newTestFetcher(t, stub, settings, logger)
	repository := &fakeRepository{failSeeds: map[string]bool{
		SourceCHMHarmonics: true,
		SourceCadastur:     true,
	}}
	ingestion := newTestIngestion(
		repository, fetcher, settings, logger,
		[]Target{stubTarget(t, stub)},
	)

	ingestion.RunCycle(context.Background())

	if len(repository.runs) != 1 {
		t.Fatalf("fetch_runs recorded = %d, want 1", len(repository.runs))
	}
	if got := repository.lastRun().Outcome; got != OutcomeOK {
		t.Fatalf("collection outcome = %q, want %q", got, OutcomeOK)
	}
	if repository.count() == 0 {
		t.Fatal("seeding failure stopped the collection that could proceed")
	}
}

// The tide series is seeded in exactly the state the contract needs: a card
// that is born unavailable for a declared structural reason, publicly exposable
// so that it renders at all, and derived because a tide, when it exists, is
// computed by us from harmonic constants.
func assertTideSeeded(t *testing.T, repository *fakeRepository) {
	t.Helper()
	if _, found := repository.sourceByCode(SourceCHMHarmonics); !found {
		t.Fatal("the CHM source was never written to the catalogue")
	}
	series, found := repository.seriesByCode(SourceCHMHarmonics, "tide_extremes")
	if !found {
		t.Fatal("the tide series was never written to the catalogue")
	}
	assertTideShape(t, series)
	assertTideUntouchedByCollection(t, repository)
}

func assertTideShape(t *testing.T, series SeriesRecord) {
	t.Helper()
	if series.CardCode != CardTide || !series.PublicExposable {
		t.Fatalf("the tide series would render no card: %+v", series)
	}
	if series.UnavailableReasonCode != ReasonConstantsNotImported {
		t.Fatalf("tide reason = %q, want %q",
			series.UnavailableReasonCode, ReasonConstantsNotImported)
	}
	if !series.Derived || series.DerivationCode != "tide_harmonic_prediction" {
		t.Fatalf("the tide series does not declare its derivation: %+v", series)
	}
}

// No observation and no run: exactly the state the view represents with its
// three null columns, and the one U-4 fixes until the CHM answers.
func assertTideUntouchedByCollection(t *testing.T, repository *fakeRepository) {
	t.Helper()
	if repository.observationsFor(SourceCHMHarmonics) != 0 {
		t.Fatal("the tide series was given an observation")
	}
	if repository.runsFor(SourceCHMHarmonics) != 0 {
		t.Fatal("the tide source was fetched")
	}
}

// Cadastur is credited and nothing else: no series, therefore no card with a
// value and no published universe (U-7).
func assertCreditedSeeded(t *testing.T, repository *fakeRepository) {
	t.Helper()
	for _, credited := range CreditedOnlySources() {
		assertCreditedOnly(t, repository, credited.SourceCode)
	}
}

func assertCreditedOnly(
	t *testing.T,
	repository *fakeRepository,
	code string,
) {
	t.Helper()
	source, found := repository.sourceByCode(code)
	if !found {
		t.Fatalf("credited source %s is absent", code)
	}
	if source.AttributionText == "" || source.LicenseURL == "" {
		t.Fatalf("credited source %s carries no attribution: %+v", code, source)
	}
	if repository.seriesCountFor(code) != 0 {
		t.Fatalf("credited source %s gained a series", code)
	}
	if repository.observationsFor(code) != 0 {
		t.Fatalf("credited source %s gained an observation", code)
	}
}
