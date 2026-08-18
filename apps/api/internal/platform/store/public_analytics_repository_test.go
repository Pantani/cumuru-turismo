package store

import (
	"context"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type publicAnalyticsQueries struct {
	generated.Querier
	presence    []generated.PublicDataCurrentPresence
	methodology generated.GetCurrentMethodologyRow
}

func (f *publicAnalyticsQueries) ListCurrentPresenceCells(
	context.Context,
	string,
) ([]generated.PublicDataCurrentPresence, error) {
	return f.presence, nil
}

func (f *publicAnalyticsQueries) ListCurrentPresenceCellsForRecentDays(
	context.Context,
	generated.ListCurrentPresenceCellsForRecentDaysParams,
) ([]generated.PublicDataCurrentPresence, error) {
	return f.presence, nil
}

func (f *publicAnalyticsQueries) ListCurrentPresenceCellsInRange(
	context.Context,
	generated.ListCurrentPresenceCellsInRangeParams,
) ([]generated.PublicDataCurrentPresence, error) {
	return f.presence, nil
}

func (f *publicAnalyticsQueries) GetCurrentMethodology(
	context.Context,
) (generated.GetCurrentMethodologyRow, error) {
	return f.methodology, nil
}

func TestPublicPresenceMapsOnlyProtectedSnapshotFields(t *testing.T) {
	t.Parallel()

	asOf := pgtype.Date{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	updatedAt := pgtype.Timestamptz{
		Time: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC), Valid: true,
	}
	queries := &publicAnalyticsQueries{presence: []generated.PublicDataCurrentPresence{{
		PeriodSelector: analytics.PresenceObservedSelector,
		PeriodStart:    asOf, PeriodEnd: nextDate(asOf), Unit: "person_day",
		Kind: "observed", Status: "protected", AsOfOn: asOf,
		DataMode: "prototype_fixtures", PrivacyPolicyVersion: "prototype-v1",
		MethodologyVersion: "explainable-baseline-v1", CoverageStatus: "protected",
		PublishedAt: updatedAt,
	}}}
	subject := &PublicAnalyticsRepository{queries: queries, timeout: time.Second}

	slice, _ := analytics.ResolvePresenceWindow("recent_30_days", "")
	got, err := subject.Presence(context.Background(), slice)
	if err != nil {
		t.Fatalf("Presence() error = %v", err)
	}
	if got.Series[0].Status != "protected" || got.Series[0].Value != nil {
		t.Fatalf("Presence() = %#v", got)
	}
	if got.Metadata.Coverage.Ratio != nil || got.Metadata.Period.TimeZone != publicTimeZone {
		t.Fatalf("metadata = %#v", got.Metadata)
	}
}

func TestPublicPresenceRejectsKindMismatchedWithWindow(t *testing.T) {
	t.Parallel()

	asOf := pgtype.Date{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	for _, test := range []struct {
		name     string
		window   string
		month    string
		selector string
		kind     string
	}{
		{
			name: "forecast in recent window", window: "recent_30_days",
			selector: analytics.PresenceObservedSelector, kind: "forecast",
		},
		{
			name: "forecast in a civil month", window: "month", month: "2026-07",
			selector: analytics.PresenceObservedSelector, kind: "forecast",
		},
		{
			name: "observed in future window", window: "next_30_days",
			selector: analytics.PresenceForecastSelector, kind: "observed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			queries := &publicAnalyticsQueries{presence: []generated.PublicDataCurrentPresence{{
				PeriodSelector: test.selector,
				PeriodStart:    asOf, PeriodEnd: nextDate(asOf), Unit: "person_day",
				Kind: test.kind, Status: "protected", AsOfOn: asOf,
				DataMode: "prototype_fixtures", PrivacyPolicyVersion: "prototype-v1",
				MethodologyVersion: "explainable-baseline-v1", CoverageStatus: "protected",
				PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}}}
			subject := &PublicAnalyticsRepository{queries: queries, timeout: time.Second}

			slice, ok := analytics.ResolvePresenceWindow(test.window, test.month)
			if !ok {
				t.Fatalf("ResolvePresenceWindow(%q, %q) rejected", test.window, test.month)
			}
			if _, err := subject.Presence(context.Background(), slice); err == nil {
				t.Fatal("Presence() error = nil")
			}
		})
	}
}

func TestForecastPeakFailsClosedForMixedHorizon(t *testing.T) {
	t.Parallel()

	asOf := pgtype.Date{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	rows := forecastSummaryRows(asOf)
	rows[8].Status = "protected"
	rows[8].PublishedLower = nil
	rows[8].PublishedCentral = nil
	rows[8].PublishedUpper = nil

	got := forecastPeak(rows, asOf)
	if got.Status != "protected" || got.Date != "" ||
		got.Lower != nil || got.Central != nil || got.Upper != nil {
		t.Fatalf("forecastPeak(mixed) = %#v", got)
	}

	rows[8].Status = "published"
	got = forecastPeak(rows, asOf)
	if got.Status != "unavailable" || got.Date != "" {
		t.Fatalf("forecastPeak(invalid central) = %#v", got)
	}
}

func TestForecastPeakRequiresCompletePublishedHorizon(t *testing.T) {
	t.Parallel()

	asOf := pgtype.Date{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	rows := forecastSummaryRows(asOf)
	highest := int32(90)
	rows[12].PublishedCentral = &highest
	rows[12].PublishedUpper = &highest

	got := forecastPeak(rows, asOf)
	if got.Status != "published" || got.Date != "2026-08-10" ||
		got.Central == nil || *got.Central != highest {
		t.Fatalf("forecastPeak(complete) = %#v", got)
	}

	got = forecastPeak(rows[:29], asOf)
	if got.Status != "unavailable" || got.Date != "" {
		t.Fatalf("forecastPeak(incomplete) = %#v", got)
	}
}

func TestPublicMethodologyIncludesNormalAndFallbackForecastBounds(t *testing.T) {
	t.Parallel()

	asOf := pgtype.Date{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), Valid: true}
	queries := &publicAnalyticsQueries{methodology: generated.GetCurrentMethodologyRow{
		AsOfOn: asOf, DataMode: "prototype_fixtures",
		PrivacyPolicyVersion: "prototype-v1",
		MethodologyVersion:   "explainable-baseline-v1",
		CoverageStatus:       "protected",
		PublishedAt: pgtype.Timestamptz{
			Time: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC), Valid: true,
		},
		PresenceInterval:               "[arrival,departure)",
		TimeZone:                       "America/Bahia",
		ObservedDefinitionCode:         "checked_presence_through_as_of",
		ForecastDefinitionCode:         "explainable-baseline-v1",
		ForecastLowerPercent:           85,
		ForecastUpperPercent:           115,
		ForecastFallbackLowerPercent:   70,
		ForecastFallbackUpperPercent:   130,
		PrimaryThreshold:               10,
		MinimumReportingAccommodations: 3,
		ComplementarySuppression:       true,
		RoundingBase:                   10,
		RoundingMode:                   "stable-half-up",
		PresenceHistoryDays:            analytics.PresenceHistoryDays,
		AllowedPresenceWindows:         analytics.AllowedPresenceWindows(),
		AllowedPreferencePeriods:       []string{"last_complete_month"},
	}}
	subject := &PublicAnalyticsRepository{queries: queries, timeout: time.Second}

	got, err := subject.Methodology(context.Background())
	if err != nil {
		t.Fatalf("Methodology() error = %v", err)
	}
	if len(got.ForecastBoundsPercent) != 2 ||
		got.ForecastBoundsPercent[0] != 85 || got.ForecastBoundsPercent[1] != 115 {
		t.Fatalf("forecast bounds = %v", got.ForecastBoundsPercent)
	}
	if len(got.ForecastFallbackBoundsPercent) != 2 ||
		got.ForecastFallbackBoundsPercent[0] != 70 ||
		got.ForecastFallbackBoundsPercent[1] != 130 {
		t.Fatalf("fallback bounds = %v", got.ForecastFallbackBoundsPercent)
	}

	queries.methodology.ForecastFallbackLowerPercent = 0
	if _, err := subject.Methodology(context.Background()); err == nil {
		t.Fatal("Methodology(invalid fallback bounds) error = nil")
	}
}

func forecastSummaryRows(asOf pgtype.Date) []generated.PublicDataCurrentSummary {
	rows := make([]generated.PublicDataCurrentSummary, 30)
	for index := range rows {
		value := int32(10 + index)
		rows[index] = generated.PublicDataCurrentSummary{
			PeriodSelector: "next_30_days",
			PeriodStart: pgtype.Date{
				Time: asOf.Time.AddDate(0, 0, index+1), Valid: true,
			},
			Kind: "forecast", Status: "published",
			PublishedLower: &value, PublishedCentral: &value, PublishedUpper: &value,
		}
	}
	return rows
}
