package config

import (
	"strings"
	"time"
)

// The user agent is institutional and fixed. It never carries tenant,
// organization, accommodation, operator, OIDC subject nor a version that
// varies per installation: cadence and per-request parameters are what leak in
// an external integration, not the server address (ADR-045 §6).
const externalContextUserAgent = "CumuruObservatorio/1.0 (+https://turismo.prado.ba.gov.br; contato@prado.ba.gov.br)"

// DefaultExternalAllowedHosts is the allowlist a deployment gets when it sets
// none. It is exported so that a test asserting how many ingestion targets the
// default enables reads the real default instead of a copy of it, which would
// keep passing against the old value after this one changed.
const DefaultExternalAllowedHosts = "api.open-meteo.com"

// ExternalContextConfig carries the first outbound HTTP surface of the product.
// The host allowlist lives here, in configuration, and never in the database:
// an UPDATE must not be able to turn the fetcher into an SSRF primitive.
type ExternalContextConfig struct {
	Enabled           bool
	DatabaseURL       string
	AllowedHosts      []string
	RequestTimeout    time.Duration
	BatchBudget       time.Duration
	MaxResponseBytes  int
	IngestionInterval time.Duration
	UserAgent         string
}

func loadExternalContext(
	environment Environment,
	process Process,
	lookup LookupEnv,
) (ExternalContextConfig, error) {
	enabled, err := parseBoolean(lookup, "EXTERNAL_CONTEXT_ENABLED", false)
	if err != nil {
		return ExternalContextConfig{}, err
	}
	config := externalContextDefaults(enabled)
	if !enabled {
		return config, nil
	}
	if !localOrTest(environment) {
		return ExternalContextConfig{}, invalid("EXTERNAL_CONTEXT_ENABLED")
	}
	if err := applyExternalContextSettings(&config, process, lookup); err != nil {
		return ExternalContextConfig{}, err
	}
	return config, nil
}

// Only the worker reads the ingestion DSN: egress happens in the worker and
// never on the request path, and the API reads the layer through the public
// pool, which sees only the view in public_data.
func applyExternalContextSettings(
	config *ExternalContextConfig,
	process Process,
	lookup LookupEnv,
) error {
	if process == ProcessWorker {
		config.DatabaseURL = required(lookup, "EXTERNAL_DATABASE_URL")
	}
	config.AllowedHosts = splitList(
		optional(lookup, "EXTERNAL_ALLOWED_HOSTS", DefaultExternalAllowedHosts),
	)
	reader := newEnvReader(lookup)
	config.RequestTimeout = reader.duration(
		"EXTERNAL_REQUEST_TIMEOUT",
		10*time.Second,
	)
	// DATABASE_TIMEOUT is a request budget and does not serve an ingestion
	// cycle; the batch gets its own ceiling.
	config.BatchBudget = reader.duration("EXTERNAL_BATCH_BUDGET", 2*time.Minute)
	config.MaxResponseBytes = reader.integer(
		"EXTERNAL_MAX_RESPONSE_BYTES",
		2*1024*1024,
	)
	config.IngestionInterval = reader.duration(
		"EXTERNAL_INGESTION_INTERVAL",
		6*time.Hour,
	)
	return reader.Err()
}

func externalContextDefaults(enabled bool) ExternalContextConfig {
	return ExternalContextConfig{
		Enabled:   enabled,
		UserAgent: externalContextUserAgent,
	}
}

func (c ExternalContextConfig) validate(
	process Process,
	environment Environment,
	databaseURL string,
	requireTLS bool,
) error {
	if !c.Enabled {
		return nil
	}
	if !localOrTest(environment) {
		return invalid("EXTERNAL_CONTEXT_ENABLED")
	}
	return firstError(
		c.validateBudgets,
		c.validateHosts,
		func() error {
			return c.validateDatabase(process, databaseURL, requireTLS)
		},
	)
}

func (c ExternalContextConfig) validateBudgets() error {
	return firstError(c.validateEgressTimeouts, c.validateEgressLimits)
}

func (c ExternalContextConfig) validateEgressTimeouts() error {
	if c.RequestTimeout <= 0 {
		return invalid("EXTERNAL_REQUEST_TIMEOUT")
	}
	if c.IngestionInterval <= 0 {
		return invalid("EXTERNAL_INGESTION_INTERVAL")
	}
	// A batch allowed to run longer than its own cycle would overlap itself.
	if c.BatchBudget <= 0 || c.BatchBudget >= c.IngestionInterval {
		return invalid("EXTERNAL_BATCH_BUDGET")
	}
	return nil
}

func (c ExternalContextConfig) validateEgressLimits() error {
	if c.MaxResponseBytes <= 0 {
		return invalid("EXTERNAL_MAX_RESPONSE_BYTES")
	}
	if strings.TrimSpace(c.UserAgent) != externalContextUserAgent {
		return invalid("EXTERNAL_USER_AGENT")
	}
	return nil
}

func (c ExternalContextConfig) validateHosts() error {
	if len(c.AllowedHosts) == 0 {
		return invalid("EXTERNAL_ALLOWED_HOSTS")
	}
	for _, host := range c.AllowedHosts {
		if !allowlistHost(host) {
			return invalid("EXTERNAL_ALLOWED_HOSTS")
		}
	}
	return nil
}

// An allowlist entry is a bare host: a scheme, path, query or port would never
// match the host of the constant URL the fetcher builds, and accepting one
// would hide a mismatch instead of failing on it.
func allowlistHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if strings.ContainsAny(host, "/?#@: ") {
		return false
	}
	return dottedHost(host)
}

func dottedHost(host string) bool {
	return strings.Contains(host, ".") &&
		!strings.HasPrefix(host, ".") &&
		!strings.HasSuffix(host, ".")
}

func (c ExternalContextConfig) validateDatabase(
	process Process,
	databaseURL string,
	requireTLS bool,
) error {
	if process != ProcessWorker {
		return nil
	}
	if err := validDatabaseURLField(
		"EXTERNAL_DATABASE_URL",
		c.DatabaseURL,
		requireTLS,
	); err != nil {
		return err
	}
	// The ingestion role must be distinct from the application role, otherwise
	// the grant model in migration 000005 constrains nothing.
	if duplicateDatabaseRoles(databaseURL, c.DatabaseURL) {
		return invalid("EXTERNAL_DATABASE_URL")
	}
	return nil
}
