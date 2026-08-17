package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Process string

const (
	ProcessAPI    Process = "api"
	ProcessWorker Process = "worker"
)

type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentTest       Environment = "test"
	EnvironmentStaging    Environment = "staging"
	EnvironmentProduction Environment = "production"
)

type OIDCMode string

const (
	OIDCModeFake OIDCMode = "fake"
	OIDCModeReal OIDCMode = "real"
)

type OIDCConfig struct {
	Mode        OIDCMode
	Issuer      string
	Audience    string
	HTTPTimeout time.Duration
}

type TelemetryConfig struct {
	Exporter string
	Endpoint string
	Timeout  time.Duration
}

type Config struct {
	Process           Process
	Environment       Environment
	HTTPAddress       string
	OperationsAddress string
	DatabaseURL       string
	DatabaseTimeout   time.Duration
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	LogLevel          string
	ServiceName       string
	TrustedProxyCIDRs []netip.Prefix
	OIDC              OIDCConfig
	Auth              AuthConfig
	Telemetry         TelemetryConfig
	Phase2            Phase2Config
	Phase3            Phase3Config
	Phase4            Phase4Config
	Phase7            Phase7Config
}

type LookupEnv func(string) (string, bool)

// resolveLookup keeps "a nil lookup reads the process environment" in one place
// so every loader shares the same default instead of restating it.
func resolveLookup(lookup LookupEnv) LookupEnv {
	if lookup == nil {
		return os.LookupEnv
	}
	return lookup
}

func knownProcess(process Process) bool {
	return process == ProcessAPI || process == ProcessWorker
}

func Load(process Process, lookup LookupEnv) (Config, error) {
	if !knownProcess(process) {
		return Config{}, errors.New("invalid process")
	}
	lookup = resolveLookup(lookup)
	cfg, err := loadConfig(process, lookup)
	if err != nil {
		return Config{}, err
	}
	return cfg, cfg.validate()
}

type loadedParts struct {
	environment       Environment
	trustedProxyCIDRs []netip.Prefix
	auth              AuthConfig
	phases            phaseConfigs
}

func loadParts(process Process, lookup LookupEnv) (loadedParts, error) {
	trustedProxyCIDRs, err := loadTrustedProxyCIDRs(process, lookup)
	if err != nil {
		return loadedParts{}, err
	}
	environment := Environment(required(lookup, "APP_ENV"))
	phases, err := loadPhases(environment, process, lookup)
	if err != nil {
		return loadedParts{}, err
	}
	auth, err := loadAuth(lookup)
	if err != nil {
		return loadedParts{}, err
	}
	return loadedParts{
		environment: environment, trustedProxyCIDRs: trustedProxyCIDRs,
		auth: auth, phases: phases,
	}, nil
}

func loadConfig(process Process, lookup LookupEnv) (Config, error) {
	parts, err := loadParts(process, lookup)
	if err != nil {
		return Config{}, err
	}
	reader := newEnvReader(lookup)
	cfg := baseConfig(
		process, parts.environment, reader,
		parts.trustedProxyCIDRs, parts.auth, parts.phases,
	)
	if err := reader.Err(); err != nil {
		return Config{}, err
	}
	applyProcessAddresses(&cfg, process, lookup)
	return cfg, nil
}

func baseConfig(
	process Process,
	environment Environment,
	reader *envReader,
	trustedProxyCIDRs []netip.Prefix,
	auth AuthConfig,
	phases phaseConfigs,
) Config {
	lookup := reader.lookup
	return Config{
		Process:           process,
		Environment:       environment,
		DatabaseURL:       required(lookup, "DATABASE_URL"),
		DatabaseTimeout:   reader.duration("DATABASE_TIMEOUT", 3*time.Second),
		ShutdownTimeout:   reader.duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: reader.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       reader.duration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:      reader.duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       reader.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		LogLevel:          optional(lookup, "LOG_LEVEL", "info"),
		TrustedProxyCIDRs: trustedProxyCIDRs,
		OIDC: OIDCConfig{
			Mode:        OIDCMode(required(lookup, "OIDC_MODE")),
			Issuer:      normalizeIssuer(required(lookup, "OIDC_ISSUER")),
			Audience:    required(lookup, "OIDC_AUDIENCE"),
			HTTPTimeout: reader.duration("OIDC_HTTP_TIMEOUT", 5*time.Second),
		},
		Telemetry: TelemetryConfig{
			Exporter: optional(lookup, "OTEL_EXPORTER", "none"),
			Endpoint: required(lookup, "OTEL_ENDPOINT"),
			Timeout:  reader.duration("OTEL_TIMEOUT", 5*time.Second),
		},
		Auth:   auth,
		Phase2: phases.phase2,
		Phase3: phases.phase3,
		Phase4: phases.phase4,
		Phase7: phases.phase7,
	}
}

// The API serves a public listener; the worker only exposes operations.
func applyProcessAddresses(cfg *Config, process Process, lookup LookupEnv) {
	if process == ProcessAPI {
		cfg.HTTPAddress = required(lookup, "HTTP_ADDRESS")
		cfg.OperationsAddress = required(lookup, "OPERATIONS_ADDRESS")
		cfg.ServiceName = "cumuru-api"
		return
	}
	cfg.OperationsAddress = optional(lookup, "WORKER_OPERATIONS_ADDRESS", "127.0.0.1:9091")
	cfg.ServiceName = "cumuru-worker"
}

type phaseConfigs struct {
	phase2 Phase2Config
	phase3 Phase3Config
	phase4 Phase4Config
	phase7 Phase7Config
}

func loadPhases(environment Environment, process Process, lookup LookupEnv) (phaseConfigs, error) {
	phase2, err := loadPhase2(lookup)
	if err != nil {
		return phaseConfigs{}, err
	}
	phase3, err := loadPhase3(environment, phase2, lookup)
	if err != nil {
		return phaseConfigs{}, err
	}
	phase4, err := loadPhase4(environment, process, lookup)
	if err != nil {
		return phaseConfigs{}, err
	}
	phase7, err := loadPhase7(environment, phase2, lookup)
	if err != nil {
		return phaseConfigs{}, err
	}
	return phaseConfigs{
		phase2: phase2, phase3: phase3, phase4: phase4, phase7: phase7,
	}, nil
}

func (c Config) validate() error {
	validators := []func() error{
		c.validateEnvironment,
		c.validateAddresses,
		c.validateDatabase,
		c.validateTimeouts,
		c.validateLogLevel,
		c.validateOIDC,
		c.validateTelemetry,
		func() error {
			return c.Phase2.validate(
				c.requiresHTTPS(),
				c.Environment,
				c.OIDC.Mode,
			)
		},
		c.Phase3.validate,
		c.Phase7.validate,
		func() error {
			return c.Phase4.validate(
				c.Process,
				c.Environment,
				c.DatabaseURL,
				c.requiresHTTPS(),
			)
		},
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) validateEnvironment() error {
	switch c.Environment {
	case EnvironmentLocal, EnvironmentTest, EnvironmentStaging, EnvironmentProduction:
		return nil
	default:
		return invalid("APP_ENV")
	}
}

func (c Config) validateAddresses() error {
	if c.Process == ProcessAPI {
		if err := validAddress("HTTP_ADDRESS", c.HTTPAddress); err != nil {
			return err
		}
	}
	if err := validAddress("OPERATIONS_ADDRESS", c.OperationsAddress); err != nil {
		return err
	}
	return nil
}

func (c Config) validateDatabase() error {
	return validDatabaseURL(c.DatabaseURL, c.requiresHTTPS())
}

func (c Config) validateTimeouts() error {
	timeouts := []struct {
		field string
		value time.Duration
	}{
		{"DATABASE_TIMEOUT", c.DatabaseTimeout},
		{"SHUTDOWN_TIMEOUT", c.ShutdownTimeout},
		{"HTTP_TIMEOUTS", c.ReadHeaderTimeout},
		{"HTTP_TIMEOUTS", c.ReadTimeout},
		{"HTTP_TIMEOUTS", c.WriteTimeout},
		{"HTTP_TIMEOUTS", c.IdleTimeout},
	}
	for _, timeout := range timeouts {
		if timeout.value <= 0 {
			return invalid(timeout.field)
		}
	}
	return nil
}

func (c Config) validateLogLevel() error {
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return invalid("LOG_LEVEL")
	}
}

// The fake verifier exists for local and test only; any other environment must
// point at a real issuer.
func (c Config) validateOIDCMode() error {
	switch c.OIDC.Mode {
	case OIDCModeFake:
		if !localOrTest(c.Environment) {
			return invalid("OIDC_MODE")
		}
	case OIDCModeReal:
	default:
		return invalid("OIDC_MODE")
	}
	return nil
}

func (c Config) validateOIDC() error {
	if err := c.validateOIDCMode(); err != nil {
		return err
	}
	if err := validURL("OIDC_ISSUER", c.OIDC.Issuer, c.requiresHTTPS()); err != nil {
		return err
	}
	if strings.TrimSpace(c.OIDC.Audience) == "" {
		return invalid("OIDC_AUDIENCE")
	}
	if c.OIDC.HTTPTimeout <= 0 {
		return invalid("OIDC_HTTP_TIMEOUT")
	}
	return nil
}

// Telemetry may be disabled outside the deployed environments; staging and
// production must export.
func (c Config) validateExporter() error {
	switch c.Telemetry.Exporter {
	case "none":
		if c.requiresHTTPS() {
			return invalid("OTEL_EXPORTER")
		}
		return nil
	case "otlp":
		return validURL("OTEL_ENDPOINT", c.Telemetry.Endpoint, c.requiresHTTPS())
	default:
		return invalid("OTEL_EXPORTER")
	}
}

func (c Config) validateTelemetry() error {
	if err := c.validateExporter(); err != nil {
		return err
	}
	if c.Telemetry.Timeout <= 0 {
		return invalid("OTEL_TIMEOUT")
	}
	return nil
}

func (c Config) requiresHTTPS() bool {
	return c.Environment == EnvironmentStaging || c.Environment == EnvironmentProduction
}

func validDatabaseURL(value string, requireTLS bool) error {
	return validDatabaseURLField("DATABASE_URL", value, requireTLS)
}

func postgresURL(parsed *url.URL) bool {
	scheme := parsed.Scheme == "postgres" || parsed.Scheme == "postgresql"
	return scheme && parsed.Host != ""
}

// A deployed database connection must authenticate the server hostname, so only
// verify-full is accepted there.
func verifyFullTLS(parsed *url.URL) bool {
	modes, ok := parsed.Query()["sslmode"]
	return ok && len(modes) == 1 && modes[0] == "verify-full"
}

func validDatabaseURLField(field, value string, requireTLS bool) error {
	parsed, err := url.Parse(value)
	if err != nil || !postgresURL(parsed) {
		return invalid(field)
	}
	if requireTLS && !verifyFullTLS(parsed) {
		return invalid(field)
	}
	return nil
}

func httpURL(parsed *url.URL) bool {
	scheme := parsed.Scheme == "http" || parsed.Scheme == "https"
	return scheme && parsed.Host != ""
}

func validURL(field, value string, requireHTTPS bool) error {
	parsed, err := url.Parse(value)
	if err != nil || !httpURL(parsed) {
		return invalid(field)
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return invalid(field)
	}
	return nil
}

func validPort(value string) bool {
	number, err := strconv.Atoi(value)
	return err == nil && number >= 1 && number <= 65535
}

func validAddress(field, value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" || !validPort(port) {
		return invalid(field)
	}
	return nil
}

func required(lookup LookupEnv, key string) string {
	value, _ := lookup(key)
	return strings.TrimSpace(value)
}

func optional(lookup LookupEnv, key, fallback string) string {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func normalizeIssuer(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), "/")
}

func invalid(field string) error {
	return fmt.Errorf("invalid configuration field %s", field)
}
