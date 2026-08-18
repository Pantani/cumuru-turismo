package analytics

import "testing"

func TestResolvePresenceWindowAcceptsEveryPublishedWindow(t *testing.T) {
	t.Parallel()

	for _, window := range AllowedPresenceWindows() {
		month := ""
		if window == "month" {
			month = "2026-02"
		}
		slice, ok := ResolvePresenceWindow(window, month)
		if !ok {
			t.Fatalf("ResolvePresenceWindow(%q, %q) rejected", window, month)
		}
		if slice.Window != window || slice.MaxSeriesLength() <= 0 {
			t.Fatalf("ResolvePresenceWindow(%q) = %#v", window, slice)
		}
	}
}

func TestResolvePresenceWindowSlicesTheObservedSeries(t *testing.T) {
	t.Parallel()

	slice, ok := ResolvePresenceWindow("recent_730_days", "")
	if !ok || slice.Selector != PresenceObservedSelector {
		t.Fatalf("recent_730_days = %#v, ok = %v", slice, ok)
	}
	if slice.LookbackDays != PresenceHistoryDays || slice.Month != "" {
		t.Fatalf("recent_730_days slice = %#v", slice)
	}
}

func TestResolvePresenceWindowBoundsTheCivilMonth(t *testing.T) {
	t.Parallel()

	slice, ok := ResolvePresenceWindow("month", "2026-02")
	if !ok || slice.Selector != PresenceObservedSelector {
		t.Fatalf("month = %#v, ok = %v", slice, ok)
	}
	if slice.MonthStart != "2026-02-01" || slice.MonthEnd != "2026-03-01" {
		t.Fatalf("month bounds = %#v", slice)
	}
	// A janela sozinha não distingue dois meses, então a chave carrega a data.
	if slice.CacheKey() != "month:2026-02" {
		t.Fatalf("CacheKey() = %q", slice.CacheKey())
	}
}

func TestResolvePresenceWindowLeavesTheForecastWhole(t *testing.T) {
	t.Parallel()

	slice, ok := ResolvePresenceWindow("next_30_days", "")
	if !ok || slice.Selector != PresenceForecastSelector {
		t.Fatalf("next_30_days = %#v, ok = %v", slice, ok)
	}
	if slice.LookbackDays != 0 || slice.MaxSeriesLength() != PresenceForecastDays {
		t.Fatalf("next_30_days slice = %#v", slice)
	}
}

// Um mês fora da janela de mês seria um parâmetro ignorado em silêncio, e a
// janela de mês sem mês não nomeia documento nenhum.
func TestResolvePresenceWindowRefusesInconsistentPairs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ window, month string }{
		{window: "recent_30_days", month: "2026-02"},
		{window: "next_30_days", month: "2026-02"},
		{window: "month", month: ""},
		{window: "month", month: "2026-13"},
		{window: "month", month: "2026-00"},
		{window: "month", month: "2026-2"},
		{window: "month", month: "2026-02-01"},
		{window: "recent_45_days", month: ""},
		{window: "observed_daily", month: ""},
		{window: "", month: ""},
	} {
		if _, ok := ResolvePresenceWindow(test.window, test.month); ok {
			t.Fatalf("ResolvePresenceWindow(%q, %q) accepted", test.window, test.month)
		}
	}
}
