package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func phase7Values() map[string]string {
	values := validPhase2Values()
	values["APP_ENV"] = "test"
	values["PHASE7_ENABLED"] = "true"
	values["PROOF_OF_WORK_HMAC_CURRENT_VERSION"] = "pow-v1"
	values["PROOF_OF_WORK_HMAC_KEYS"] = encodedKey("pow-v1", "proof-of-work")
	return values
}

func loadTestPhase7(t *testing.T, values map[string]string) (Phase7Config, error) {
	t.Helper()
	phase2, err := loadPhase2(lookupMap(values))
	if err != nil {
		return Phase7Config{}, err
	}
	config, err := loadPhase7(Environment(values["APP_ENV"]), phase2, lookupMap(values))
	if err != nil {
		return Phase7Config{}, err
	}
	return config, config.validate()
}

func TestLoadPhase7AcceptsAnIndependentPrototypeKeyring(t *testing.T) {
	t.Parallel()

	got, err := loadTestPhase7(t, phase7Values())
	if err != nil {
		t.Fatalf("loadPhase7() error = %v", err)
	}
	if !got.Enabled || got.DifficultyBase != 12 || got.DifficultyCeiling != 18 {
		t.Fatalf("loadPhase7() = %#v", got)
	}
}

// Both fragment surfaces live on the origin the invite already uses, so the
// phase adds no host to keep in step with the nginx configuration.
func TestPhase7DerivesTheFragmentSurfacesFromTheInviteOrigin(t *testing.T) {
	t.Parallel()

	got, err := loadTestPhase7(t, phase7Values())
	if err != nil {
		t.Fatalf("loadPhase7() error = %v", err)
	}
	if url := got.SelfRegistrationURL.String(); url != "http://127.0.0.1:5173/i" {
		t.Fatalf("SelfRegistrationURL = %q", url)
	}
	if url := got.ActivationURL.String(); url != "http://127.0.0.1:5173/ativacao" {
		t.Fatalf("ActivationURL = %q", url)
	}
}

// Fail closed at startup: a challenge signed with an absent or short key is a
// MAC anybody can forge, and the whole control would be theatre.
func TestLoadPhase7RejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		change  func(map[string]string)
		wantErr string
	}{
		"production enablement": {
			func(values map[string]string) { values["APP_ENV"] = "production" },
			"PHASE7_ENABLED",
		},
		"absent keyring": {
			func(values map[string]string) {
				delete(values, "PROOF_OF_WORK_HMAC_KEYS")
				delete(values, "PROOF_OF_WORK_HMAC_CURRENT_VERSION")
			},
			"PROOF_OF_WORK_HMAC_CURRENT_VERSION",
		},
		"short key": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_HMAC_KEYS"] = "pow-v1=" +
					base64.StdEncoding.EncodeToString([]byte("too-short"))
			},
			"PROOF_OF_WORK_HMAC_KEYS",
		},
		"current version absent from the ring": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_HMAC_CURRENT_VERSION"] = "pow-v9"
			},
			"PROOF_OF_WORK_HMAC_CURRENT_VERSION",
		},
		"key shared with the invite ring": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_HMAC_KEYS"] = "pow-v1=" +
					strings.SplitN(values["INVITE_HMAC_KEYS"], "=", 2)[1]
			},
			"PROOF_OF_WORK_HMAC_KEYS",
		},
		"lifetime beyond an hour": {
			func(values map[string]string) { values["PROOF_OF_WORK_TTL"] = "2h" },
			"PROOF_OF_WORK_TTL",
		},
		"zero difficulty base": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_DIFFICULTY_BASE"] = "0"
			},
			"PROOF_OF_WORK_DIFFICULTY_BASE",
		},
		"ceiling below base": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_DIFFICULTY_CEILING"] = "8"
			},
			"PROOF_OF_WORK_DIFFICULTY_CEILING",
		},
		"ceiling beyond the contract": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_DIFFICULTY_CEILING"] = "33"
			},
			"PROOF_OF_WORK_DIFFICULTY_CEILING",
		},
		"zero step": {
			func(values map[string]string) {
				values["PROOF_OF_WORK_REQUESTS_PER_BIT"] = "0"
			},
			"PROOF_OF_WORK_REQUESTS_PER_BIT",
		},
		"zero submit limit": {
			func(values map[string]string) {
				values["SELF_SERVICE_SUBMIT_RATE_LIMIT"] = "0"
			},
			"SELF_SERVICE_SUBMIT_RATE_LIMIT",
		},
		"zero sweep batch": {
			func(values map[string]string) {
				values["APPROVAL_EXPIRY_BATCH_SIZE"] = "0"
			},
			"APPROVAL_EXPIRY_BATCH_SIZE",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			values := phase7Values()
			test.change(values)
			_, err := loadTestPhase7(t, values)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadPhase7() error = %v, want %s", err, test.wantErr)
			}
		})
	}
}

// Disabled is the default, and a disabled phase must not require any key.
func TestPhase7IsOffByDefaultAndNeedsNoKey(t *testing.T) {
	t.Parallel()

	values := validPhase2Values()
	values["APP_ENV"] = "test"
	got, err := loadTestPhase7(t, values)
	if err != nil {
		t.Fatalf("loadPhase7() error = %v", err)
	}
	if got.Enabled {
		t.Fatal("Phase7 is enabled without being asked for")
	}
}
