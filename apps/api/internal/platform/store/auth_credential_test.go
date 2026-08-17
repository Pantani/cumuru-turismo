package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type credentialQueries struct {
	generated.Querier
	failures int
	cleared  int
}

func (f *credentialQueries) RegisterAuthFailure(
	_ context.Context,
	_ generated.RegisterAuthFailureParams,
) error {
	f.failures++
	return nil
}

func (f *credentialQueries) ClearAuthFailures(_ context.Context, _ pgtype.UUID) error {
	f.cleared++
	return nil
}

var credentialSecret = strings.Repeat("fixture-", 3)

// A pending_activation account has no hash. Answering it without a verification
// would let a probe separate "pending" from "wrong password" by timing, which
// is the account-state oracle N-41 forbids.
func TestAccountWithoutHashFallsBackToAnUnmatchableCredential(t *testing.T) {
	t.Parallel()

	pending := authAccount{status: "pending_activation"}
	if pending.credential() != access.UnmatchableHash() {
		t.Fatal("account without a hash did not fall back to the unmatchable encoding")
	}
}

func TestAccountWithHashUsesTheStoredCredential(t *testing.T) {
	t.Parallel()

	encoded, err := access.NewPasswordHasher().Hash(credentialSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	active := authAccount{status: accountStatusActive, passwordHash: &encoded}
	if active.credential() != encoded {
		t.Fatal("stored hash was not used")
	}
}

// Both refusals must walk the same path: one verification, one failure write,
// one ErrAuthRejected. Any early return would answer sooner for the account
// state that took it.
func TestPasswordAttemptPaysTheSameCostForEveryRefusal(t *testing.T) {
	t.Parallel()

	encoded, err := access.NewPasswordHasher().Hash(credentialSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	tests := map[string]struct {
		account  authAccount
		password string
	}{
		"pending account with the eventual password": {
			authAccount{status: "pending_activation"}, credentialSecret,
		},
		"disabled account with the right password": {
			authAccount{status: "disabled", passwordHash: &encoded}, credentialSecret,
		},
		"active account with the wrong password": {
			authAccount{status: accountStatusActive, passwordHash: &encoded},
			strings.Repeat("divergent-", 3),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			queries := &credentialQueries{}
			store := New(queries, 0)
			err := store.applyPasswordAttempt(
				context.Background(), queries, test.account, test.password,
			)
			if !errors.Is(err, ErrAuthRejected) {
				t.Fatalf("applyPasswordAttempt() error = %v, want ErrAuthRejected", err)
			}
			if queries.failures != 1 || queries.cleared != 0 {
				t.Fatalf(
					"failures = %d, cleared = %d; want a single registered failure",
					queries.failures, queries.cleared,
				)
			}
		})
	}
}

func TestPasswordAttemptAcceptsTheActiveCredential(t *testing.T) {
	t.Parallel()

	encoded, err := access.NewPasswordHasher().Hash(credentialSecret)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	queries := &credentialQueries{}
	store := New(queries, 0)
	account := authAccount{
		id:           uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		status:       accountStatusActive,
		passwordHash: &encoded,
	}
	if err := store.applyPasswordAttempt(
		context.Background(), queries, account, credentialSecret,
	); err != nil {
		t.Fatalf("applyPasswordAttempt() error = %v", err)
	}
	if queries.cleared != 1 || queries.failures != 0 {
		t.Fatalf("cleared = %d, failures = %d", queries.cleared, queries.failures)
	}
}
