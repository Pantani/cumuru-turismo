package config_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

func TestLoadPhase2RuntimeConfiguration(t *testing.T) {
	t.Parallel()

	got, err := config.Load(config.ProcessAPI, lookup(validLocal()))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertEqual(t, "InviteTTL", got.Phase2.InviteTTL, 72*time.Hour)
	assertEqual(t, "IdempotencyTTL", got.Phase2.IdempotencyTTL, 30*24*time.Hour)
	assertEqual(t, "InviteContextRateLimit", got.Phase2.InviteContextRateLimit, 30)
	assertEqual(t, "InviteSubmitRateLimit", got.Phase2.InviteSubmitRateLimit, 10)
	assertEqual(t, "InviteKeys.CurrentVersion", got.Phase2.InviteKeys.CurrentVersion, "invite-v2")
	assertEqual(t, "InviteKeys.Keys", len(got.Phase2.InviteKeys.Keys), 2)
	assertEqual(t, "CursorKeys.CurrentVersion", got.Phase2.CursorKeys.CurrentVersion, "cursor-v1")
	assertEqual(t, "CursorKeys.Keys", len(got.Phase2.CursorKeys.Keys), 1)
	assertEqual(
		t,
		"AccommodationOnboardingEnabled",
		got.Phase2.AccommodationOnboardingEnabled,
		false,
	)
	assertEqual(
		t,
		"InviteBaseURL",
		got.Phase2.InviteBaseURL.String(),
		"http://127.0.0.1:5173/convites",
	)
}

func TestLoadAccommodationOnboardingFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		want    bool
		wantErr string
	}{
		{
			name: "local fake explicitly enabled",
			values: merge(validLocal(), map[string]string{
				"ACCOMMODATION_ONBOARDING_ENABLED": "true",
			}),
			want: true,
		},
		{
			name: "test fake explicitly enabled",
			values: merge(validLocal(), map[string]string{
				"APP_ENV":                          "test",
				"ACCOMMODATION_ONBOARDING_ENABLED": "true",
			}),
			want: true,
		},
		{
			name: "local real rejected",
			values: merge(validLocal(), map[string]string{
				"OIDC_MODE":                        "real",
				"ACCOMMODATION_ONBOARDING_ENABLED": "true",
			}),
			wantErr: "ACCOMMODATION_ONBOARDING_ENABLED",
		},
		{
			name: "production real rejected",
			values: merge(validProduction(), map[string]string{
				"ACCOMMODATION_ONBOARDING_ENABLED": "true",
			}),
			wantErr: "ACCOMMODATION_ONBOARDING_ENABLED",
		},
		{
			name: "invalid boolean rejected",
			values: merge(validLocal(), map[string]string{
				"ACCOMMODATION_ONBOARDING_ENABLED": "sometimes",
			}),
			wantErr: "ACCOMMODATION_ONBOARDING_ENABLED",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertAccommodationOnboardingResult(t, tt.values, tt.want, tt.wantErr)
		})
	}
}

func assertAccommodationOnboardingResult(
	t *testing.T,
	values map[string]string,
	want bool,
	wantErr string,
) {
	t.Helper()
	got, err := config.Load(config.ProcessAPI, lookup(values))
	if wantErr != "" {
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("Load() error = %v, want field %s", err, wantErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Phase2.AccommodationOnboardingEnabled != want {
		t.Fatalf(
			"AccommodationOnboardingEnabled = %t, want %t",
			got.Phase2.AccommodationOnboardingEnabled,
			want,
		)
	}
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestLoadPhase2RejectsUnsafeConfigurationWithoutLeakingSecrets(t *testing.T) {
	t.Parallel()

	const sentinel = "secret-sentinel-must-not-leak"
	encodedSentinel := "c2VjcmV0LXNlbnRpbmVsLW11c3Qtbm90LWxlYWs="
	encodedInviteKey := inviteFixtureEncodedKey(t)
	tests := []struct {
		name   string
		values map[string]string
		field  string
	}{
		{
			name: "missing current invite key",
			values: merge(validLocal(), map[string]string{
				"INVITE_HMAC_CURRENT_VERSION": "missing",
			}),
			field: "INVITE_HMAC_CURRENT_VERSION",
		},
		{
			name: "short actor key",
			values: merge(validLocal(), map[string]string{
				"ACTOR_HMAC_KEYS": "actor-v1=" + encodedSentinel,
			}),
			field: "ACTOR_HMAC_KEYS",
		},
		{
			name: "same key across purposes",
			values: merge(validLocal(), map[string]string{
				"ACTOR_HMAC_KEYS": "actor-v1=" + encodedInviteKey,
			}),
			field: "HMAC_KEYRINGS",
		},
		{
			name: "missing cursor keyring",
			values: merge(validLocal(), map[string]string{
				"CURSOR_HMAC_CURRENT_VERSION": "",
			}),
			field: "CURSOR_HMAC_CURRENT_VERSION",
		},
		{
			name: "cursor key reused by invite",
			values: merge(validLocal(), map[string]string{
				"CURSOR_HMAC_KEYS": "cursor-v1=" + encodedInviteKey,
			}),
			field: "HMAC_KEYRINGS",
		},
		{
			name: "wildcard CORS",
			values: merge(validLocal(), map[string]string{
				"CORS_ALLOWED_ORIGINS": "*",
			}),
			field: "CORS_ALLOWED_ORIGINS",
		},
		{
			name: "production invite URL requires HTTPS",
			values: merge(validProduction(), map[string]string{
				"INVITE_BASE_URL": "http://registro.invalid/convites",
			}),
			field: "INVITE_BASE_URL",
		},
		{
			name: "invalid replay TTL",
			values: merge(validLocal(), map[string]string{
				"IDEMPOTENCY_TTL": "71h",
			}),
			field: "IDEMPOTENCY_TTL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertPhase2ConfigRejected(t, tt.values, tt.field, sentinel)
		})
	}
}

func inviteFixtureEncodedKey(t *testing.T) string {
	t.Helper()
	loaded, err := config.Load(config.ProcessAPI, lookup(validLocal()))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	key, ok := loaded.Phase2.InviteKeys.Key("invite-v2")
	if !ok {
		t.Fatal("invite-v2 key is missing")
	}
	return base64.StdEncoding.EncodeToString(key)
}

func assertPhase2ConfigRejected(t *testing.T, values map[string]string, field, sentinel string) {
	t.Helper()
	_, err := config.Load(config.ProcessAPI, lookup(values))
	if err == nil || !strings.Contains(err.Error(), field) {
		t.Fatalf("Load() error = %v, want field %s", err, field)
	}
	for _, value := range values {
		if (value != "" && strings.Contains(err.Error(), value)) ||
			strings.Contains(err.Error(), sentinel) {
			t.Fatalf("Load() leaked configuration value: %v", err)
		}
	}
}

func TestPhase2KeyringsReturnDefensiveCopies(t *testing.T) {
	t.Parallel()

	got, err := config.Load(config.ProcessAPI, lookup(validLocal()))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	first, ok := got.Phase2.InviteKeys.Key("invite-v2")
	if !ok {
		t.Fatal("invite-v2 key is missing")
	}
	second, _ := got.Phase2.InviteKeys.Key("invite-v2")
	first[0] ^= 0xff
	if bytes.Equal(first, second) {
		t.Fatal("Key() returned shared secret storage")
	}
}
