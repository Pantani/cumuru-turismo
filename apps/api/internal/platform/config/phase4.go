package config

import (
	"math"
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
	if !localOrTest(environment) {
		return Phase4Config{}, invalid("PHASE4_ENABLED")
	}
	if err := applyPhase4Settings(&config, process, lookup); err != nil {
		return Phase4Config{}, err
	}
	return config, nil
}

// The thresholds and intervals are read together so a single malformed field
// stops the load, naming itself, before any of them reaches a range check. Only
// the API talks to the public database, so only it reads that DSN.
func applyPhase4Settings(config *Phase4Config, process Process, lookup LookupEnv) error {
	if process == ProcessAPI {
		config.PublicDatabaseURL = required(lookup, "PUBLIC_DATABASE_URL")
	}
	reader := newEnvReader(lookup)
	config.PublicDatabaseTimeout = reader.duration(
		"PUBLIC_DATABASE_TIMEOUT",
		3*time.Second,
	)
	config.PrimaryCellThreshold = reader.integer(
		"PHASE4_PRIMARY_CELL_THRESHOLD",
		10,
	)
	config.MinimumReportingAccommodations = reader.integer(
		"PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS",
		3,
	)
	config.RoundingBase = reader.integer("PHASE4_ROUNDING_BASE", 10)
	config.PreRegisteredWeight = reader.decimal(
		"PHASE4_PRE_REGISTERED_WEIGHT",
		0.80,
	)
	config.IncrementalInterval = reader.duration(
		"PHASE4_INCREMENTAL_INTERVAL",
		15*time.Minute,
	)
	config.FullReconciliationInterval = reader.duration(
		"PHASE4_FULL_RECONCILIATION_INTERVAL",
		24*time.Hour,
	)
	return reader.Err()
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
	if !localOrTest(environment) {
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
	return firstError(
		c.validatePolicyVersions,
		c.validateThresholds,
		c.validateTimingAndWeight,
	)
}

func (c Phase4Config) validatePolicyVersions() error {
	if c.PrivacyPolicyVersion != phase4PolicyVersion {
		return invalid("PHASE4_PRIVACY_POLICY_VERSION")
	}
	if c.MethodologyVersion != phase4MethodologyVersion {
		return invalid("PHASE4_METHODOLOGY_VERSION")
	}
	if c.DataMode != phase4DataMode {
		return invalid("PHASE4_DATA_MODE")
	}
	return nil
}

// The suppression thresholds are frozen: weakening them would change what the
// public dashboard is allowed to reveal.
func (c Phase4Config) validateThresholds() error {
	if c.PrimaryCellThreshold != 10 {
		return invalid("PHASE4_PRIMARY_CELL_THRESHOLD")
	}
	if c.MinimumReportingAccommodations != 3 {
		return invalid("PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS")
	}
	if c.RoundingBase != 10 {
		return invalid("PHASE4_ROUNDING_BASE")
	}
	return nil
}

func (c Phase4Config) validateTimingAndWeight() error {
	if math.Abs(c.PreRegisteredWeight-0.80) > 0.000001 {
		return invalid("PHASE4_PRE_REGISTERED_WEIGHT")
	}
	return c.validateIntervals()
}

func (c Phase4Config) validateIntervals() error {
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
