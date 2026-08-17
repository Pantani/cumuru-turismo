package config

import (
	"math"
	"net/url"
	"time"
)

const (
	// maximumChallengeTTL keeps a challenge from becoming stock to farm. Ten
	// minutes is enough to fill the form on a slow phone and short enough that a
	// harvested batch is worthless before it is used.
	maximumChallengeTTL = time.Hour

	minDifficultyBits = 1
	maxDifficultyBits = 32
)

// SelfServiceConfig carries the open self-registration channel and the account
// activation capability.
type SelfServiceConfig struct {
	Enabled bool
	// SelfRegistrationURL and ActivationURL carry the token in the fragment
	// (/i#token, /ativacao#token). A fragment never leaves the browser, so the
	// token cannot reach a request line, an access log, a WAF or a CDN — which
	// matters far more for a poster that stays on a wall for months than for a
	// 72-hour invite (ADR-039).
	SelfRegistrationURL         *url.URL
	ActivationURL               *url.URL
	ChallengeTTL                time.Duration
	DifficultyBase              uint8
	DifficultyCeiling           uint8
	DifficultyRequestsPerBit    int32
	SelfServiceContextRateLimit int
	SelfServiceSubmitRateLimit  int
	ActivationContextRateLimit  int
	ActivationSubmitRateLimit   int
	ExpirySweepBatchSize        int32
	ProofOfWorkKeys             KeyringConfig
}

func loadSelfService(
	environment Environment,
	core CoreConfig,
	lookup LookupEnv,
) (SelfServiceConfig, error) {
	enabled, err := parseBoolean(lookup, "SELF_SERVICE_ENABLED", false)
	if err != nil {
		return SelfServiceConfig{}, err
	}
	if !enabled {
		return SelfServiceConfig{}, nil
	}
	if !localOrTest(environment) {
		return SelfServiceConfig{}, invalid("SELF_SERVICE_ENABLED")
	}
	defaults, err := selfServiceDefaults(lookup)
	if err != nil {
		return SelfServiceConfig{}, err
	}
	return applySelfServiceSettings(defaults, core, lookup)
}

func applySelfServiceSettings(
	config SelfServiceConfig,
	core CoreConfig,
	lookup LookupEnv,
) (SelfServiceConfig, error) {
	if err := applySelfServiceKeyring(&config, core, lookup); err != nil {
		return SelfServiceConfig{}, err
	}
	if err := applySelfServiceURLs(&config, core); err != nil {
		return SelfServiceConfig{}, err
	}
	return config, nil
}

// A malformed value stops the load naming its own field, instead of becoming a
// zero that a later range check has to catch — the difficulty and rate limits
// below are exactly the kind of field where a silent zero would be read as
// "no proof of work required".
func selfServiceDefaults(lookup LookupEnv) (SelfServiceConfig, error) {
	reader := newEnvReader(lookup)
	config := SelfServiceConfig{
		Enabled:      true,
		ChallengeTTL: reader.duration("PROOF_OF_WORK_TTL", 10*time.Minute),
		DifficultyBase: uint8(reader.integerInRange(
			"PROOF_OF_WORK_DIFFICULTY_BASE", 12,
			minDifficultyBits, maxDifficultyBits,
		)),
		DifficultyCeiling: uint8(reader.integerInRange(
			"PROOF_OF_WORK_DIFFICULTY_CEILING", 18,
			minDifficultyBits, maxDifficultyBits,
		)),
		DifficultyRequestsPerBit: int32(reader.integerInRange(
			"PROOF_OF_WORK_REQUESTS_PER_BIT", 2, 1, math.MaxInt32,
		)),
		SelfServiceContextRateLimit: reader.integer(
			"SELF_SERVICE_CONTEXT_RATE_LIMIT", 60,
		),
		SelfServiceSubmitRateLimit: reader.integer(
			"SELF_SERVICE_SUBMIT_RATE_LIMIT", 20,
		),
		ActivationContextRateLimit: reader.integer(
			"ACTIVATION_CONTEXT_RATE_LIMIT", 30,
		),
		ActivationSubmitRateLimit: reader.integer(
			"ACTIVATION_SUBMIT_RATE_LIMIT", 10,
		),
		ExpirySweepBatchSize: int32(reader.integerInRange(
			"APPROVAL_EXPIRY_BATCH_SIZE", 200, 1, math.MaxInt32,
		)),
	}
	if err := reader.Err(); err != nil {
		return SelfServiceConfig{}, err
	}
	return config, nil
}

// The proof-of-work keyring must not share a key with any other ring. A leaked
// challenge key would otherwise let somebody forge an invite token or an
// idempotency digest, which are unrelated domains.
func applySelfServiceKeyring(
	config *SelfServiceConfig,
	core CoreConfig,
	lookup LookupEnv,
) error {
	keyring, err := parseKeyring(
		"PROOF_OF_WORK_HMAC_CURRENT_VERSION",
		required(lookup, "PROOF_OF_WORK_HMAC_CURRENT_VERSION"),
		"PROOF_OF_WORK_HMAC_KEYS",
		required(lookup, "PROOF_OF_WORK_HMAC_KEYS"),
	)
	if err != nil {
		return err
	}
	if keyringsOverlap(append(core.keyrings(), keyring)) {
		return invalid("PROOF_OF_WORK_HMAC_KEYS")
	}
	config.ProofOfWorkKeys = keyring
	return nil
}

// Both surfaces live on the origin the invite already uses, so the feature adds
// no environment variable and no second host to keep in step with the nginx
// configuration.
func applySelfServiceURLs(config *SelfServiceConfig, core CoreConfig) error {
	if !bareURL(core.InviteBaseURL) {
		return invalid("INVITE_BASE_URL")
	}
	origin := *core.InviteBaseURL
	origin.Path = ""
	selfRegistration := origin
	selfRegistration.Path = "/i"
	activation := origin
	activation.Path = "/ativacao"
	config.SelfRegistrationURL = &selfRegistration
	config.ActivationURL = &activation
	return nil
}

func (c SelfServiceConfig) validate() error {
	if !c.Enabled {
		return nil
	}
	validators := []func() error{
		c.validateChallenge,
		c.validateDifficulty,
		c.validateRateLimits,
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

// An absent or unusable key fails the process at startup. Generating a
// challenge with an empty key would produce a MAC anybody can forge, and the
// whole control would be theatre.
func (c SelfServiceConfig) validateChallenge() error {
	if c.ChallengeTTL <= 0 || c.ChallengeTTL > maximumChallengeTTL {
		return invalid("PROOF_OF_WORK_TTL")
	}
	if !c.usableKey() {
		return invalid("PROOF_OF_WORK_HMAC_KEYS")
	}
	if c.ExpirySweepBatchSize <= 0 {
		return invalid("APPROVAL_EXPIRY_BATCH_SIZE")
	}
	return nil
}

func (c SelfServiceConfig) usableKey() bool {
	key, ok := c.ProofOfWorkKeys.Key(c.ProofOfWorkKeys.CurrentVersion)
	return ok && len(key) >= minimumHMACKeyBytes
}

func (c SelfServiceConfig) validateDifficulty() error {
	if !inDifficultyRange(c.DifficultyBase) {
		return invalid("PROOF_OF_WORK_DIFFICULTY_BASE")
	}
	if c.DifficultyCeiling < c.DifficultyBase || !inDifficultyRange(c.DifficultyCeiling) {
		return invalid("PROOF_OF_WORK_DIFFICULTY_CEILING")
	}
	if c.DifficultyRequestsPerBit <= 0 {
		return invalid("PROOF_OF_WORK_REQUESTS_PER_BIT")
	}
	return nil
}

func inDifficultyRange(bits uint8) bool {
	return bits >= minDifficultyBits && bits <= maxDifficultyBits
}

// The limits are calibrated not to deny service behind CGNAT, where one /24 can
// cover thousands of subscribers. The cost curve is the proof-of-work
// difficulty, not a tighter bucket.
func (c SelfServiceConfig) validateRateLimits() error {
	limits := map[string]int{
		"SELF_SERVICE_CONTEXT_RATE_LIMIT": c.SelfServiceContextRateLimit,
		"SELF_SERVICE_SUBMIT_RATE_LIMIT":  c.SelfServiceSubmitRateLimit,
		"ACTIVATION_CONTEXT_RATE_LIMIT":   c.ActivationContextRateLimit,
		"ACTIVATION_SUBMIT_RATE_LIMIT":    c.ActivationSubmitRateLimit,
	}
	for field, limit := range limits {
		if limit <= 0 {
			return invalid(field)
		}
	}
	return nil
}
