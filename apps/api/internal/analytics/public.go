package analytics

import (
	"context"
	"errors"
)

var ErrPublicUnavailable = errors.New("public analytics unavailable")

type PublicReader interface {
	Summary(context.Context) (PublicSummary, error)
	Presence(context.Context, PresenceSlice) (PublicPresence, error)
	Preferences(context.Context, string) (PublicPreferences, error)
	Methodology(context.Context) (PublicMethodology, error)
}

type QualityReader interface {
	Quality(context.Context, string) (QualitySnapshot, error)
}

type PublicPeriod struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	EndExclusive bool   `json:"end_exclusive"`
	TimeZone     string `json:"time_zone"`
}

type PublicCoverage struct {
	Status string `json:"status"`
	Ratio  *int32 `json:"ratio,omitempty"`
}

type PublicMetadata struct {
	Period               PublicPeriod   `json:"period"`
	Unit                 string         `json:"unit"`
	DataMode             string         `json:"data_mode"`
	UpdatedAt            string         `json:"updated_at"`
	PrivacyPolicyVersion string         `json:"privacy_policy_version"`
	MethodologyVersion   string         `json:"methodology_version"`
	Coverage             PublicCoverage `json:"coverage"`
}

type PresencePoint struct {
	Date    string `json:"date"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Value   *int32 `json:"value,omitempty"`
	Lower   *int32 `json:"lower,omitempty"`
	Central *int32 `json:"central,omitempty"`
	Upper   *int32 `json:"upper,omitempty"`
}

type ForecastPeak struct {
	Date    string `json:"date,omitempty"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Lower   *int32 `json:"lower,omitempty"`
	Central *int32 `json:"central,omitempty"`
	Upper   *int32 `json:"upper,omitempty"`
}

type PublicPresence struct {
	Metadata PublicMetadata  `json:"metadata"`
	Window   string          `json:"window"`
	Month    string          `json:"month,omitempty"`
	Series   []PresencePoint `json:"series"`
}

type PublicSummary struct {
	Metadata                   PublicMetadata `json:"metadata"`
	PresenceToday              PresencePoint  `json:"presence_today"`
	ForecastPeakNextThirtyDays ForecastPeak   `json:"forecast_peak_next_30_days"`
}

type PreferenceCategory struct {
	CategoryCode string `json:"category_code"`
	Status       string `json:"status"`
	SharePercent *int32 `json:"share_percent,omitempty"`
}

type PublicPreferenceMetric struct {
	MetricCode    string               `json:"metric_code"`
	DimensionCode string               `json:"dimension_code"`
	Categories    []PreferenceCategory `json:"categories"`
}

type PublicPreferences struct {
	Metadata PublicMetadata           `json:"metadata"`
	Period   string                   `json:"period"`
	Metrics  []PublicPreferenceMetric `json:"metrics"`
}

type PublicMethodology struct {
	Metadata                       PublicMetadata `json:"metadata"`
	PresenceInterval               string         `json:"presence_interval"`
	TimeZone                       string         `json:"time_zone"`
	ObservedDefinitionCode         string         `json:"observed_definition_code"`
	ForecastDefinitionCode         string         `json:"forecast_definition_code"`
	ForecastBoundsPercent          []int32        `json:"forecast_bounds_percent"`
	ForecastFallbackBoundsPercent  []int32        `json:"forecast_fallback_bounds_percent"`
	PrimaryThreshold               int32          `json:"primary_threshold"`
	MinimumReportingAccommodations int32          `json:"minimum_reporting_accommodations"`
	ComplementarySuppression       bool           `json:"complementary_suppression"`
	RoundingBase                   int32          `json:"rounding_base"`
	RoundingMode                   string         `json:"rounding_mode"`
	PresenceHistoryDays            int32          `json:"presence_history_days"`
	AllowedPresenceWindows         []string       `json:"allowed_presence_windows"`
	AllowedPreferencePeriods       []string       `json:"allowed_preference_periods"`
}

type QualityCount struct {
	Status     string `json:"status"`
	Value      *int32 `json:"value,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
}

type QualityCoverage struct {
	CategoryCode string   `json:"category_code"`
	Status       string   `json:"status"`
	Ratio        *float64 `json:"ratio,omitempty"`
}

type QualitySnapshot struct {
	Window                   string            `json:"window"`
	UpdatedAt                string            `json:"updated_at"`
	IncompleteStays          QualityCount      `json:"incomplete_stays"`
	OverduePlannedDepartures QualityCount      `json:"overdue_planned_departures"`
	SilentAccommodations     QualityCount      `json:"silent_accommodations"`
	AggregationFailures      QualityCount      `json:"aggregation_failures"`
	SuspectedDuplicates      QualityCount      `json:"suspected_duplicates"`
	FNRHFailures             QualityCount      `json:"fnrh_failures"`
	CoverageByCategory       []QualityCoverage `json:"coverage_by_category"`
}
