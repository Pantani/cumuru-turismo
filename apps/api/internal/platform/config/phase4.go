package config

import (
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	phase4PolicyVersion      = "prototype-v1"
	phase4MethodologyVersion = "explainable-baseline-v1"
	phase4DataMode           = "prototype_fixtures"
)

type Phase4Config struct {
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

func loadPhase4(
	environment Environment,
	process Process,
	lookup LookupEnv,
) (Phase4Config, error) {
	enabled, err := parseBoolean(lookup, "PHASE4_ENABLED", false)
	if err != nil {
		return Phase4Config{}, err
	}
	config := phase4Defaults(enabled)
	if !enabled {
		return config, nil
	}
	if environment != EnvironmentLocal && environment != EnvironmentTest {
		return Phase4Config{}, invalid("PHASE4_ENABLED")
	}
	if process == ProcessAPI {
		config.PublicDatabaseURL = required(lookup, "PUBLIC_DATABASE_URL")
	}
	config.PublicDatabaseTimeout = duration(
		lookup,
		"PUBLIC_DATABASE_TIMEOUT",
		3*time.Second,
	)
	config.PrimaryCellThreshold = positiveInteger(
		lookup,
		"PHASE4_PRIMARY_CELL_THRESHOLD",
		10,
	)
	config.MinimumReportingAccommodations = positiveInteger(
		lookup,
		"PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS",
		3,
	)
	config.RoundingBase = positiveInteger(
		lookup,
		"PHASE4_ROUNDING_BASE",
		10,
	)
	config.PreRegisteredWeight = decimal(
		lookup,
		"PHASE4_PRE_REGISTERED_WEIGHT",
		0.80,
	)
	config.IncrementalInterval = duration(
		lookup,
		"PHASE4_INCREMENTAL_INTERVAL",
		15*time.Minute,
	)
	config.FullReconciliationInterval = duration(
		lookup,
		"PHASE4_FULL_RECONCILIATION_INTERVAL",
		24*time.Hour,
	)
	return config, nil
}

func phase4Defaults(enabled bool) Phase4Config {
	return Phase4Config{
		Enabled:                        enabled,
		PrivacyPolicyVersion:           phase4PolicyVersion,
		MethodologyVersion:             phase4MethodologyVersion,
		DataMode:                       phase4DataMode,
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
		RoundingBase:                   10,
		PreRegisteredWeight:            0.80,
	}
}

func (c Phase4Config) validate(
	process Process,
	environment Environment,
	databaseURL string,
	requireTLS bool,
) error {
	if !c.Enabled {
		return nil
	}
	if environment != EnvironmentLocal && environment != EnvironmentTest {
		return invalid("PHASE4_ENABLED")
	}
	if err := c.validatePolicy(); err != nil {
		return err
	}
	if process == ProcessAPI {
		return c.validatePublicDatabase(databaseURL, requireTLS)
	}
	return nil
}

func (c Phase4Config) validatePolicy() error {
	if c.PrivacyPolicyVersion != phase4PolicyVersion {
		return invalid("PHASE4_PRIVACY_POLICY_VERSION")
	}
	if c.MethodologyVersion != phase4MethodologyVersion {
		return invalid("PHASE4_METHODOLOGY_VERSION")
	}
	if c.DataMode != phase4DataMode {
		return invalid("PHASE4_DATA_MODE")
	}
	if c.PrimaryCellThreshold != 10 {
		return invalid("PHASE4_PRIMARY_CELL_THRESHOLD")
	}
	if c.MinimumReportingAccommodations != 3 {
		return invalid("PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS")
	}
	if c.RoundingBase != 10 {
		return invalid("PHASE4_ROUNDING_BASE")
	}
	return c.validateTimingAndWeight()
}

func (c Phase4Config) validateTimingAndWeight() error {
	if math.Abs(c.PreRegisteredWeight-0.80) > 0.000001 {
		return invalid("PHASE4_PRE_REGISTERED_WEIGHT")
	}
	if c.IncrementalInterval <= 0 || c.IncrementalInterval > 15*time.Minute {
		return invalid("PHASE4_INCREMENTAL_INTERVAL")
	}
	if c.FullReconciliationInterval != 24*time.Hour {
		return invalid("PHASE4_FULL_RECONCILIATION_INTERVAL")
	}
	if c.PublicDatabaseTimeout <= 0 {
		return invalid("PUBLIC_DATABASE_TIMEOUT")
	}
	return nil
}

func (c Phase4Config) validatePublicDatabase(
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

func decimal(lookup LookupEnv, field string, fallback float64) float64 {
	value, ok := lookup(field)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return math.NaN()
	}
	return parsed
}
