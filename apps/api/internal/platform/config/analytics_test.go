package config

import (
	"strings"
	"testing"
)

func TestLoadAnalyticsAcceptsFrozenPrototypePolicy(t *testing.T) {
	t.Parallel()

	values := validAnalyticsValues()
	got, err := loadAnalytics(EnvironmentTest, ProcessAPI, lookupMap(values))
	if err != nil {
		t.Fatalf("loadAnalytics() error = %v", err)
	}
	if err := got.validate(
		ProcessAPI,
		EnvironmentTest,
		values["DATABASE_URL"],
		false,
	); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if got.PrivacyPolicyVersion != analyticsPolicyVersion {
		t.Fatalf("policy = %q", got.PrivacyPolicyVersion)
	}
}

func TestLoadAnalyticsRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		field   string
		value   string
		wantErr string
	}{
		{"missing public DSN", "PUBLIC_DATABASE_URL", "", "PUBLIC_DATABASE_URL"},
		{"shared runtime DSN", "PUBLIC_DATABASE_URL", "same", "PUBLIC_DATABASE_URL"},
		{"lower threshold", "ANALYTICS_PRIMARY_CELL_THRESHOLD", "9", "ANALYTICS_PRIMARY_CELL_THRESHOLD"},
		{"fewer accommodations", "ANALYTICS_MINIMUM_REPORTING_ACCOMMODATIONS", "2", "ANALYTICS_MINIMUM_REPORTING_ACCOMMODATIONS"},
		{"changed rounding", "ANALYTICS_ROUNDING_BASE", "5", "ANALYTICS_ROUNDING_BASE"},
		{"changed weight", "ANALYTICS_PRE_REGISTERED_WEIGHT", "0.9", "ANALYTICS_PRE_REGISTERED_WEIGHT"},
		// A malformed weight used to become NaN, and every comparison against
		// NaN is false, so the frozen-policy check passed it straight through to
		// the forecast arithmetic.
		{"malformed weight", "ANALYTICS_PRE_REGISTERED_WEIGHT", "zero-eight", "ANALYTICS_PRE_REGISTERED_WEIGHT"},
		{"not-a-number weight", "ANALYTICS_PRE_REGISTERED_WEIGHT", "NaN", "ANALYTICS_PRE_REGISTERED_WEIGHT"},
		{"malformed interval", "ANALYTICS_INCREMENTAL_INTERVAL", "15", "ANALYTICS_INCREMENTAL_INTERVAL"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertUnsafeAnalyticsConfig(t, test.field, test.value, test.wantErr)
		})
	}
}

func assertUnsafeAnalyticsConfig(t *testing.T, field, value, wantErr string) {
	t.Helper()
	values := validAnalyticsValues()
	if field == "PUBLIC_DATABASE_URL" && value == "same" {
		value = values["DATABASE_URL"]
	}
	values[field] = value
	got, err := loadAnalytics(EnvironmentTest, ProcessAPI, lookupMap(values))
	if err == nil {
		err = got.validate(
			ProcessAPI,
			EnvironmentTest,
			values["DATABASE_URL"],
			false,
		)
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("analytics error = %v, want %s", err, wantErr)
	}
}

func validAnalyticsValues() map[string]string {
	return map[string]string{
		"APP_ENV":                                    "test",
		"DATABASE_URL":                               "postgres://cumuru_app:fixture@127.0.0.1/cumuru?sslmode=disable",
		"ANALYTICS_ENABLED":                          "true",
		"PUBLIC_DATABASE_URL":                        "postgres://cumuru_public:fixture@127.0.0.1/cumuru?sslmode=disable",
		"PUBLIC_DATABASE_TIMEOUT":                    "3s",
		"ANALYTICS_PRIMARY_CELL_THRESHOLD":           "10",
		"ANALYTICS_MINIMUM_REPORTING_ACCOMMODATIONS": "3",
		"ANALYTICS_ROUNDING_BASE":                    "10",
		"ANALYTICS_PRE_REGISTERED_WEIGHT":            "0.80",
		"ANALYTICS_INCREMENTAL_INTERVAL":             "15m",
		"ANALYTICS_FULL_RECONCILIATION_INTERVAL":     "24h",
	}
}
