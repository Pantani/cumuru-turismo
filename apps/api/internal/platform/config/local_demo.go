package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type LocalDemoConfig struct {
	Application             Config
	WorkerDatabaseURL       string
	ProvisioningDatabaseURL string
}

func LoadLocalDemo(lookup LookupEnv) (LocalDemoConfig, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if err := validateLocalDemoMode(lookup); err != nil {
		return LocalDemoConfig{}, err
	}
	application, err := Load(ProcessWorker, lookup)
	if err != nil {
		return LocalDemoConfig{}, err
	}
	if !application.Phase3.Enabled || !application.Phase4.Enabled {
		return LocalDemoConfig{}, errors.New("local demo requires phases 3 and 4")
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

func validateLocalDemoMode(lookup LookupEnv) error {
	enabled, err := parseBoolean(lookup, "LOCAL_DEMO_ENABLED", false)
	if err != nil || !enabled {
		return invalid("LOCAL_DEMO_ENABLED")
	}
	environment := Environment(required(lookup, "APP_ENV"))
	if environment != EnvironmentLocal && environment != EnvironmentTest {
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
	if duplicateLocalDemoDatabaseURL(
		applicationURL,
		workerURL,
		provisioningURL,
	) {
		return "", "", errors.New("local demo database roles must be distinct")
	}
	return workerURL, provisioningURL, nil
}

func duplicateLocalDemoDatabaseURL(values ...string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.User == nil || parsed.User.Username() == "" {
			return true
		}
		role := parsed.User.Username()
		if seen[role] {
			return true
		}
		seen[role] = true
	}
	return false
}
