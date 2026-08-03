package config_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

func TestLoadLocalDemoFailsClosedAndSeparatesDatabaseRoles(t *testing.T) {
	t.Parallel()

	valid := localDemoValues()
	tests := []struct {
		name    string
		change  map[string]string
		wantErr string
	}{
		{name: "valid"},
		{
			name: "test environment",
			change: map[string]string{
				"APP_ENV": "test",
			},
		},
		{
			name: "disabled",
			change: map[string]string{
				"LOCAL_DEMO_ENABLED": "false",
			},
			wantErr: "LOCAL_DEMO_ENABLED",
		},
		{
			name: "production",
			change: map[string]string{
				"APP_ENV": "production",
			},
			wantErr: "APP_ENV",
		},
		{
			name: "real oidc",
			change: map[string]string{
				"OIDC_MODE": "real",
			},
			wantErr: "OIDC_MODE",
		},
		{
			name: "same worker role",
			change: map[string]string{
				"WORKER_DATABASE_URL": valid["DATABASE_URL"],
			},
			wantErr: "roles must be distinct",
		},
		{
			name: "same worker username in another dsn",
			change: map[string]string{
				"WORKER_DATABASE_URL": "postgres://local:other@postgres:5432/other?sslmode=disable",
			},
			wantErr: "roles must be distinct",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertLocalDemoLoad(t, merge(valid, test.change), test.wantErr)
		})
	}
}

func assertLocalDemoLoad(
	t *testing.T,
	values map[string]string,
	wantErr string,
) {
	t.Helper()
	_, err := config.LoadLocalDemo(lookup(values))
	if wantErr == "" && err != nil {
		t.Fatalf("LoadLocalDemo() error = %v", err)
	}
	if wantErr != "" &&
		(err == nil || !strings.Contains(err.Error(), wantErr)) {
		t.Fatalf("LoadLocalDemo() error = %v, want %q", err, wantErr)
	}
}

func localDemoValues() map[string]string {
	return merge(validLocal(), map[string]string{
		"LOCAL_DEMO_ENABLED":                  "true",
		"PHASE3_ENABLED":                      "true",
		"SURVEY_CAPABILITY_TTL":               "24h",
		"SURVEY_FREE_TEXT_TTL":                "24h",
		"SURVEY_SUBMIT_RATE_LIMIT":            "10",
		"SURVEY_FREE_TEXT_CLEANUP_ENABLED":    "true",
		"SURVEY_HMAC_CURRENT_VERSION":         "survey-v1",
		"SURVEY_HMAC_KEYS":                    localDemoEncodedKey("survey-v1", "survey"),
		"SURVEY_FREE_TEXT_CURRENT_VERSION":    "free-v1",
		"SURVEY_FREE_TEXT_KEYS":               localDemoEncodedKey("free-v1", "free-text"),
		"PHASE4_ENABLED":                      "true",
		"PHASE4_INCREMENTAL_INTERVAL":         "15m",
		"PHASE4_FULL_RECONCILIATION_INTERVAL": "24h",
		"WORKER_DATABASE_URL":                 "postgres://worker:local@127.0.0.1:5432/cumuru?sslmode=disable",
		"PROVISIONING_DATABASE_URL":           "postgres://migration:local@127.0.0.1:5432/cumuru?sslmode=disable",
	})
}

func localDemoEncodedKey(version, seed string) string {
	key := make([]byte, 32)
	copy(key, seed)
	for index := len(seed); index < len(key); index++ {
		key[index] = byte(index + 1)
	}
	return version + "=" + base64.StdEncoding.EncodeToString(key)
}
