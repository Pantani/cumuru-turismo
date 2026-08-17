package proofofwork_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/proofofwork"
	"github.com/google/uuid"
)

const ttl = 10 * time.Minute

var (
	issuedAt = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	poster   = uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	other    = uuid.MustParse("019f0000-0000-7000-8000-000000000002")
)

func newIssuer(t *testing.T) *proofofwork.Issuer {
	t.Helper()
	issuer, err := proofofwork.NewIssuer(proofofwork.Keyring{
		CurrentVersion: "pow-v2",
		Keys: map[string][]byte{
			"pow-v1": []byte("first-proof-of-work-key-is-at-least-32-bytes"),
			"pow-v2": []byte("second-proof-of-work-key-is-at-least-32-bytes"),
		},
	}, ttl)
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	return issuer
}

// solve mirrors the browser: base64url of an 8-byte big-endian counter, digest
// over the concatenated UTF-8 of challenge and solution.
func solve(t *testing.T, challenge string, bits uint8) string {
	t.Helper()
	for counter := uint64(0); counter < 1<<22; counter++ {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, counter)
		solution := base64.RawURLEncoding.EncodeToString(encoded)
		digest := sha256.Sum256([]byte(challenge + solution))
		if leadingZeroBits(digest[:]) >= int(bits) {
			return solution
		}
	}
	t.Fatalf("no solution found for %d bits", bits)
	return ""
}

func leadingZeroBits(digest []byte) int {
	bits := 0
	for _, value := range digest {
		for mask := byte(0x80); mask != 0; mask >>= 1 {
			if value&mask != 0 {
				return bits
			}
			bits++
		}
	}
	return bits
}

func TestSolvedChallengeIsAcceptedAndProducesASpend(t *testing.T) {
	t.Parallel()

	issuer := newIssuer(t)
	challenge, err := issuer.Issue(poster, 8, issuedAt)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if challenge.Algorithm != proofofwork.Algorithm || challenge.DifficultyBits != 8 {
		t.Fatalf("Issue() = %#v", challenge)
	}
	if !challenge.ExpiresAt.Equal(issuedAt.Add(ttl)) {
		t.Fatalf("ExpiresAt = %s", challenge.ExpiresAt)
	}
	solution := solve(t, challenge.Value, 8)
	spend, err := issuer.Verify(challenge.Value, solution, poster, issuedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if len(spend.ChallengeHMAC) != 32 || spend.KeyVersion != "pow-v2" {
		t.Fatalf("Verify() spend = %#v", spend)
	}
}

// N-15: nothing about the challenge may be negotiable by the client.
func TestVerifyRefusesEveryTamperedChallenge(t *testing.T) {
	t.Parallel()

	issuer := newIssuer(t)
	challenge, _ := issuer.Issue(poster, 8, issuedAt)
	solution := solve(t, challenge.Value, 8)

	tests := map[string]struct {
		challenge string
		solution  string
		invite    uuid.UUID
		now       time.Time
	}{
		"absent challenge": {"", solution, poster, issuedAt},
		"absent solution":  {challenge.Value, "", poster, issuedAt},
		"another accommodation": {
			challenge.Value, solution, other, issuedAt,
		},
		"tampered mac": {
			flipLastCharacter(challenge.Value), solution, poster, issuedAt,
		},
		"unsolved":     {challenge.Value, "AAAAAAAAAAA", poster, issuedAt},
		"expired":      {challenge.Value, solution, poster, issuedAt.Add(ttl)},
		"issued later": {challenge.Value, solution, poster, issuedAt.Add(-time.Second)},
		"oversized solution": {
			challenge.Value, string(make([]byte, 65)), poster, issuedAt,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := issuer.Verify(test.challenge, test.solution, test.invite, test.now)
			if !errors.Is(err, proofofwork.ErrRejected) {
				t.Fatalf("Verify() error = %v, want ErrRejected", err)
			}
		})
	}
}

// The difficulty travels inside the MAC. A client that re-encodes the challenge
// with fewer bits breaks the MAC, so it cannot buy itself a cheaper toll.
func TestDifficultyCannotBeLoweredByTheClient(t *testing.T) {
	t.Parallel()

	issuer := newIssuer(t)
	expensive, _ := issuer.Issue(poster, 12, issuedAt)
	cheapSolution := solveBetween(t, expensive.Value, 4, 12)

	// The solution satisfies four bits and not twelve, so it is refused under
	// the difficulty the challenge actually carries.
	if _, err := issuer.Verify(
		expensive.Value, cheapSolution, poster, issuedAt,
	); !errors.Is(err, proofofwork.ErrRejected) {
		t.Fatalf("Verify() error = %v, want ErrRejected", err)
	}
	if _, err := issuer.Verify(
		expensive.Value, solve(t, expensive.Value, 12), poster, issuedAt,
	); err != nil {
		t.Fatalf("Verify() error = %v; the twelve-bit solution must pass", err)
	}
}

// solveBetween pins the case deterministically: a solution that clears the
// lower bar and provably fails the higher one. Solving for the lower bar alone
// would pass the higher one once every 2^(high-low) runs.
func solveBetween(t *testing.T, challenge string, low, high uint8) string {
	t.Helper()
	for counter := uint64(0); counter < 1<<22; counter++ {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, counter)
		solution := base64.RawURLEncoding.EncodeToString(encoded)
		digest := sha256.Sum256([]byte(challenge + solution))
		bits := leadingZeroBits(digest[:])
		if bits >= int(low) && bits < int(high) {
			return solution
		}
	}
	t.Fatalf("no solution found between %d and %d bits", low, high)
	return ""
}

// N-17: the spend digest is what the caller inserts into the nonce book, and it
// must be stable for the same challenge — the primary key conflict is the
// replay, and a digest that changed per call would make the book useless.
func TestSpendDigestIsStableAndDistinctPerChallenge(t *testing.T) {
	t.Parallel()

	issuer := newIssuer(t)
	first, _ := issuer.Issue(poster, 6, issuedAt)
	second, _ := issuer.Issue(poster, 6, issuedAt)
	if first.Value == second.Value {
		t.Fatal("two challenges reused the same nonce")
	}
	firstSolution := solve(t, first.Value, 6)
	spend, err := issuer.Verify(first.Value, firstSolution, poster, issuedAt)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	replay, err := issuer.Verify(first.Value, firstSolution, poster, issuedAt)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if string(spend.ChallengeHMAC) != string(replay.ChallengeHMAC) {
		t.Fatal("the same challenge produced two spend digests")
	}
	secondSolution := solve(t, second.Value, 6)
	otherSpend, _ := issuer.Verify(second.Value, secondSolution, poster, issuedAt)
	if string(spend.ChallengeHMAC) == string(otherSpend.ChallengeHMAC) {
		t.Fatal("two challenges produced the same spend digest")
	}
}

// A challenge signed under a retired key still verifies inside its TTL, so a
// rotation does not refuse the guest who opened the form a minute earlier.
func TestVerifyWalksTheWholeKeyring(t *testing.T) {
	t.Parallel()

	old, err := proofofwork.NewIssuer(proofofwork.Keyring{
		CurrentVersion: "pow-v1",
		Keys: map[string][]byte{
			"pow-v1": []byte("first-proof-of-work-key-is-at-least-32-bytes"),
		},
	}, ttl)
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	challenge, _ := old.Issue(poster, 6, issuedAt)
	solution := solve(t, challenge.Value, 6)

	spend, err := newIssuer(t).Verify(challenge.Value, solution, poster, issuedAt)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if spend.KeyVersion != "pow-v1" {
		t.Fatalf("KeyVersion = %q, want the version that signed it", spend.KeyVersion)
	}
}

func TestIssuerRejectsAnUnusableConfiguration(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		keyring proofofwork.Keyring
		ttl     time.Duration
	}{
		"missing current version": {
			proofofwork.Keyring{CurrentVersion: "absent", Keys: map[string][]byte{
				"pow-v1": []byte("first-proof-of-work-key-is-at-least-32-bytes"),
			}}, ttl,
		},
		"short key": {
			proofofwork.Keyring{CurrentVersion: "pow-v1", Keys: map[string][]byte{
				"pow-v1": []byte("too-short"),
			}}, ttl,
		},
		"no lifetime": {
			proofofwork.Keyring{CurrentVersion: "pow-v1", Keys: map[string][]byte{
				"pow-v1": []byte("first-proof-of-work-key-is-at-least-32-bytes"),
			}}, 0,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := proofofwork.NewIssuer(test.keyring, test.ttl); err == nil {
				t.Fatal("NewIssuer() accepted an unusable configuration")
			}
		})
	}
}

func TestIssueRejectsAnOutOfContractDifficulty(t *testing.T) {
	t.Parallel()

	issuer := newIssuer(t)
	for _, bits := range []uint8{0, proofofwork.MaxDifficultyBits + 1} {
		if _, err := issuer.Issue(poster, bits, issuedAt); err == nil {
			t.Fatalf("Issue(%d bits) accepted a difficulty outside the contract", bits)
		}
	}
	if _, err := issuer.Issue(uuid.Nil, 8, issuedAt); err == nil {
		t.Fatal("Issue() accepted a challenge bound to no poster")
	}
}

func flipLastCharacter(value string) string {
	last := value[len(value)-1]
	if last == 'A' {
		return value[:len(value)-1] + "B"
	}
	return value[:len(value)-1] + "A"
}
