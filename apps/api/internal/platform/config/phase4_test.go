package config

import (
	"strings"
	"testing"
)

func TestLoadPhase4AcceptsFrozenPrototypePolicy(t *testing.T) {
	t.Parallel()

	values := validPhase4Values()
	got, err := loadPhase4(EnvironmentTest, ProcessAPI, lookupMap(values))
	if err != nil {
		t.Fatalf("loadPhase4() error = %v", err)
	}
	if err := got.validate(
		ProcessAPI,
		EnvironmentTest,
		values["DATABASE_URL"],
		false,
	); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if got.PrivacyPolicyVersion != phase4PolicyVersion {
		t.Fatalf("policy = %q", got.PrivacyPolicyVersion)
	}
}

func TestLoadPhase4RejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		field   string
		value   string
		wantErr string
	}{
		{"missing public DSN", "PUBLIC_DATABASE_URL", "", "PUBLIC_DATABASE_URL"},
		{"shared runtime DSN", "PUBLIC_DATABASE_URL", "same", "PUBLIC_DATABASE_URL"},
		{"lower threshold", "PHASE4_PRIMARY_CELL_THRESHOLD", "9", "PHASE4_PRIMARY_CELL_THRESHOLD"},
		{"fewer accommodations", "PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS", "2", "PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS"},
		{"changed rounding", "PHASE4_ROUNDING_BASE", "5", "PHASE4_ROUNDING_BASE"},
		{"changed weight", "PHASE4_PRE_REGISTERED_WEIGHT", "0.9", "PHASE4_PRE_REGISTERED_WEIGHT"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertUnsafePhase4Config(t, test.field, test.value, test.wantErr)
		})
	}
}

func assertUnsafePhase4Config(t *testing.T, field, value, wantErr string) {
	t.Helper()
	values := validPhase4Values()
	if field == "PUBLIC_DATABASE_URL" && value == "same" {
		value = values["DATABASE_URL"]
	}
	values[field] = value
	got, err := loadPhase4(EnvironmentTest, ProcessAPI, lookupMap(values))
	if err == nil {
		err = got.validate(
			ProcessAPI,
			EnvironmentTest,
			values["DATABASE_URL"],
			false,
		)
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("phase 4 error = %v, want %s", err, wantErr)
	}
}

func validPhase4Values() map[string]string {
	return map[string]string{
		"APP_ENV":                                 "test",
		"DATABASE_URL":                            "postgres://cumuru_app:fixture@127.0.0.1/cumuru?sslmode=disable",
		"PHASE4_ENABLED":                          "true",
		"PUBLIC_DATABASE_URL":                     "postgres://cumuru_public:fixture@127.0.0.1/cumuru?sslmode=disable",
		"PUBLIC_DATABASE_TIMEOUT":                 "3s",
		"PHASE4_PRIMARY_CELL_THRESHOLD":           "10",
		"PHASE4_MINIMUM_REPORTING_ACCOMMODATIONS": "3",
		"PHASE4_ROUNDING_BASE":                    "10",
		"PHASE4_PRE_REGISTERED_WEIGHT":            "0.80",
		"PHASE4_INCREMENTAL_INTERVAL":             "15m",
		"PHASE4_FULL_RECONCILIATION_INTERVAL":     "24h",
	}
}
