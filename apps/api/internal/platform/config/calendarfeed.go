package config

import "time"

// CalendarFeedConfig carries the import of the lodging platform's calendar.
//
// It is off unless declared, and off means the routes are never registered:
// the feature makes outbound requests to hosts named by users and stores a
// bearer URL, so a half-configured deployment must have no surface at all
// rather than a surface that fails at the first request (ADR-044).
type CalendarFeedConfig struct {
	Enabled bool
	// FetchTimeout bounds one outbound request. The worker's cycle budget is
	// built from it, so a host that accepts the connection and then goes quiet
	// costs one feed's worth of time and not the whole cycle.
	FetchTimeout time.Duration
	FetchLimit   int64
	BatchSize    int32
	// URLKeys seals the address and FingerprintKeys blinds it for uniqueness.
	// They are two rings and not one because they answer different questions:
	// one has to be reversible and the other must never be.
	URLKeys         KeyringConfig
	FingerprintKeys KeyringConfig
}

const (
	maximumCalendarFetchTimeout = 30 * time.Second
	maximumCalendarBatchSize    = 200
)

func loadCalendarFeed(
	environment Environment,
	core CoreConfig,
	lookup LookupEnv,
) (CalendarFeedConfig, error) {
	enabled, err := parseBoolean(lookup, "CALENDAR_FEED_ENABLED", false)
	if err != nil {
		return CalendarFeedConfig{}, err
	}
	if !enabled {
		return CalendarFeedConfig{}, nil
	}
	// The runtime is PROTOTYPE_ONLY and the KMS gate is open: the sealing key
	// lives in an environment variable, so a real deployment must not be able to
	// turn this on by setting one flag.
	if !localOrTest(environment) {
		return CalendarFeedConfig{}, invalid("CALENDAR_FEED_ENABLED")
	}
	return calendarFeedSettings(core, lookup)
}

func calendarFeedSettings(
	core CoreConfig,
	lookup LookupEnv,
) (CalendarFeedConfig, error) {
	reader := newEnvReader(lookup)
	config := CalendarFeedConfig{
		Enabled:      true,
		FetchTimeout: reader.duration("CALENDAR_FEED_FETCH_TIMEOUT", 10*time.Second),
		FetchLimit:   int64(reader.integer("CALENDAR_FEED_FETCH_LIMIT", 4<<20)),
		BatchSize:    int32(reader.integer("CALENDAR_FEED_BATCH_SIZE", 25)),
	}
	if err := reader.Err(); err != nil {
		return CalendarFeedConfig{}, err
	}
	if err := applyCalendarFeedKeyrings(&config, core, lookup); err != nil {
		return CalendarFeedConfig{}, err
	}
	return config, config.validate()
}

// Neither ring may share a key with any other. A leaked sealing key would
// otherwise reverse an idempotency digest or a blinded document, which are
// unrelated domains that happen to live in the same process.
func applyCalendarFeedKeyrings(
	config *CalendarFeedConfig,
	core CoreConfig,
	lookup LookupEnv,
) error {
	sealing, err := parseKeyring(
		"CALENDAR_FEED_URL_CURRENT_VERSION",
		required(lookup, "CALENDAR_FEED_URL_CURRENT_VERSION"),
		"CALENDAR_FEED_URL_KEYS",
		required(lookup, "CALENDAR_FEED_URL_KEYS"),
	)
	if err != nil {
		return err
	}
	fingerprint, err := parseKeyring(
		"CALENDAR_FEED_FINGERPRINT_CURRENT_VERSION",
		required(lookup, "CALENDAR_FEED_FINGERPRINT_CURRENT_VERSION"),
		"CALENDAR_FEED_FINGERPRINT_KEYS",
		required(lookup, "CALENDAR_FEED_FINGERPRINT_KEYS"),
	)
	if err != nil {
		return err
	}
	if keyringsOverlap(append(core.keyrings(), sealing, fingerprint)) {
		return invalid("CALENDAR_FEED_URL_KEYS")
	}
	config.URLKeys = sealing
	config.FingerprintKeys = fingerprint
	return nil
}

func (c CalendarFeedConfig) validate() error {
	if !c.Enabled {
		return nil
	}
	validators := []func() error{
		c.validateFetch,
		c.validateBatch,
		c.validateKeyring,
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c CalendarFeedConfig) validateFetch() error {
	if c.FetchTimeout <= 0 || c.FetchTimeout > maximumCalendarFetchTimeout {
		return invalid("CALENDAR_FEED_FETCH_TIMEOUT")
	}
	if c.FetchLimit <= 0 {
		return invalid("CALENDAR_FEED_FETCH_LIMIT")
	}
	return nil
}

func (c CalendarFeedConfig) validateBatch() error {
	if c.BatchSize <= 0 || c.BatchSize > maximumCalendarBatchSize {
		return invalid("CALENDAR_FEED_BATCH_SIZE")
	}
	return nil
}

// AES-GCM demands exactly 32 bytes, while the HMAC rings accept at least that:
// the sealing ring is checked here so a short key fails at startup instead of at
// the first feed somebody registers.
func (c CalendarFeedConfig) validateKeyring() error {
	if !exactKeyLength(c.URLKeys, aesKeyBytes) {
		return invalid("CALENDAR_FEED_URL_KEYS")
	}
	return nil
}

const aesKeyBytes = 32

func exactKeyLength(keyring KeyringConfig, length int) bool {
	for _, key := range keyring.Keys {
		if len(key) != length {
			return false
		}
	}
	return true
}
