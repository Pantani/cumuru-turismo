package config

import (
	"errors"
	"time"
)

const (
	minimumSessionIdleTTL     = time.Minute
	maximumSessionAbsoluteTTL = 24 * time.Hour
	maximumLockoutDuration    = time.Hour
)

// AuthConfig governs the local e-mail and password track. It exists so an
// operator without CNPJ, Cadastur or a federal credential can reach the local
// observatory; it never replaces the municipal IdP when one is configured.
type AuthConfig struct {
	Enabled            bool
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	MaxFailedAttempts  int
	LockoutDuration    time.Duration
}

func loadAuth(lookup LookupEnv) (AuthConfig, error) {
	enabled, err := parseBoolean(lookup, "LOCAL_AUTH_ENABLED", true)
	if err != nil {
		return AuthConfig{}, err
	}
	auth := AuthConfig{
		Enabled:            enabled,
		SessionIdleTTL:     duration(lookup, "SESSION_IDLE_TTL", 30*time.Minute),
		SessionAbsoluteTTL: duration(lookup, "SESSION_ABSOLUTE_TTL", 12*time.Hour),
		MaxFailedAttempts:  positiveInteger(lookup, "LOGIN_MAX_FAILED_ATTEMPTS", 5),
		LockoutDuration:    duration(lookup, "LOGIN_LOCKOUT_DURATION", 15*time.Minute),
	}
	if !enabled {
		return auth, nil
	}
	return auth, auth.validate()
}

func (a AuthConfig) validate() error {
	return firstError(a.validateSessionTTLs, a.validateLockout)
}

func (a AuthConfig) validateSessionTTLs() error {
	if a.SessionIdleTTL < minimumSessionIdleTTL {
		return errors.New("SESSION_IDLE_TTL is below the supported minimum")
	}
	if a.SessionAbsoluteTTL < a.SessionIdleTTL {
		return errors.New("SESSION_ABSOLUTE_TTL must not be shorter than SESSION_IDLE_TTL")
	}
	if a.SessionAbsoluteTTL > maximumSessionAbsoluteTTL {
		return errors.New("SESSION_ABSOLUTE_TTL exceeds the supported maximum")
	}
	return nil
}

func (a AuthConfig) validateLockout() error {
	if a.MaxFailedAttempts < 3 || a.MaxFailedAttempts > 100 {
		return errors.New("LOGIN_MAX_FAILED_ATTEMPTS must stay between 3 and 100")
	}
	if a.LockoutDuration <= 0 || a.LockoutDuration > maximumLockoutDuration {
		return errors.New("LOGIN_LOCKOUT_DURATION must be positive and at most one hour")
	}
	return nil
}
