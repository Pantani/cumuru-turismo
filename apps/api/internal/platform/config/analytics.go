package config

import (
	"math"
	"strings"
	"time"
)

const (
	analyticsPolicyVersion      = "prototype-v1"
	analyticsMethodologyVersion = "explainable-baseline-v1"
	analyticsDataMode           = "prototype_fixtures"
)

type AnalyticsConfig struct {
	Enabled                        bool
	PublicDatabaseURL              string
	PublicDatabaseTimeout          time.Duration
	PrivacyPolicyVersion           string
	MethodologyVersion             string
	DataMode                       string
	PrimaryCellThreshold           int
	MinimumReportingAccommodations int
	RoundingBase                   int
	PreRegisteredWeight            float64
	IncrementalInterval            time.Duration
	FullReconciliationInterval     time.Duration
}

func loadAnalytics(
	environment Environment,
	process Process,
	lookup LookupEnv,
) (AnalyticsConfig, error) {
	enabled, err := parseBoolean(lookup, "ANALYTICS_ENABLED", false)
	if err != nil {
		return AnalyticsConfig{}, err
	}
	config := analyticsDefaults(enabled)
	if !enabled {
		return config, nil
	}
	if !localOrTest(environment) {
		return AnalyticsConfig{}, invalid("ANALYTICS_ENABLED")
	}
	if err := applyAnalyticsSettings(&config, process, lookup); err != nil {
		return AnalyticsConfig{}, err
	}
	return config, nil
}

// The thresholds and intervals are read together so a single malformed field
// stops the load, naming itself, before any of them reaches a range check. Only
// the API talks to the public database, so only it reads that DSN.
func applyAnalyticsSettings(config *AnalyticsConfig, process Process, lookup LookupEnv) error {
	if process == ProcessAPI {
		config.PublicDatabaseURL = required(lookup, "PUBLIC_DATABASE_URL")
	}
	reader := newEnvReader(lookup)
	config.PublicDatabaseTimeout = reader.duration(
		"PUBLIC_DATABASE_TIMEOUT",
		3*time.Second,
	)
	config.PrimaryCellThreshold = reader.integer(
		"ANALYTICS_PRIMARY_CELL_THRESHOLD",
		10,
	)
	config.MinimumReportingAccommodations = reader.integer(
		"ANALYTICS_MINIMUM_REPORTING_ACCOMMODATIONS",
		3,
	)
	config.RoundingBase = reader.integer("ANALYTICS_ROUNDING_BASE", 10)
	config.PreRegisteredWeight = reader.decimal(
		"ANALYTICS_PRE_REGISTERED_WEIGHT",
		0.80,
	)
	config.IncrementalInterval = reader.duration(
		"ANALYTICS_INCREMENTAL_INTERVAL",
		15*time.Minute,
	)
	config.FullReconciliationInterval = reader.duration(
		"ANALYTICS_FULL_RECONCILIATION_INTERVAL",
		24*time.Hour,
	)
	return reader.Err()
}

func analyticsDefaults(enabled bool) AnalyticsConfig {
	return AnalyticsConfig{
		Enabled:                        enabled,
		PrivacyPolicyVersion:           analyticsPolicyVersion,
		MethodologyVersion:             analyticsMethodologyVersion,
		DataMode:                       analyticsDataMode,
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
		RoundingBase:                   10,
		PreRegisteredWeight:            0.80,
	}
}

func (c AnalyticsConfig) validate(
	process Process,
	environment Environment,
	databaseURL string,
	requireTLS bool,
) error {
	if !c.Enabled {
		return nil
	}
	if !localOrTest(environment) {
		return invalid("ANALYTICS_ENABLED")
	}
	if err := c.validatePolicy(); err != nil {
		return err
	}
	if process == ProcessAPI {
		return c.validatePublicDatabase(databaseURL, requireTLS)
	}
	return nil
}

func (c AnalyticsConfig) validatePolicy() error {
	return firstError(
		c.validatePolicyVersions,
		c.validateThresholds,
		c.validateTimingAndWeight,
	)
}

func (c AnalyticsConfig) validatePolicyVersions() error {
	if c.PrivacyPolicyVersion != analyticsPolicyVersion {
		return invalid("ANALYTICS_PRIVACY_POLICY_VERSION")
	}
	if c.MethodologyVersion != analyticsMethodologyVersion {
		return invalid("ANALYTICS_METHODOLOGY_VERSION")
	}
	if c.DataMode != analyticsDataMode {
		return invalid("ANALYTICS_DATA_MODE")
	}
	return nil
}

// The suppression thresholds are frozen: weakening them would change what the
// public dashboard is allowed to reveal.
func (c AnalyticsConfig) validateThresholds() error {
	if c.PrimaryCellThreshold != 10 {
		return invalid("ANALYTICS_PRIMARY_CELL_THRESHOLD")
	}
	if c.MinimumReportingAccommodations != 3 {
		return invalid("ANALYTICS_MINIMUM_REPORTING_ACCOMMODATIONS")
	}
	if c.RoundingBase != 10 {
		return invalid("ANALYTICS_ROUNDING_BASE")
	}
	return nil
}

func (c AnalyticsConfig) validateTimingAndWeight() error {
	if math.Abs(c.PreRegisteredWeight-0.80) > 0.000001 {
		return invalid("ANALYTICS_PRE_REGISTERED_WEIGHT")
	}
	return c.validateIntervals()
}

func (c AnalyticsConfig) validateIntervals() error {
	if c.IncrementalInterval <= 0 || c.IncrementalInterval > 15*time.Minute {
		return invalid("ANALYTICS_INCREMENTAL_INTERVAL")
	}
	if c.FullReconciliationInterval != 24*time.Hour {
		return invalid("ANALYTICS_FULL_RECONCILIATION_INTERVAL")
	}
	if c.PublicDatabaseTimeout <= 0 {
		return invalid("PUBLIC_DATABASE_TIMEOUT")
	}
	return nil
}

func (c AnalyticsConfig) validatePublicDatabase(
	databaseURL string,
	requireTLS bool,
) error {
	if err := validDatabaseURLField(
		"PUBLIC_DATABASE_URL",
		c.PublicDatabaseURL,
		requireTLS,
	); err != nil {
		return err
	}
	if strings.TrimSpace(c.PublicDatabaseURL) == strings.TrimSpace(databaseURL) {
		return invalid("PUBLIC_DATABASE_URL")
	}
	return nil
}
