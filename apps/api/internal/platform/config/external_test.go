package config

import (
	"strings"
	"testing"
)

func TestLoadExternalContextAcceptsPrototypeIngestion(t *testing.T) {
	t.Parallel()

	values := validExternalContextValues()
	got, err := loadExternalContext(
		EnvironmentTest,
		ProcessWorker,
		lookupMap(values),
	)
	if err != nil {
		t.Fatalf("loadExternalContext() error = %v", err)
	}
	if err := got.validate(
		ProcessWorker,
		EnvironmentTest,
		values["DATABASE_URL"],
		false,
	); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if got.UserAgent != externalContextUserAgent {
		t.Fatalf("user agent = %q", got.UserAgent)
	}
}

// The API never reads the ingestion DSN: egress happens in the worker, and the
// API reaches the layer through the public pool and the view in public_data.
func TestLoadExternalContextLeavesAPIWithoutIngestionDSN(t *testing.T) {
	t.Parallel()

	values := validExternalContextValues()
	delete(values, "EXTERNAL_DATABASE_URL")
	got, err := loadExternalContext(
		EnvironmentTest,
		ProcessAPI,
		lookupMap(values),
	)
	if err != nil {
		t.Fatalf("loadExternalContext() error = %v", err)
	}
	if got.DatabaseURL != "" {
		t.Fatalf("api ingestion DSN = %q, want empty", got.DatabaseURL)
	}
	if err := got.validate(
		ProcessAPI,
		EnvironmentTest,
		values["DATABASE_URL"],
		false,
	); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestLoadExternalContextRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		field   string
		value   string
		wantErr string
	}{
		{"deployed environment", "APP_ENV", "production", "EXTERNAL_CONTEXT_ENABLED"},
		{"missing ingestion DSN", "EXTERNAL_DATABASE_URL", "", "EXTERNAL_DATABASE_URL"},
		// Reusing the application role would leave the grant model of migration
		// 000003 constraining nothing.
		{"shared runtime role", "EXTERNAL_DATABASE_URL", "same", "EXTERNAL_DATABASE_URL"},
		{"empty allowlist", "EXTERNAL_ALLOWED_HOSTS", ",", "EXTERNAL_ALLOWED_HOSTS"},
		// A URL in the allowlist would never match the host of the constant URL
		// the fetcher builds, hiding a mismatch instead of failing on it.
		{"allowlist with scheme", "EXTERNAL_ALLOWED_HOSTS", "https://api.open-meteo.com", "EXTERNAL_ALLOWED_HOSTS"},
		{"allowlist with path", "EXTERNAL_ALLOWED_HOSTS", "api.open-meteo.com/v1", "EXTERNAL_ALLOWED_HOSTS"},
		{"allowlist with port", "EXTERNAL_ALLOWED_HOSTS", "api.open-meteo.com:443", "EXTERNAL_ALLOWED_HOSTS"},
		{"batch outlasting its cycle", "EXTERNAL_BATCH_BUDGET", "7h", "EXTERNAL_BATCH_BUDGET"},
		{"zero batch budget", "EXTERNAL_BATCH_BUDGET", "0s", "EXTERNAL_BATCH_BUDGET"},
		{"malformed timeout", "EXTERNAL_REQUEST_TIMEOUT", "ten", "EXTERNAL_REQUEST_TIMEOUT"},
		{"unbounded response", "EXTERNAL_MAX_RESPONSE_BYTES", "0", "EXTERNAL_MAX_RESPONSE_BYTES"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertUnsafeExternalContextConfig(
				t,
				test.field,
				test.value,
				test.wantErr,
			)
		})
	}
}

func assertUnsafeExternalContextConfig(
	t *testing.T,
	field, value, wantErr string,
) {
	t.Helper()
	values := validExternalContextValues()
	environment := EnvironmentTest
	if field == "EXTERNAL_DATABASE_URL" && value == "same" {
		value = values["DATABASE_URL"]
	}
	values[field] = value
	if field == "APP_ENV" {
		environment = Environment(value)
	}
	got, err := loadExternalContext(
		environment,
		ProcessWorker,
		lookupMap(values),
	)
	if err == nil {
		err = got.validate(
			ProcessWorker,
			environment,
			values["DATABASE_URL"],
			false,
		)
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("external context error = %v, want %s", err, wantErr)
	}
}

func validExternalContextValues() map[string]string {
	return map[string]string{
		"APP_ENV":                     "test",
		"DATABASE_URL":                "postgres://cumuru_worker:fixture@127.0.0.1/cumuru?sslmode=disable",
		"EXTERNAL_CONTEXT_ENABLED":    "true",
		"EXTERNAL_DATABASE_URL":       "postgres://cumuru_external:fixture@127.0.0.1/cumuru?sslmode=disable",
		"EXTERNAL_ALLOWED_HOSTS":      "api.open-meteo.com",
		"EXTERNAL_REQUEST_TIMEOUT":    "10s",
		"EXTERNAL_BATCH_BUDGET":       "2m",
		"EXTERNAL_MAX_RESPONSE_BYTES": "2097152",
		"EXTERNAL_INGESTION_INTERVAL": "6h",
	}
}
