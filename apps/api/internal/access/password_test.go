package access_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

// validSecret is assembled at run time so no credential-shaped literal lands in
// the source tree or in a secret scanner report.
var validSecret = strings.Repeat("fixture-", 3)

func TestHashProducesVerifiableArgon2idEncoding(t *testing.T) {
	t.Parallel()
	encoded, err := access.NewPasswordHasher().Hash(validSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("Hash() = %q, want an Argon2id PHC encoding", encoded)
	}
	if err := access.VerifyPassword(encoded, validSecret); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
}

func TestHashSaltsEveryCredential(t *testing.T) {
	t.Parallel()
	hasher := access.NewPasswordHasher()
	first, err := hasher.Hash(validSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := hasher.Hash(validSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if first == second {
		t.Fatal("Hash() reused a salt for the same secret")
	}
}

func TestVerifyPasswordRejects(t *testing.T) {
	t.Parallel()
	encoded, err := access.NewPasswordHasher().Hash(validSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	tests := map[string]struct {
		encoded  string
		password string
	}{
		"wrong secret":      {encoded, strings.Repeat("divergent-", 3)},
		"too short":         {encoded, "curta"},
		"empty encoding":    {"", validSecret},
		"foreign algorithm": {"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA", validSecret},
		"truncated segments": {
			"$argon2id$v=19$m=19456,t=2,p=1$c2FsdA", validSecret,
		},
		"zero cost": {
			"$argon2id$v=19$m=0,t=2,p=1$c2FsdA$aGFzaA", validSecret,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := access.VerifyPassword(test.encoded, test.password); err == nil {
				t.Fatal("VerifyPassword() accepted an invalid credential")
			}
		})
	}
}

func TestHashRejectsUnsupportedLength(t *testing.T) {
	t.Parallel()
	hasher := access.NewPasswordHasher()
	tests := map[string]string{
		"below minimum": strings.Repeat("a", access.MinPasswordLength-1),
		"above maximum": strings.Repeat("a", access.MaxPasswordLength+1),
	}
	for name, secret := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := hasher.Hash(secret); !errors.Is(err, access.ErrInvalidPassword) {
				t.Fatalf("Hash() error = %v, want ErrInvalidPassword", err)
			}
		})
	}
}
