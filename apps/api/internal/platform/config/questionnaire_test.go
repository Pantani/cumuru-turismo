package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadQuestionnaireAcceptsIndependentPrototypeKeys(t *testing.T) {
	t.Parallel()

	values := questionnaireValues()
	core, err := loadCore(lookupMap(values))
	if err != nil {
		t.Fatalf("loadCore() error = %v", err)
	}
	got, err := loadQuestionnaire(EnvironmentTest, core, lookupMap(values))
	if err != nil {
		t.Fatalf("loadQuestionnaire() error = %v", err)
	}
	if !got.Enabled || got.PrimaryStableKey != "tourism_profile" {
		t.Fatalf("loadQuestionnaire() = %#v", got)
	}
}

func TestLoadQuestionnaireRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		change  func(map[string]string)
		wantErr string
	}{
		{
			name: "production enablement",
			change: func(values map[string]string) {
				values["APP_ENV"] = "production"
			},
			wantErr: "QUESTIONNAIRE_ENABLED",
		},
		{
			name: "missing cleanup",
			change: func(values map[string]string) {
				values["SURVEY_FREE_TEXT_CLEANUP_ENABLED"] = "false"
			},
			wantErr: "SURVEY_FREE_TEXT_CLEANUP_ENABLED",
		},
		{
			name: "ttl over 24 hours",
			change: func(values map[string]string) {
				values["SURVEY_FREE_TEXT_TTL"] = "25h"
			},
			wantErr: "SURVEY_FREE_TEXT_TTL",
		},
		{
			name: "overlap with invite keyring",
			change: func(values map[string]string) {
				values["SURVEY_HMAC_CURRENT_VERSION"] =
					values["INVITE_HMAC_CURRENT_VERSION"]
				values["SURVEY_HMAC_KEYS"] = values["INVITE_HMAC_KEYS"]
			},
			wantErr: "QUESTIONNAIRE_KEYRINGS",
		},
		{
			// The document keyring blinds a CPF under ADR-038. It was absent
			// from the core list the overlap check walks, so a survey key
			// could silently reuse it.
			name: "overlap with document keyring",
			change: func(values map[string]string) {
				values["SURVEY_HMAC_CURRENT_VERSION"] =
					values["DOCUMENT_HMAC_CURRENT_VERSION"]
				values["SURVEY_HMAC_KEYS"] = values["DOCUMENT_HMAC_KEYS"]
			},
			wantErr: "QUESTIONNAIRE_KEYRINGS",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertUnsafeQuestionnaireConfig(t, test.change, test.wantErr)
		})
	}
}

func assertUnsafeQuestionnaireConfig(
	t *testing.T,
	change func(map[string]string),
	wantErr string,
) {
	t.Helper()
	values := questionnaireValues()
	change(values)
	core, err := loadCore(lookupMap(values))
	if err != nil {
		t.Fatalf("loadCore() error = %v", err)
	}
	environment := Environment(values["APP_ENV"])
	got, err := loadQuestionnaire(environment, core, lookupMap(values))
	if err == nil {
		err = got.validate()
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("loadQuestionnaire() error = %v, want %s", err, wantErr)
	}
}

func questionnaireValues() map[string]string {
	values := validCoreValues()
	values["APP_ENV"] = "test"
	values["QUESTIONNAIRE_ENABLED"] = "true"
	values["SURVEY_CAPABILITY_TTL"] = "24h"
	values["SURVEY_FREE_TEXT_TTL"] = "24h"
	values["SURVEY_SUBMIT_RATE_LIMIT"] = "10"
	values["SURVEY_FREE_TEXT_CLEANUP_ENABLED"] = "true"
	values["SURVEY_HMAC_CURRENT_VERSION"] = "survey-v1"
	values["SURVEY_HMAC_KEYS"] = encodedKey("survey-v1", "survey-hmac")
	values["SURVEY_FREE_TEXT_CURRENT_VERSION"] = "free-text-v1"
	values["SURVEY_FREE_TEXT_KEYS"] = encodedKey("free-text-v1", "free-text")
	return values
}

func validCoreValues() map[string]string {
	return map[string]string{
		"INVITE_BASE_URL":                  "http://127.0.0.1:5173/convites",
		"INVITE_TTL":                       "72h",
		"IDEMPOTENCY_TTL":                  "720h",
		"RATE_LIMIT_WINDOW":                "1m",
		"INVITE_CONTEXT_RATE_LIMIT":        "30",
		"INVITE_SUBMIT_RATE_LIMIT":         "10",
		"CORS_ALLOWED_ORIGINS":             "http://127.0.0.1:5173",
		"INVITE_HMAC_CURRENT_VERSION":      "invite-v1",
		"INVITE_HMAC_KEYS":                 encodedKey("invite-v1", "invite"),
		"ACTOR_HMAC_CURRENT_VERSION":       "actor-v1",
		"ACTOR_HMAC_KEYS":                  encodedKey("actor-v1", "actor"),
		"IDEMPOTENCY_HMAC_CURRENT_VERSION": "idem-v1",
		"IDEMPOTENCY_HMAC_KEYS":            encodedKey("idem-v1", "idem"),
		"RATE_LIMIT_HMAC_CURRENT_VERSION":  "rate-v1",
		"RATE_LIMIT_HMAC_KEYS":             encodedKey("rate-v1", "rate"),
		"CURSOR_HMAC_CURRENT_VERSION":      "cursor-v1",
		"CURSOR_HMAC_KEYS":                 encodedKey("cursor-v1", "cursor"),
		"DOCUMENT_HMAC_CURRENT_VERSION":    "document-v1",
		"DOCUMENT_HMAC_KEYS":               encodedKey("document-v1", "document"),
	}
}

func encodedKey(version, seed string) string {
	key := make([]byte, 32)
	copy(key, []byte(seed))
	for index := len(seed); index < len(key); index++ {
		key[index] = byte(index + 1)
	}
	return version + "=" + base64.StdEncoding.EncodeToString(key)
}

func lookupMap(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
