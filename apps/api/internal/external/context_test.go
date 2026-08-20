package external

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublishedCardCarriesUnitSeriesAndProvenance(t *testing.T) {
	now := reference(t)
	document, err := BuildDocument(weatherRows(t, now), creditedSources(), now)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}
	card := document.Cards[0]

	if card.Status != StatusPublished {
		t.Fatalf("status = %q, want %q", card.Status, StatusPublished)
	}
	if card.UnitCode != "celsius" || len(card.Series) != 2 {
		t.Fatalf("unexpected published body: %+v", card)
	}
	if card.ReasonCode != "" {
		t.Fatalf("published card carries reason_code %q", card.ReasonCode)
	}
	assertProvenanceComplete(t, card.Provenance)
	if card.Provenance.Revision != 3 {
		t.Fatalf("revision = %d, want the latest observation's 3",
			card.Provenance.Revision)
	}
}

// Provenance is mandatory in the unavailable branch too. Without it a failing
// card degrades into an anonymous box and the CC-BY attribution obligation
// becomes conditional on a fetch succeeding.
func TestUnavailableCardStillCarriesProvenance(t *testing.T) {
	now := reference(t)
	document, err := BuildDocument(
		[]ContextRow{tideRow()}, creditedSources(), now,
	)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}
	card := document.Cards[0]

	if card.Status != StatusUnavailable {
		t.Fatalf("status = %q, want %q", card.Status, StatusUnavailable)
	}
	if card.ReasonCode != ReasonConstantsNotImported {
		t.Fatalf("reason_code = %q, want %q",
			card.ReasonCode, ReasonConstantsNotImported)
	}
	if card.UnitCode != "" || card.Series != nil {
		t.Fatalf("unavailable card leaked a value: %+v", card)
	}
	assertProvenanceComplete(t, card.Provenance)
	if card.Provenance.Revision != 0 {
		t.Fatalf("revision = %d, want 0 without an observation",
			card.Provenance.Revision)
	}
	if card.Provenance.ObservedAt != "" {
		t.Fatalf("observed_at present without an observation: %q",
			card.Provenance.ObservedAt)
	}
}

// The tide card is born unavailable and no code path unlocks it. It is only
// lawful to call something a tide, and to publish high and low water times,
// when it derives from the harmonic constants of a named CHM station.
func TestTideCardNeverPublishesAValue(t *testing.T) {
	now := reference(t)
	row := tideRow()
	// Even handed an observation, the structural reason outranks it.
	start := now.Add(-2 * time.Hour)
	end := now.Add(-time.Hour)
	value := 1.4
	row.PeriodStart = &start
	row.PeriodEnd = &end
	row.Value = &value
	row.LastFetchOutcome = OutcomeOK

	document, err := BuildDocument([]ContextRow{row}, creditedSources(), now)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}

	card := document.Cards[0]
	if card.Status != StatusUnavailable ||
		card.ReasonCode != ReasonConstantsNotImported {
		t.Fatalf("tide card published: %+v", card)
	}
	if strings.Contains(string(marshal(t, card)), "1.4") {
		t.Fatal("tide card leaked a numeric value")
	}
}

// A dead source is a card, not a document. 503 belongs to the case where the
// whole document cannot be assembled and to nothing else.
func TestDeadSourceDegradesOnlyItsOwnCard(t *testing.T) {
	now := reference(t)
	rows := append(weatherRows(t, now), tideRow())
	rows[0].LastFetchOutcome = OutcomeHTTPError
	rows[1].LastFetchOutcome = OutcomeHTTPError

	document, err := BuildDocument(rows, creditedSources(), now)
	if err != nil {
		t.Fatalf("a dead source brought down the document: %v", err)
	}
	if len(document.Cards) != 2 {
		t.Fatalf("cards = %d, want 2", len(document.Cards))
	}
	if document.Cards[0].Status != StatusPublished {
		t.Fatal("a source with observations stopped publishing")
	}
}

func TestReasonCodesComeFromTheLastRunOutcome(t *testing.T) {
	cases := map[string]string{
		OutcomeOK:            ReasonSourceDataMissing,
		OutcomeUnchanged:     ReasonSourceDataMissing,
		OutcomeRateLimited:   ReasonSourceRateLimited,
		OutcomeHTTPError:     ReasonSourceUnavailable,
		OutcomeParseError:    ReasonSourceUnavailable,
		OutcomeWriteError:    ReasonSourceUnavailable,
		OutcomeSkippedBudget: ReasonSourceUnavailable,
		"":                   ReasonSourceUnavailable,
	}
	now := reference(t)
	for outcome, want := range cases {
		t.Run(outcome, func(t *testing.T) {
			row := weatherRow()
			row.LastFetchOutcome = outcome
			document, err := BuildDocument(
				[]ContextRow{row}, creditedSources(), now,
			)
			if err != nil {
				t.Fatalf("document not assembled: %v", err)
			}
			if got := document.Cards[0].ReasonCode; got != want {
				t.Fatalf("reason_code = %q, want %q", got, want)
			}
		})
	}
}

// Data older than the lag the source itself declares means the source failed to
// publish on time, which is a different fact from a failed fetch.
func TestDataOlderThanTheDeclaredLagIsStale(t *testing.T) {
	now := reference(t)
	rows := weatherRows(t, now.Add(-72*time.Hour))
	document, err := BuildDocument(rows, creditedSources(), now)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}
	if got := document.Cards[0].ReasonCode; got != ReasonStaleBeyondLag {
		t.Fatalf("reason_code = %q, want %q", got, ReasonStaleBeyondLag)
	}
}

// Two series behind one card would put two units and two provenances under a
// single label, so it is refused loudly rather than resolved by picking one.
func TestTwoSeriesUnderOneCardAreRefused(t *testing.T) {
	now := reference(t)
	rows := weatherRows(t, now)
	rows[1].SeriesCode = "temperature_2m_min"

	if _, err := BuildDocument(rows, creditedSources(), now); err == nil {
		t.Fatal("two series under one card were accepted")
	}
}

func TestEmptyLayerCannotBeAssembled(t *testing.T) {
	now := reference(t)
	if _, err := BuildDocument(nil, creditedSources(), now); err == nil {
		t.Fatal("a document with no card was assembled")
	}
	if _, err := BuildDocument(weatherRows(t, now), nil, now); err == nil {
		t.Fatal("a document with no credited source was assembled")
	}
}

// The layer carries no coverage, no ratio, no sample size and no accommodation
// count. No surface may combine an external number with a protected cell, and
// the payload gives nobody the operands to try.
func TestPayloadCarriesNoProtectedSeriesVocabulary(t *testing.T) {
	now := reference(t)
	document, err := BuildDocument(
		append(weatherRows(t, now), tideRow()), creditedSources(), now,
	)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}
	body := strings.ToLower(string(marshal(t, document)))
	for _, forbidden := range []string{
		"coverage", "ratio", "sample_size", "accommodation",
		"protected", "privacy_policy_version", "suppress",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("payload carries %q", forbidden)
		}
	}
}

// data_mode is per card. A page that mixes real weather with fictional presence
// under one global label lies in both directions.
func TestDocumentCarriesNoGlobalDataMode(t *testing.T) {
	now := reference(t)
	document, err := BuildDocument(weatherRows(t, now), creditedSources(), now)
	if err != nil {
		t.Fatalf("document not assembled: %v", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(marshal(t, document), &decoded); err != nil {
		t.Fatalf("document not decodable: %v", err)
	}
	if _, present := decoded["data_mode"]; present {
		t.Fatal("document carries a global data_mode")
	}
	if document.Cards[0].DataMode == "" {
		t.Fatal("card carries no data_mode of its own")
	}
	if document.Layer != LayerCode ||
		document.DisclaimerCode != DisclaimerCode {
		t.Fatalf("document constants drifted: %+v", document)
	}
}

func assertProvenanceComplete(t *testing.T, provenance Provenance) {
	t.Helper()
	missing := []string{}
	for name, value := range map[string]string{
		"source_code":      provenance.SourceCode,
		"publisher":        provenance.Publisher,
		"license_code":     provenance.LicenseCode,
		"license_url":      provenance.LicenseURL,
		"attribution_text": provenance.AttributionText,
		"terms_url":        provenance.TermsURL,
		"retrieved_at":     provenance.RetrievedAt,
		"covered_start":    provenance.CoveredPeriod.Start,
		"covered_end":      provenance.CoveredPeriod.End,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("provenance is missing %v", missing)
	}
	if !provenance.CoveredPeriod.EndExclusive ||
		provenance.CoveredPeriod.TimeZone != PublicTimeZone {
		t.Fatalf("covered period is not a half-open Bahia period: %+v",
			provenance.CoveredPeriod)
	}
}

func reference(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
}

func weatherRow() ContextRow {
	return ContextRow{
		CardCode:           CardWeatherDaily,
		SourceCode:         SourceOpenMeteoForecast,
		SeriesCode:         "temperature_2m_max",
		UnitCode:           "celsius",
		DataMode:           "real_source",
		DeclaredLagSeconds: int64((6 * time.Hour).Seconds()),
		Publisher:          "Open-Meteo",
		LicenseCode:        "CC-BY-4.0",
		LicenseURL:         "https://creativecommons.org/licenses/by/4.0/",
		AttributionText:    "Dados meteorológicos por Open-Meteo.com",
		TermsURL:           "https://open-meteo.com/en/terms",
		LastFetchOutcome:   OutcomeOK,
	}
}

func weatherRows(t *testing.T, latest time.Time) []ContextRow {
	t.Helper()
	rows := []ContextRow{}
	for index, revision := range []int32{1, 3} {
		row := weatherRow()
		start := latest.Add(time.Duration(index-1) * 24 * time.Hour)
		end := start.Add(24 * time.Hour)
		value := 26.0 + float64(index)
		retrieved := latest
		row.PeriodStart = &start
		row.PeriodEnd = &end
		row.Value = &value
		row.RetrievedAt = &retrieved
		row.Revision = revision
		rows = append(rows, row)
	}
	return rows
}

func tideRow() ContextRow {
	source, series := TideSeries()
	return ContextRow{
		CardCode:              series.CardCode,
		SourceCode:            series.SourceCode,
		SeriesCode:            series.SeriesCode,
		UnitCode:              series.UnitCode,
		DataMode:              series.DataMode,
		Derived:               series.Derived,
		DerivationCode:        series.DerivationCode,
		UnavailableReasonCode: series.UnavailableReasonCode,
		Publisher:             source.Publisher,
		LicenseCode:           source.LicenseCode,
		LicenseURL:            source.LicenseURL,
		AttributionText:       source.AttributionText,
		TermsURL:              source.TermsURL,
	}
}

func creditedSources() []CreditedSource {
	return []CreditedSource{{
		SourceCode:      SourceCadastur,
		Publisher:       "Ministério do Turismo",
		LicenseCode:     "LicenseRef-Cadastur-Termos-de-Uso",
		LicenseURL:      "https://cadastur.turismo.gov.br/",
		AttributionText: "Cadastur, Ministério do Turismo",
		TermsURL:        "https://cadastur.turismo.gov.br/",
	}}
}

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("payload not encodable: %v", err)
	}
	return body
}
