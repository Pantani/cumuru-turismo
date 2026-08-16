package config

import (
	"strconv"
	"strings"
	"time"
)

const (
	maximumSurveyTTL   = 24 * time.Hour
	aesGCM256KeyLength = 32
)

type Phase3Config struct {
	Enabled                bool
	PrimaryStableKey       string
	SurveyTTL              time.Duration
	FreeTextTTL            time.Duration
	SurveySubmitRateLimit  int
	FreeTextCleanupEnabled bool
	SurveyKeys             KeyringConfig
	FreeTextKeys           KeyringConfig
}

func loadPhase3(
	environment Environment,
	phase2 Phase2Config,
	lookup LookupEnv,
) (Phase3Config, error) {
	enabled, err := parseBoolean(lookup, "PHASE3_ENABLED", false)
	if err != nil {
		return Phase3Config{}, err
	}
	config := Phase3Config{
		Enabled:          enabled,
		PrimaryStableKey: "tourism_profile",
	}
	if !enabled {
		return config, nil
	}
	if !localOrTest(environment) {
		return Phase3Config{}, invalid("PHASE3_ENABLED")
	}

	if err := applyPhase3Settings(&config, phase2, lookup); err != nil {
		return Phase3Config{}, err
	}
	return config, nil
}

// A survey keyring must never share a key with a phase 2 keyring: one leaked
// key would otherwise span two capability domains.
func (k phase3Keyrings) distinctFrom(phase2 Phase2Config) error {
	existing := phase2Keyrings(phase2)
	if keyringsOverlap(append(existing, k.survey, k.freeText)) {
		return invalid("PHASE3_KEYRINGS")
	}
	return nil
}

func applyPhase3Settings(
	config *Phase3Config,
	phase2 Phase2Config,
	lookup LookupEnv,
) error {
	keys, err := loadPhase3Keyrings(lookup)
	if err != nil {
		return err
	}
	config.SurveyTTL = duration(lookup, "SURVEY_CAPABILITY_TTL", maximumSurveyTTL)
	config.FreeTextTTL = duration(lookup, "SURVEY_FREE_TEXT_TTL", maximumSurveyTTL)
	config.SurveySubmitRateLimit = positiveInteger(
		lookup,
		"SURVEY_SUBMIT_RATE_LIMIT",
		10,
	)
	config.FreeTextCleanupEnabled, err = parseBoolean(
		lookup,
		"SURVEY_FREE_TEXT_CLEANUP_ENABLED",
		false,
	)
	if err != nil {
		return err
	}
	config.SurveyKeys = keys.survey
	config.FreeTextKeys = keys.freeText
	return keys.distinctFrom(phase2)
}

type phase3Keyrings struct {
	survey   KeyringConfig
	freeText KeyringConfig
}

func loadPhase3Keyrings(lookup LookupEnv) (phase3Keyrings, error) {
	surveyKeys, err := parseKeyring(
		"SURVEY_HMAC_CURRENT_VERSION",
		required(lookup, "SURVEY_HMAC_CURRENT_VERSION"),
		"SURVEY_HMAC_KEYS",
		required(lookup, "SURVEY_HMAC_KEYS"),
	)
	if err != nil {
		return phase3Keyrings{}, err
	}
	freeTextKeys, err := parseKeyring(
		"SURVEY_FREE_TEXT_CURRENT_VERSION",
		required(lookup, "SURVEY_FREE_TEXT_CURRENT_VERSION"),
		"SURVEY_FREE_TEXT_KEYS",
		required(lookup, "SURVEY_FREE_TEXT_KEYS"),
	)
	if err != nil {
		return phase3Keyrings{}, err
	}
	return phase3Keyrings{survey: surveyKeys, freeText: freeTextKeys}, nil
}

func (c Phase3Config) validate() error {
	if !c.Enabled {
		return nil
	}
	if c.PrimaryStableKey != "tourism_profile" {
		return invalid("PHASE3_PRIMARY_STABLE_KEY")
	}
	if err := c.validateDurations(); err != nil {
		return err
	}
	return c.validateSurveyPolicy()
}

// Free text is only accepted when its erase pipeline is enabled, so plaintext
// always has a deletion path.
func (c Phase3Config) validateSurveyPolicy() error {
	if c.SurveySubmitRateLimit <= 0 {
		return invalid("SURVEY_SUBMIT_RATE_LIMIT")
	}
	if !c.FreeTextCleanupEnabled {
		return invalid("SURVEY_FREE_TEXT_CLEANUP_ENABLED")
	}
	return c.validateFreeTextKeys()
}

func (c Phase3Config) validateDurations() error {
	if c.SurveyTTL <= 0 || c.SurveyTTL > maximumSurveyTTL {
		return invalid("SURVEY_CAPABILITY_TTL")
	}
	if c.FreeTextTTL <= 0 || c.FreeTextTTL > maximumSurveyTTL {
		return invalid("SURVEY_FREE_TEXT_TTL")
	}
	return nil
}

func (c Phase3Config) validateFreeTextKeys() error {
	for _, key := range c.FreeTextKeys.Keys {
		if len(key) != aesGCM256KeyLength {
			return invalid("SURVEY_FREE_TEXT_KEYS")
		}
	}
	return nil
}

func phase2Keyrings(config Phase2Config) []KeyringConfig {
	return []KeyringConfig{
		config.InviteKeys,
		config.ActorKeys,
		config.IdempotencyKeys,
		config.RateLimitKeys,
		config.CursorKeys,
	}
}

func parseBoolean(lookup LookupEnv, field string, fallback bool) (bool, error) {
	value, ok := lookup(field)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, invalid(field)
	}
	return parsed, nil
}
