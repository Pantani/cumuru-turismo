package config

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	minimumHMACKeyBytes = 32
	minimumReplayTTL    = 30 * 24 * time.Hour
)

var keyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type KeyringConfig struct {
	CurrentVersion string
	Keys           map[string][]byte
}

func (k KeyringConfig) Key(version string) ([]byte, bool) {
	key, ok := k.Keys[version]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), key...), true
}

type Phase2Config struct {
	InviteBaseURL                  *url.URL
	InviteTTL                      time.Duration
	IdempotencyTTL                 time.Duration
	RateLimitWindow                time.Duration
	InviteContextRateLimit         int
	InviteSubmitRateLimit          int
	AccommodationOnboardingEnabled bool
	CORSAllowedOrigins             []string
	InviteKeys                     KeyringConfig
	ActorKeys                      KeyringConfig
	IdempotencyKeys                KeyringConfig
	RateLimitKeys                  KeyringConfig
	CursorKeys                     KeyringConfig
}

func loadPhase2(lookup LookupEnv) (Phase2Config, error) {
	onboardingEnabled, err := parseBoolean(
		lookup,
		"ACCOMMODATION_ONBOARDING_ENABLED",
		false,
	)
	if err != nil {
		return Phase2Config{}, err
	}
	baseURL, err := parseAbsoluteURL("INVITE_BASE_URL", required(lookup, "INVITE_BASE_URL"))
	if err != nil {
		return Phase2Config{}, err
	}
	keyrings, err := loadKeyrings(lookup)
	if err != nil {
		return Phase2Config{}, err
	}
	return Phase2Config{
		InviteBaseURL:                  baseURL,
		InviteTTL:                      duration(lookup, "INVITE_TTL", 72*time.Hour),
		IdempotencyTTL:                 duration(lookup, "IDEMPOTENCY_TTL", minimumReplayTTL),
		RateLimitWindow:                duration(lookup, "RATE_LIMIT_WINDOW", time.Minute),
		InviteContextRateLimit:         positiveInteger(lookup, "INVITE_CONTEXT_RATE_LIMIT", 30),
		InviteSubmitRateLimit:          positiveInteger(lookup, "INVITE_SUBMIT_RATE_LIMIT", 10),
		AccommodationOnboardingEnabled: onboardingEnabled,
		CORSAllowedOrigins:             splitList(required(lookup, "CORS_ALLOWED_ORIGINS")),
		InviteKeys:                     keyrings[0],
		ActorKeys:                      keyrings[1],
		IdempotencyKeys:                keyrings[2],
		RateLimitKeys:                  keyrings[3],
		CursorKeys:                     keyrings[4],
	}, nil
}

func loadKeyrings(lookup LookupEnv) ([5]KeyringConfig, error) {
	specs := [5]struct {
		versionField string
		keysField    string
	}{
		{"INVITE_HMAC_CURRENT_VERSION", "INVITE_HMAC_KEYS"},
		{"ACTOR_HMAC_CURRENT_VERSION", "ACTOR_HMAC_KEYS"},
		{"IDEMPOTENCY_HMAC_CURRENT_VERSION", "IDEMPOTENCY_HMAC_KEYS"},
		{"RATE_LIMIT_HMAC_CURRENT_VERSION", "RATE_LIMIT_HMAC_KEYS"},
		{"CURSOR_HMAC_CURRENT_VERSION", "CURSOR_HMAC_KEYS"},
	}
	var result [5]KeyringConfig
	for index, spec := range specs {
		keyring, err := parseKeyring(
			spec.versionField,
			required(lookup, spec.versionField),
			spec.keysField,
			required(lookup, spec.keysField),
		)
		if err != nil {
			return [5]KeyringConfig{}, err
		}
		result[index] = keyring
	}
	if keyringsOverlap(result[:]) {
		return [5]KeyringConfig{}, invalid("HMAC_KEYRINGS")
	}
	return result, nil
}

func parseKeyring(versionField, currentVersion, keysField, encoded string) (KeyringConfig, error) {
	if !keyVersionPattern.MatchString(currentVersion) {
		return KeyringConfig{}, invalid(versionField)
	}
	keys := make(map[string][]byte)
	for _, entry := range splitList(encoded) {
		version, key, err := parseKeyEntry(entry, keysField)
		if err != nil {
			return KeyringConfig{}, err
		}
		if _, duplicate := keys[version]; duplicate {
			return KeyringConfig{}, invalid(keysField)
		}
		keys[version] = key
	}
	if _, ok := keys[currentVersion]; !ok {
		return KeyringConfig{}, invalid(versionField)
	}
	return KeyringConfig{CurrentVersion: currentVersion, Keys: keys}, nil
}

func parseKeyEntry(entry, field string) (string, []byte, error) {
	version, value, ok := strings.Cut(entry, "=")
	if !ok || !keyVersionPattern.MatchString(version) {
		return "", nil, invalid(field)
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(key) < minimumHMACKeyBytes {
		return "", nil, invalid(field)
	}
	return version, key, nil
}

func keyringsOverlap(keyrings []KeyringConfig) bool {
	seen := make([][]byte, 0)
	for _, keyring := range keyrings {
		for _, key := range keyring.Keys {
			if containsKey(seen, key) {
				return true
			}
			seen = append(seen, key)
		}
	}
	return false
}

func containsKey(keys [][]byte, candidate []byte) bool {
	for _, key := range keys {
		if bytes.Equal(key, candidate) {
			return true
		}
	}
	return false
}

func (c Phase2Config) validate(
	requireHTTPS bool,
	environment Environment,
	oidcMode OIDCMode,
) error {
	validators := []func() error{
		func() error { return validateInviteURL(c.InviteBaseURL, requireHTTPS) },
		func() error { return validateOrigins(c.CORSAllowedOrigins, requireHTTPS) },
		c.validateDurations,
		c.validateRateLimits,
		func() error { return c.validateAccommodationOnboarding(environment, oidcMode) },
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c Phase2Config) validateAccommodationOnboarding(
	environment Environment,
	oidcMode OIDCMode,
) error {
	if !c.AccommodationOnboardingEnabled {
		return nil
	}
	if environment != EnvironmentLocal && environment != EnvironmentTest {
		return invalid("ACCOMMODATION_ONBOARDING_ENABLED")
	}
	if oidcMode != OIDCModeFake {
		return invalid("ACCOMMODATION_ONBOARDING_ENABLED")
	}
	return nil
}

func (c Phase2Config) validateDurations() error {
	if c.InviteTTL <= 0 || c.InviteTTL > minimumReplayTTL {
		return invalid("INVITE_TTL")
	}
	if c.IdempotencyTTL < minimumReplayTTL || c.IdempotencyTTL < c.InviteTTL {
		return invalid("IDEMPOTENCY_TTL")
	}
	if c.RateLimitWindow <= 0 || c.RateLimitWindow > time.Hour {
		return invalid("RATE_LIMIT_WINDOW")
	}
	return nil
}

func (c Phase2Config) validateRateLimits() error {
	if c.InviteContextRateLimit <= 0 {
		return invalid("INVITE_CONTEXT_RATE_LIMIT")
	}
	if c.InviteSubmitRateLimit <= 0 {
		return invalid("INVITE_SUBMIT_RATE_LIMIT")
	}
	return nil
}

func validateInviteURL(value *url.URL, requireHTTPS bool) error {
	if value == nil || value.User != nil || value.RawQuery != "" || value.Fragment != "" {
		return invalid("INVITE_BASE_URL")
	}
	if requireHTTPS && value.Scheme != "https" {
		return invalid("INVITE_BASE_URL")
	}
	return nil
}

func validateOrigins(origins []string, requireHTTPS bool) error {
	if len(origins) == 0 {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	for _, origin := range origins {
		if err := validateOrigin(origin, requireHTTPS); err != nil {
			return err
		}
	}
	return nil
}

func validateOrigin(origin string, requireHTTPS bool) error {
	parsed, err := parseAbsoluteURL("CORS_ALLOWED_ORIGINS", origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	return nil
}

func parseAbsoluteURL(field, value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, invalid(field)
	}
	return parsed, nil
}

func positiveInteger(lookup LookupEnv, field string, fallback int) int {
	value, ok := lookup(field)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
