package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

func TestLoadAPIValidatesEnvironmentAndModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{
			name:   "local fake",
			values: validLocal(),
		},
		{
			name: "unknown environment",
			values: merge(validLocal(), map[string]string{
				"APP_ENV": "preview",
			}),
			wantErr: "APP_ENV",
		},
		{
			name: "fake in production",
			values: merge(validProduction(), map[string]string{
				"OIDC_MODE": "fake",
			}),
			wantErr: "OIDC_MODE",
		},
		{
			name: "production without telemetry endpoint",
			values: merge(validProduction(), map[string]string{
				"OTEL_ENDPOINT": "",
			}),
			wantErr: "OTEL_ENDPOINT",
		},
		{
			name: "production issuer must use https",
			values: merge(validProduction(), map[string]string{
				"OIDC_ISSUER": "http://identity.invalid",
			}),
			wantErr: "OIDC_ISSUER",
		},
		{
			name: "database URL is required",
			values: merge(validLocal(), map[string]string{
				"DATABASE_URL": "",
			}),
			wantErr: "DATABASE_URL",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLoadResult(t, tt.values, tt.wantErr)
		})
	}
}

func assertLoadResult(t *testing.T, values map[string]string, wantErr string) {
	t.Helper()
	got, err := config.Load(config.ProcessAPI, lookup(values))
	if wantErr != "" {
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Load() error = %v, want field %q", err, wantErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.DatabaseTimeout != 3*time.Second {
		t.Fatalf("DatabaseTimeout = %s, want 3s", got.DatabaseTimeout)
	}
}

func TestLoadDoesNotExposeSecretValues(t *testing.T) {
	t.Parallel()

	const secret = "super-secret-password"
	values := merge(validLocal(), map[string]string{
		"DATABASE_URL": "postgres://user:" + secret + "@%",
	})

	_, err := config.Load(config.ProcessAPI, lookup(values))
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked secret: %q", err)
	}
}

func TestLoadRequiresDatabaseTLSOutsideLocalAndTest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     string
		dsn     string
		wantErr bool
	}{
		{
			name:    "production missing sslmode",
			env:     "production",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru",
			wantErr: true,
		},
		{
			name:    "production disables TLS",
			env:     "production",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=disable",
			wantErr: true,
		},
		{
			name:    "staging rejects opportunistic TLS",
			env:     "staging",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=prefer",
			wantErr: true,
		},
		{
			name:    "production rejects encryption without server authentication",
			env:     "production",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=require",
			wantErr: true,
		},
		{
			name:    "staging rejects ca validation without hostname validation",
			env:     "staging",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-ca",
			wantErr: true,
		},
		{
			name:    "production rejects duplicate modes",
			env:     "production",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-full&sslmode=verify-full",
			wantErr: true,
		},
		{
			name:    "production rejects ambiguous mode",
			env:     "production",
			dsn:     "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-full%2Crequire",
			wantErr: true,
		},
		{
			name: "production accepts verify full",
			env:  "production",
			dsn:  "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-full",
		},
		{
			name: "staging accepts verify full",
			env:  "staging",
			dsn:  "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-full",
		},
		{
			name: "local accepts disabled TLS for disposable database",
			env:  "local",
			dsn:  "postgres://runtime:placeholder@127.0.0.1/cumuru?sslmode=disable",
		},
		{
			name: "test accepts omitted TLS mode for disposable database",
			env:  "test",
			dsn:  "postgres://runtime:placeholder@127.0.0.1/cumuru",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertDatabaseTLS(t, tt.env, tt.dsn, tt.wantErr)
		})
	}
}

func assertDatabaseTLS(t *testing.T, environment, dsn string, wantErr bool) {
	t.Helper()
	values := merge(validProduction(), map[string]string{
		"APP_ENV":      environment,
		"DATABASE_URL": dsn,
	})
	_, err := config.Load(config.ProcessAPI, lookup(values))
	if !wantErr {
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Load() error = %v, want sanitized DATABASE_URL error", err)
	}
	if strings.Contains(err.Error(), dsn) || strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("Load() leaked DSN: %v", err)
	}
}

func validLocal() map[string]string {
	return merge(map[string]string{
		"APP_ENV":             "local",
		"HTTP_ADDRESS":        "127.0.0.1:8080",
		"OPERATIONS_ADDRESS":  "127.0.0.1:9090",
		"DATABASE_URL":        "postgres://local:local@127.0.0.1:5432/cumuru?sslmode=disable",
		"OIDC_MODE":           "fake",
		"OIDC_ISSUER":         "https://oidc.invalid/local",
		"OIDC_AUDIENCE":       "cumuru-local",
		"OTEL_EXPORTER":       "none",
		"TRUSTED_PROXY_CIDRS": "127.0.0.1/32,::1/128",
	}, validPhase2())
}

func validProduction() map[string]string {
	return merge(map[string]string{
		"APP_ENV":             "production",
		"HTTP_ADDRESS":        "0.0.0.0:8080",
		"OPERATIONS_ADDRESS":  "0.0.0.0:9090",
		"DATABASE_URL":        "postgres://runtime:placeholder@database.invalid/cumuru?sslmode=verify-full",
		"OIDC_MODE":           "real",
		"OIDC_ISSUER":         "https://identity.invalid",
		"OIDC_AUDIENCE":       "cumuru",
		"OTEL_EXPORTER":       "otlp",
		"OTEL_ENDPOINT":       "https://telemetry.invalid/v1/traces",
		"TRUSTED_PROXY_CIDRS": "10.20.30.40/32",
	}, merge(validPhase2(), map[string]string{
		"INVITE_BASE_URL":      "https://registro.invalid/convites",
		"CORS_ALLOWED_ORIGINS": "https://registro.invalid",
	}))
}

func validPhase2() map[string]string {
	return map[string]string{
		"INVITE_BASE_URL":                  "http://127.0.0.1:5173/convites",
		"INVITE_TTL":                       "72h",
		"IDEMPOTENCY_TTL":                  "720h",
		"RATE_LIMIT_WINDOW":                "1m",
		"INVITE_CONTEXT_RATE_LIMIT":        "30",
		"INVITE_SUBMIT_RATE_LIMIT":         "10",
		"CORS_ALLOWED_ORIGINS":             "http://127.0.0.1:5173",
		"INVITE_HMAC_CURRENT_VERSION":      "invite-v2",
		"INVITE_HMAC_KEYS":                 "invite-v1=YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=,invite-v2=MTIzNDU2Nzg5MGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eA==",
		"ACTOR_HMAC_CURRENT_VERSION":       "actor-v1",
		"ACTOR_HMAC_KEYS":                  "actor-v1=YWN0b3Ita2V5LWlzLWF0LWxlYXN0LTMyLWJ5dGVzLWxvbmc=",
		"IDEMPOTENCY_HMAC_CURRENT_VERSION": "idem-v1",
		"IDEMPOTENCY_HMAC_KEYS":            "idem-v1=aWRlbXBvdGVuY3kta2V5LWlzLWF0LWxlYXN0LTMyLWJ5dGVz",
		"RATE_LIMIT_HMAC_CURRENT_VERSION":  "rate-v1",
		"RATE_LIMIT_HMAC_KEYS":             "rate-v1=cmF0ZS1saW1pdC1rZXktaXMtYXQtbGVhc3QtMzItYnl0ZXM=",
		"CURSOR_HMAC_CURRENT_VERSION":      "cursor-v1",
		"CURSOR_HMAC_KEYS":                 "cursor-v1=Y3Vyc29yLWtleS1pcy1hdC1sZWFzdC0zMi1ieXRlcy1sb25n",
		"DOCUMENT_HMAC_CURRENT_VERSION":    "document-v1",
		"DOCUMENT_HMAC_KEYS":               "document-v1=ZG9jdW1lbnQta2V5LWlzLWF0LWxlYXN0LTMyLWJ5dGVzLW9r",
	}
}

func merge(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func lookup(values map[string]string) config.LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
