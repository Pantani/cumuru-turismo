package config

import (
	"net/url"
	"time"
)

const minimumReplayTTL = 30 * 24 * time.Hour

// CoreConfig carries the assisted registration slice — accommodations, stays
// and invites — together with the keyrings, replay window and rate limits every
// other feature builds on.
type CoreConfig struct {
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
	DocumentKeys                   KeyringConfig
}

// keyrings lists every ring CoreConfig owns. A ring missing from this list is
// one another feature's key may silently duplicate, which is exactly what the
// overlap check exists to prevent — DocumentKeys was added to CoreConfig
// without being listed, so a survey key could share the key that blinds a CPF.
func (c CoreConfig) keyrings() []KeyringConfig {
	return []KeyringConfig{
		c.InviteKeys,
		c.ActorKeys,
		c.IdempotencyKeys,
		c.RateLimitKeys,
		c.CursorKeys,
		c.DocumentKeys,
	}
}

func loadCore(lookup LookupEnv) (CoreConfig, error) {
	onboardingEnabled, err := parseBoolean(
		lookup,
		"ACCOMMODATION_ONBOARDING_ENABLED",
		false,
	)
	if err != nil {
		return CoreConfig{}, err
	}
	baseURL, err := parseAbsoluteURL("INVITE_BASE_URL", required(lookup, "INVITE_BASE_URL"))
	if err != nil {
		return CoreConfig{}, err
	}
	keyrings, err := loadKeyrings(lookup)
	if err != nil {
		return CoreConfig{}, err
	}
	return readCoreSettings(lookup, baseURL, onboardingEnabled, keyrings)
}

func readCoreSettings(
	lookup LookupEnv,
	baseURL *url.URL,
	onboardingEnabled bool,
	keyrings [6]KeyringConfig,
) (CoreConfig, error) {
	reader := newEnvReader(lookup)
	config := CoreConfig{
		InviteBaseURL:                  baseURL,
		InviteTTL:                      reader.duration("INVITE_TTL", 72*time.Hour),
		IdempotencyTTL:                 reader.duration("IDEMPOTENCY_TTL", minimumReplayTTL),
		RateLimitWindow:                reader.duration("RATE_LIMIT_WINDOW", time.Minute),
		InviteContextRateLimit:         reader.integer("INVITE_CONTEXT_RATE_LIMIT", 30),
		InviteSubmitRateLimit:          reader.integer("INVITE_SUBMIT_RATE_LIMIT", 10),
		AccommodationOnboardingEnabled: onboardingEnabled,
		CORSAllowedOrigins:             splitList(required(lookup, "CORS_ALLOWED_ORIGINS")),
		InviteKeys:                     keyrings[0],
		ActorKeys:                      keyrings[1],
		IdempotencyKeys:                keyrings[2],
		RateLimitKeys:                  keyrings[3],
		CursorKeys:                     keyrings[4],
		DocumentKeys:                   keyrings[5],
	}
	if err := reader.Err(); err != nil {
		return CoreConfig{}, err
	}
	return config, nil
}

func (c CoreConfig) validate(
	requireHTTPS bool,
	environment Environment,
	oidcMode OIDCMode,
) error {
	return firstError(
		func() error { return validateInviteURL(c.InviteBaseURL, requireHTTPS) },
		func() error { return validateOrigins(c.CORSAllowedOrigins, requireHTTPS) },
		c.validateDurations,
		c.validateRateLimits,
		func() error { return c.validateAccommodationOnboarding(environment, oidcMode) },
	)
}

func (c CoreConfig) validateAccommodationOnboarding(
	environment Environment,
	oidcMode OIDCMode,
) error {
	if !c.AccommodationOnboardingEnabled {
		return nil
	}
	if !localOrTest(environment) {
		return invalid("ACCOMMODATION_ONBOARDING_ENABLED")
	}
	if oidcMode != OIDCModeFake {
		return invalid("ACCOMMODATION_ONBOARDING_ENABLED")
	}
	return nil
}

// The replay window must outlast any invite, otherwise a retry could execute a
// second time after its idempotency record expired.
func (c CoreConfig) validateDurations() error {
	return firstError(c.validateInviteWindow, c.validateRateWindow)
}

func (c CoreConfig) validateInviteWindow() error {
	if c.InviteTTL <= 0 || c.InviteTTL > minimumReplayTTL {
		return invalid("INVITE_TTL")
	}
	if !c.validReplayWindow() {
		return invalid("IDEMPOTENCY_TTL")
	}
	return nil
}

func (c CoreConfig) validateRateWindow() error {
	if c.RateLimitWindow <= 0 || c.RateLimitWindow > time.Hour {
		return invalid("RATE_LIMIT_WINDOW")
	}
	return nil
}

func (c CoreConfig) validReplayWindow() bool {
	return c.IdempotencyTTL >= minimumReplayTTL && c.IdempotencyTTL >= c.InviteTTL
}

func (c CoreConfig) validateRateLimits() error {
	if c.InviteContextRateLimit <= 0 {
		return invalid("INVITE_CONTEXT_RATE_LIMIT")
	}
	if c.InviteSubmitRateLimit <= 0 {
		return invalid("INVITE_SUBMIT_RATE_LIMIT")
	}
	return nil
}
