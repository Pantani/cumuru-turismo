package store

import (
	"context"
	"sort"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const publicTimeZone = "America/Bahia"

type PublicAnalyticsRepository struct {
	queries generated.Querier
	timeout time.Duration
}

func NewPublicAnalyticsRepository(
	pool *pgxpool.Pool,
	timeout time.Duration,
) *PublicAnalyticsRepository {
	return &PublicAnalyticsRepository{queries: generated.New(pool), timeout: timeout}
}

var _ analytics.PublicReader = (*PublicAnalyticsRepository)(nil)

func (r *PublicAnalyticsRepository) Summary(ctx context.Context) (analytics.PublicSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	rows, err := r.queries.ListCurrentSummaryCells(ctx)
	if err != nil || len(rows) == 0 {
		return analytics.PublicSummary{}, analytics.ErrPublicUnavailable
	}
	today, forecast, err := summaryPoints(rows)
	if err != nil {
		return analytics.PublicSummary{}, err
	}
	metadata, err := summaryMetadata(rows)
	if err != nil {
		return analytics.PublicSummary{}, err
	}
	return analytics.PublicSummary{
		Metadata:                   metadata,
		PresenceToday:              today,
		ForecastPeakNextThirtyDays: forecast,
	}, nil
}

func (r *PublicAnalyticsRepository) Presence(
	ctx context.Context,
	slice analytics.PresenceSlice,
) (analytics.PublicPresence, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	rows, err := r.presenceRows(ctx, slice)
	if err != nil || !presenceRowCount(len(rows), slice) {
		return analytics.PublicPresence{}, analytics.ErrPublicUnavailable
	}
	metadata, err := presenceMetadata(rows)
	if err != nil {
		return analytics.PublicPresence{}, err
	}
	series, err := presenceSeries(rows, slice)
	if err != nil {
		return analytics.PublicPresence{}, err
	}
	return analytics.PublicPresence{
		Metadata: metadata, Window: slice.Window, Month: slice.Month, Series: series,
	}, nil
}

// A previsão é publicada já no horizonte e não tem o que recortar; a série
// observada é longa e sempre chega por um dos dois recortes.
func (r *PublicAnalyticsRepository) presenceRows(
	ctx context.Context,
	slice analytics.PresenceSlice,
) ([]generated.PublicDataCurrentPresence, error) {
	if slice.Selector == analytics.PresenceForecastSelector {
		return r.queries.ListCurrentPresenceCells(ctx, slice.Selector)
	}
	if slice.Month != "" {
		return r.queries.ListCurrentPresenceCellsInRange(
			ctx,
			generated.ListCurrentPresenceCellsInRangeParams{
				PeriodSelector: slice.Selector,
				StartOn:        dateToPG(slice.MonthStart),
				EndOn:          dateToPG(slice.MonthEnd),
			},
		)
	}
	return r.queries.ListCurrentPresenceCellsForRecentDays(
		ctx,
		generated.ListCurrentPresenceCellsForRecentDaysParams{
			PeriodSelector: slice.Selector,
			LookbackDays:   slice.LookbackDays,
		},
	)
}

// Um documento mais longo que a janela pedida é seletor corrompido, não excesso
// a truncar: o recorte já aconteceu no banco.
func presenceRowCount(count int, slice analytics.PresenceSlice) bool {
	return count > 0 && count <= slice.MaxSeriesLength()
}

func presenceSeries(
	rows []generated.PublicDataCurrentPresence,
	slice analytics.PresenceSlice,
) ([]analytics.PresencePoint, error) {
	series := make([]analytics.PresencePoint, 0, len(rows))
	for _, row := range rows {
		if row.PeriodSelector != slice.Selector ||
			!validPresenceCell(slice.Selector, row.Status, row.Kind) {
			return nil, analytics.ErrPublicUnavailable
		}
		series = append(series, presencePoint(
			dateString(row.PeriodStart), row.Kind, row.Status,
			row.PublishedValue, row.PublishedLower, row.PublishedCentral, row.PublishedUpper,
		))
	}
	return series, nil
}

func (r *PublicAnalyticsRepository) Preferences(
	ctx context.Context,
	period string,
) (analytics.PublicPreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	rows, err := r.queries.ListCurrentPreferenceCells(ctx, period)
	if err != nil || len(rows) != 2 {
		return analytics.PublicPreferences{}, analytics.ErrPublicUnavailable
	}
	metadata, err := preferenceMetadata(rows)
	if err != nil {
		return analytics.PublicPreferences{}, err
	}
	categories, err := preferenceCategories(rows, period)
	if err != nil {
		return analytics.PublicPreferences{}, err
	}
	return analytics.PublicPreferences{
		Metadata: metadata,
		Period:   period,
		Metrics: []analytics.PublicPreferenceMetric{{
			MetricCode: "first_visit_share", DimensionCode: "visit_profile",
			Categories: categories,
		}},
	}, nil
}

func (r *PublicAnalyticsRepository) Methodology(
	ctx context.Context,
) (analytics.PublicMethodology, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	row, err := r.queries.GetCurrentMethodology(ctx)
	if err != nil || !validForecastBounds(row) {
		return analytics.PublicMethodology{}, analytics.ErrPublicUnavailable
	}
	metadata, err := publicMetadata(
		row.AsOfOn, nextDate(row.AsOfOn), "person_day", row.DataMode,
		row.PrivacyPolicyVersion, row.MethodologyVersion, row.CoverageStatus,
		row.CoverageRatioPercent, row.PublishedAt,
	)
	if err != nil {
		return analytics.PublicMethodology{}, err
	}
	return analytics.PublicMethodology{
		Metadata: metadata, PresenceInterval: row.PresenceInterval,
		TimeZone: row.TimeZone, ObservedDefinitionCode: row.ObservedDefinitionCode,
		ForecastDefinitionCode: row.ForecastDefinitionCode,
		ForecastBoundsPercent:  []int32{row.ForecastLowerPercent, row.ForecastUpperPercent},
		ForecastFallbackBoundsPercent: []int32{
			row.ForecastFallbackLowerPercent,
			row.ForecastFallbackUpperPercent,
		},
		PrimaryThreshold:               row.PrimaryThreshold,
		MinimumReportingAccommodations: row.MinimumReportingAccommodations,
		ComplementarySuppression:       row.ComplementarySuppression,
		RoundingBase:                   row.RoundingBase, RoundingMode: row.RoundingMode,
		PresenceHistoryDays:      row.PresenceHistoryDays,
		AllowedPresenceWindows:   append([]string(nil), row.AllowedPresenceWindows...),
		AllowedPreferencePeriods: append([]string(nil), row.AllowedPreferencePeriods...),
	}, nil
}

func preferenceCategories(
	rows []generated.PublicDataCurrentPreference,
	period string,
) ([]analytics.PreferenceCategory, error) {
	categories := make([]analytics.PreferenceCategory, 0, len(rows))
	for _, row := range rows {
		if row.PeriodSelector != period || row.DimensionCode != "visit_profile" {
			return nil, analytics.ErrPublicUnavailable
		}
		if !validPreference(row.CategoryCode, row.Status, row.SharePercent) {
			return nil, analytics.ErrPublicUnavailable
		}
		categories = append(categories, analytics.PreferenceCategory{
			CategoryCode: row.CategoryCode, Status: row.Status, SharePercent: row.SharePercent,
		})
	}
	return categories, nil
}

func validForecastBounds(row generated.GetCurrentMethodologyRow) bool {
	return row.ForecastLowerPercent == 85 &&
		row.ForecastUpperPercent == 115 &&
		row.ForecastFallbackLowerPercent == 70 &&
		row.ForecastFallbackUpperPercent == 130
}

func summaryPoints(
	rows []generated.PublicDataCurrentSummary,
) (analytics.PresencePoint, analytics.ForecastPeak, error) {
	asOf := dateString(rows[0].AsOfOn)
	today := todayPoint(rows, asOf)
	forecasts := forecastRows(rows)
	if today.Date == "" || len(forecasts) == 0 {
		return today, analytics.ForecastPeak{}, analytics.ErrPublicUnavailable
	}
	peak := forecastPeak(forecasts, rows[0].AsOfOn)
	return today, peak, nil
}

func forecastPeak(
	rows []generated.PublicDataCurrentSummary,
	asOf pgtype.Date,
) analytics.ForecastPeak {
	ordered := append([]generated.PublicDataCurrentSummary(nil), rows...)
	sort.SliceStable(ordered, func(first, second int) bool {
		return ordered[first].PeriodStart.Time.Before(ordered[second].PeriodStart.Time)
	})
	if status := invalidForecastHorizonStatus(ordered, asOf); status != "" {
		return analytics.ForecastPeak{Kind: "forecast", Status: status}
	}
	peak := ordered[0]
	for _, row := range ordered[1:] {
		if *row.PublishedCentral > *peak.PublishedCentral {
			peak = row
		}
	}
	return analytics.ForecastPeak{
		Date: dateString(peak.PeriodStart), Kind: "forecast", Status: peak.Status,
		Lower: peak.PublishedLower, Central: peak.PublishedCentral, Upper: peak.PublishedUpper,
	}
}

func invalidForecastHorizonStatus(
	rows []generated.PublicDataCurrentSummary,
	asOf pgtype.Date,
) string {
	if validForecastHorizon(rows, asOf) {
		return ""
	}
	for _, row := range rows {
		if row.Status == "protected" {
			return "protected"
		}
	}
	return "unavailable"
}

func validForecastHorizon(
	rows []generated.PublicDataCurrentSummary,
	asOf pgtype.Date,
) bool {
	if len(rows) != 30 || !asOf.Valid {
		return false
	}
	for index, row := range rows {
		expected := asOf.Time.AddDate(0, 0, index+1)
		if !validForecastRow(row, expected) {
			return false
		}
	}
	return true
}

func todayPoint(
	rows []generated.PublicDataCurrentSummary,
	asOf string,
) analytics.PresencePoint {
	for _, row := range rows {
		if row.Kind == "observed" && dateString(row.PeriodStart) == asOf {
			return presencePoint(
				asOf, row.Kind, row.Status, row.PublishedValue,
				row.PublishedLower, row.PublishedCentral, row.PublishedUpper,
			)
		}
	}
	return analytics.PresencePoint{}
}

func forecastRows(
	rows []generated.PublicDataCurrentSummary,
) []generated.PublicDataCurrentSummary {
	forecasts := make([]generated.PublicDataCurrentSummary, 0, 30)
	for _, row := range rows {
		if row.Kind == "forecast" &&
			row.PeriodSelector == analytics.PresenceForecastSelector {
			forecasts = append(forecasts, row)
		}
	}
	return forecasts
}

func forecastRowShape(row generated.PublicDataCurrentSummary) bool {
	return row.PeriodSelector == analytics.PresenceForecastSelector &&
		row.Kind == "forecast" &&
		row.Status == "published"
}

func validForecastRow(
	row generated.PublicDataCurrentSummary,
	expected time.Time,
) bool {
	if !forecastRowShape(row) || !row.PeriodStart.Valid {
		return false
	}
	return row.PeriodStart.Time.Equal(expected) && validForecastInterval(row)
}

func orderedInterval(lower, central, upper *int32) bool {
	return *lower >= 0 && *lower <= *central && *central <= *upper
}

func validForecastInterval(row generated.PublicDataCurrentSummary) bool {
	lower, central, upper := row.PublishedLower, row.PublishedCentral, row.PublishedUpper
	if lower == nil || central == nil || upper == nil {
		return false
	}
	return orderedInterval(lower, central, upper)
}

func summaryMetadata(
	rows []generated.PublicDataCurrentSummary,
) (analytics.PublicMetadata, error) {
	first := rows[0]
	start := first.AsOfOn
	end := nextDate(first.AsOfOn)
	for _, row := range rows {
		if row.Kind == "forecast" && row.PeriodEnd.Valid && row.PeriodEnd.Time.After(end.Time) {
			end = row.PeriodEnd
		}
	}
	return publicMetadata(
		start, end, first.Unit, first.DataMode, first.PrivacyPolicyVersion,
		first.MethodologyVersion, first.CoverageStatus, first.CoverageRatioPercent,
		first.PublishedAt,
	)
}

func presenceMetadata(
	rows []generated.PublicDataCurrentPresence,
) (analytics.PublicMetadata, error) {
	first, last := rows[0], rows[len(rows)-1]
	return publicMetadata(
		first.PeriodStart, last.PeriodEnd, first.Unit, first.DataMode,
		first.PrivacyPolicyVersion, first.MethodologyVersion, first.CoverageStatus,
		first.CoverageRatioPercent, first.PublishedAt,
	)
}

func preferenceMetadata(
	rows []generated.PublicDataCurrentPreference,
) (analytics.PublicMetadata, error) {
	first := rows[0]
	return publicMetadata(
		first.PeriodStart, first.PeriodEnd, first.Unit, first.DataMode,
		first.PrivacyPolicyVersion, first.MethodologyVersion, first.CoverageStatus,
		first.CoverageRatioPercent, first.PublishedAt,
	)
}

func publicMetadata(
	start, end pgtype.Date,
	unit, dataMode, policyVersion, methodologyVersion, coverageStatus string,
	coverageRatio *int32,
	publishedAt pgtype.Timestamptz,
) (analytics.PublicMetadata, error) {
	if !validMetadata(start, end, publishedAt, unit, dataMode, policyVersion, methodologyVersion) {
		return analytics.PublicMetadata{}, analytics.ErrPublicUnavailable
	}
	if !validCoverage(coverageStatus, coverageRatio) {
		return analytics.PublicMetadata{}, analytics.ErrPublicUnavailable
	}
	return analytics.PublicMetadata{
		Period: analytics.PublicPeriod{
			Start: dateString(start), End: dateString(end),
			EndExclusive: true, TimeZone: publicTimeZone,
		},
		Unit: unit, DataMode: dataMode, UpdatedAt: publishedAt.Time.UTC().Format(time.RFC3339),
		PrivacyPolicyVersion: policyVersion, MethodologyVersion: methodologyVersion,
		Coverage: analytics.PublicCoverage{Status: coverageStatus, Ratio: coverageRatio},
	}, nil
}

func validMetadataPeriod(start, end pgtype.Date, publishedAt pgtype.Timestamptz) bool {
	return start.Valid && end.Valid && end.Time.After(start.Time) && publishedAt.Valid
}

func validMetadataPolicy(unit, dataMode, policyVersion, methodologyVersion string) bool {
	validUnit := unit == "person_day" || unit == "survey_response"
	return validUnit && dataMode == "prototype_fixtures" &&
		policyVersion == "prototype-v1" &&
		methodologyVersion == "explainable-baseline-v1"
}

func validMetadata(
	start, end pgtype.Date,
	publishedAt pgtype.Timestamptz,
	unit, dataMode, policyVersion, methodologyVersion string,
) bool {
	return validMetadataPeriod(start, end, publishedAt) &&
		validMetadataPolicy(unit, dataMode, policyVersion, methodologyVersion)
}

// A published share is rounded to a multiple of five; a withheld one carries no
// value at all, never a substitute.
func publishedShare(value *int32) bool {
	return value != nil && *value >= 0 && *value <= 100 && *value%5 == 0
}

func withheldStatus(status string) bool {
	return status == "protected" || status == "unavailable"
}

func validCoverage(status string, ratio *int32) bool {
	if status == "published" {
		return publishedShare(ratio)
	}
	return withheldStatus(status) && ratio == nil
}

func validPublicCell(status, kind string) bool {
	if kind != "observed" && kind != "forecast" {
		return false
	}
	return status == "published" || withheldStatus(status)
}

func validPresenceCell(selector, status, kind string) bool {
	if selector == analytics.PresenceObservedSelector && kind != "observed" {
		return false
	}
	if selector == analytics.PresenceForecastSelector && kind != "forecast" {
		return false
	}
	return validPublicCell(status, kind)
}

func validPreference(category, status string, share *int32) bool {
	if category != "first_visit" && category != "returning" {
		return false
	}
	if status == "published" {
		return publishedShare(share)
	}
	return withheldStatus(status) && share == nil
}

func presencePoint(
	date, kind, status string,
	value, lower, central, upper *int32,
) analytics.PresencePoint {
	return analytics.PresencePoint{
		Date: date, Kind: kind, Status: status, Value: value,
		Lower: lower, Central: central, Upper: upper,
	}
}

func nextDate(value pgtype.Date) pgtype.Date {
	if !value.Valid {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: value.Time.AddDate(0, 0, 1), Valid: true}
}
