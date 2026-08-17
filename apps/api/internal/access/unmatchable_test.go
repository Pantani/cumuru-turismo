package access_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

// The refusal has to cost a full Argon2id verification. ErrInvalidPassword is
// what proves the derivation actually ran: a malformed encoding would be
// rejected by the decoder, in nanoseconds, and would leak the account state
// through timing.
func TestUnmatchableHashRefusesAtTheCostOfARealVerification(t *testing.T) {
	t.Parallel()

	encoded := access.UnmatchableHash()
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("UnmatchableHash() = %q, want an Argon2id PHC encoding", encoded)
	}
	err := access.VerifyPassword(encoded, strings.Repeat("fixture-", 3))
	if !errors.Is(err, access.ErrInvalidPassword) {
		t.Fatalf("VerifyPassword() error = %v, want ErrInvalidPassword", err)
	}
}

// The same parameters as a live credential: a cheaper cost would still be
// measurable from the outside.
func TestUnmatchableHashCarriesTheProductionCost(t *testing.T) {
	t.Parallel()

	reference, err := access.NewPasswordHasher().Hash(strings.Repeat("fixture-", 3))
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if parameters(reference) != parameters(access.UnmatchableHash()) {
		t.Fatalf(
			"UnmatchableHash() cost = %q, want %q",
			parameters(access.UnmatchableHash()), parameters(reference),
		)
	}
}

func parameters(encoded string) string {
	segments := strings.Split(encoded, "$")
	if len(segments) != 6 {
		return ""
	}
	return segments[3]
}
