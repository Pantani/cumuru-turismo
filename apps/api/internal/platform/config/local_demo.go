package config

import (
	"errors"
	"net/url"
	"strings"
)

type LocalDemoConfig struct {
	Application             Config
	WorkerDatabaseURL       string
	ProvisioningDatabaseURL string
}

func LoadLocalDemo(lookup LookupEnv) (LocalDemoConfig, error) {
	lookup = resolveLookup(lookup)
	if err := validateLocalDemoMode(lookup); err != nil {
		return LocalDemoConfig{}, err
	}
	application, err := localDemoApplication(lookup)
	if err != nil {
		return LocalDemoConfig{}, err
	}
	workerURL, provisioningURL, err := localDemoDatabaseURLs(
		lookup,
		application.DatabaseURL,
	)
	if err != nil {
		return LocalDemoConfig{}, err
	}
	return LocalDemoConfig{
		Application: application, WorkerDatabaseURL: workerURL,
		ProvisioningDatabaseURL: provisioningURL,
	}, nil
}

// The seeder writes fixture data, so it refuses to run anywhere but a local or
// test environment backed by the fake verifier.
func localDemoApplication(lookup LookupEnv) (Config, error) {
	application, err := Load(ProcessWorker, lookup)
	if err != nil {
		return Config{}, err
	}
	if !application.Questionnaire.Enabled || !application.Analytics.Enabled {
		return Config{}, errors.New("local demo requires questionnaire and analytics")
	}
	return application, nil
}

func validateLocalDemoMode(lookup LookupEnv) error {
	enabled, err := parseBoolean(lookup, "LOCAL_DEMO_ENABLED", false)
	if err != nil || !enabled {
		return invalid("LOCAL_DEMO_ENABLED")
	}
	if !localOrTest(Environment(required(lookup, "APP_ENV"))) {
		return invalid("APP_ENV")
	}
	if OIDCMode(required(lookup, "OIDC_MODE")) != OIDCModeFake {
		return invalid("OIDC_MODE")
	}
	return nil
}

func localDemoDatabaseURLs(
	lookup LookupEnv,
	applicationURL string,
) (string, string, error) {
	workerURL := required(lookup, "WORKER_DATABASE_URL")
	provisioningURL := required(lookup, "PROVISIONING_DATABASE_URL")
	if err := validDatabaseURLField(
		"WORKER_DATABASE_URL",
		workerURL,
		false,
	); err != nil {
		return "", "", err
	}
	if err := validDatabaseURLField(
		"PROVISIONING_DATABASE_URL",
		provisioningURL,
		false,
	); err != nil {
		return "", "", err
	}
	if duplicateDatabaseRoles(
		applicationURL,
		workerURL,
		provisioningURL,
	) {
		return "", "", errors.New("local demo database roles must be distinct")
	}
	return workerURL, provisioningURL, nil
}

// Each process must connect as a distinct role so the grant model actually
// constrains it.
func duplicateDatabaseRoles(values ...string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		role, ok := databaseRole(value)
		if !ok || seen[role] {
			return true
		}
		seen[role] = true
	}
	return false
}

func databaseRole(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User == nil {
		return "", false
	}
	role := parsed.User.Username()
	return role, role != ""
}
