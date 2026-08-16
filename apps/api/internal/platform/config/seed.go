package config

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

// SeedProfile bounds what the seeder may write. It is the single switch that
// separates an environment which only needs an administrator from one that may
// also receive prototype fixtures.
type SeedProfile string

const (
	// SeedProfileNone keeps the seeder inert; it connects to nothing.
	SeedProfileNone SeedProfile = "none"
	// SeedProfileAdmin writes the administrator account and, when a catalog is
	// supplied, the accommodations it lists. It is the only profile a deployed
	// environment accepts.
	SeedProfileAdmin SeedProfile = "admin"
	// SeedProfileAdminDemo adds the fictional demo fixtures on top of the
	// administrator. Local and test only.
	SeedProfileAdminDemo SeedProfile = "admin+demo"
)

const defaultAdminDisplayName = "Administração Cumuru"

// accountEmailPattern mirrors the accounts_email_shaped database constraint so a
// rejected address fails during configuration instead of at INSERT time.
var accountEmailPattern = regexp.MustCompile(
	`^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$`,
)

// SeedAdmin is the bootstrap administrator. The secret is never compiled in and
// never defaulted: an absent value stops the seeder instead of creating an
// account with a guessable credential.
type SeedAdmin struct {
	Email       string
	DisplayName string
	Password    string
	MustChange  bool
}

// SeedConfig is loaded by the seeder binary only. The API and the worker never
// read it, so no runtime process carries the bootstrap secret.
type SeedConfig struct {
	Application             Config
	Profile                 SeedProfile
	ProvisioningDatabaseURL string
	Admin                   SeedAdmin
	CatalogPath             string
}

func LoadSeed(lookup LookupEnv) (SeedConfig, error) {
	lookup = resolveLookup(lookup)
	application, err := Load(ProcessWorker, lookup)
	if err != nil {
		return SeedConfig{}, err
	}
	profile, err := seedProfile(application.Environment, lookup)
	if err != nil {
		return SeedConfig{}, err
	}
	if profile == SeedProfileNone {
		return SeedConfig{Application: application, Profile: profile}, nil
	}
	cfg, err := activeSeedConfig(application, profile, lookup)
	if err != nil {
		return SeedConfig{}, err
	}
	return cfg, cfg.validate()
}

func activeSeedConfig(
	application Config,
	profile SeedProfile,
	lookup LookupEnv,
) (SeedConfig, error) {
	admin, err := seedAdmin(application.Environment, lookup)
	if err != nil {
		return SeedConfig{}, err
	}
	return SeedConfig{
		Application:             application,
		Profile:                 profile,
		ProvisioningDatabaseURL: required(lookup, "PROVISIONING_DATABASE_URL"),
		Admin:                   admin,
		CatalogPath:             optional(lookup, "SEED_ACCOMMODATION_CATALOG", ""),
	}, nil
}

// seedProfile derives both switches from the environment: a deployed
// environment stays inert until an operator turns SEED_ENABLED on explicitly.
func seedProfile(
	environment Environment,
	lookup LookupEnv,
) (SeedProfile, error) {
	prototype := localOrTest(environment)
	enabled, err := parseBoolean(lookup, "SEED_ENABLED", prototype)
	if err != nil {
		return "", err
	}
	if !enabled {
		return SeedProfileNone, nil
	}
	return SeedProfile(optional(
		lookup, "SEED_PROFILE", string(defaultSeedProfile(prototype)),
	)), nil
}

func defaultSeedProfile(prototype bool) SeedProfile {
	if prototype {
		return SeedProfileAdminDemo
	}
	return SeedProfileAdmin
}

func seedAdmin(
	environment Environment,
	lookup LookupEnv,
) (SeedAdmin, error) {
	mustChange, err := adminMustChangePassword(environment, lookup)
	if err != nil {
		return SeedAdmin{}, err
	}
	return SeedAdmin{
		Email: strings.ToLower(required(lookup, "SEED_ADMIN_EMAIL")),
		DisplayName: optional(
			lookup, "SEED_ADMIN_DISPLAY_NAME", defaultAdminDisplayName,
		),
		Password:   required(lookup, "SEED_ADMIN_PASSWORD"),
		MustChange: mustChange,
	}, nil
}

// adminMustChangePassword defaults to forced rotation everywhere but local and
// test, and refuses to be relaxed in a deployed environment: a bootstrap secret
// that survives the first login is a long-lived shared credential.
func adminMustChangePassword(
	environment Environment,
	lookup LookupEnv,
) (bool, error) {
	prototype := localOrTest(environment)
	mustChange, err := parseBoolean(
		lookup, "SEED_ADMIN_MUST_CHANGE_PASSWORD", !prototype,
	)
	if err != nil {
		return false, err
	}
	if !mustChange && !prototype {
		return false, invalid("SEED_ADMIN_MUST_CHANGE_PASSWORD")
	}
	return mustChange, nil
}

func (s SeedConfig) validate() error {
	return firstError(
		s.validateProfile,
		s.validateAdmin,
		s.validateDatabases,
	)
}

// validateProfile keeps the fictional fixtures out of staging and production.
// The profile is the boundary; no downstream check repeats it.
func (s SeedConfig) validateProfile() error {
	switch s.Profile {
	case SeedProfileNone, SeedProfileAdmin:
		return nil
	case SeedProfileAdminDemo:
		if !localOrTest(s.Application.Environment) {
			return invalid("SEED_PROFILE")
		}
		return nil
	default:
		return invalid("SEED_PROFILE")
	}
}

func (s SeedConfig) validateAdmin() error {
	if !accountEmailPattern.MatchString(s.Admin.Email) {
		return invalid("SEED_ADMIN_EMAIL")
	}
	if strings.TrimSpace(s.Admin.DisplayName) == "" {
		return invalid("SEED_ADMIN_DISPLAY_NAME")
	}
	if access.ValidatePasswordLength(s.Admin.Password) != nil {
		return invalid("SEED_ADMIN_PASSWORD")
	}
	return nil
}

// validateDatabases keeps the seeder on the provisioning role: the application
// role has no INSERT grant on auth.accounts, so reusing it would fail closed at
// runtime instead of at startup.
func (s SeedConfig) validateDatabases() error {
	if err := validDatabaseURLField(
		"PROVISIONING_DATABASE_URL",
		s.ProvisioningDatabaseURL,
		s.Application.requiresHTTPS(),
	); err != nil {
		return err
	}
	if duplicateDatabaseRoles(
		s.Application.DatabaseURL,
		s.ProvisioningDatabaseURL,
	) {
		return errors.New("seed database roles must be distinct")
	}
	return nil
}
