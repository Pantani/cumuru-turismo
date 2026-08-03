package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func ActorDigestForTest(key []byte, issuer, subject string) []byte {
	return keyedDigest(key, "actor", actorValue(issuer, subject))
}

type rollbackTxStub struct {
	pgx.Tx
	contextErr error
}

func (s *rollbackTxStub) Rollback(ctx context.Context) error {
	s.contextErr = ctx.Err()
	return nil
}

func TestRollbackLocalDemoUsesFreshContext(t *testing.T) {
	t.Parallel()

	tx := &rollbackTxStub{}
	rollbackLocalDemo(tx, time.Second)
	if tx.contextErr != nil {
		t.Fatalf("rollback context error = %v, want fresh context", tx.contextErr)
	}
}

func TestClassifyLocalDemoOrganizationReadErrorBeforeNameMismatch(t *testing.T) {
	t.Parallel()

	databaseErr := errors.New("database unavailable")
	err := classifyLocalOrganizationResult("", databaseErr, "expected")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("classification = %v, want ErrUnavailable", err)
	}
	if errors.Is(err, ErrLocalDemoConflict) {
		t.Fatalf("classification = %v, database failure must not be a conflict", err)
	}
}

func TestClassifyLocalDemoOrganizationMissingOrMismatched(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		got  string
		err  error
	}{
		{name: "missing", err: pgx.ErrNoRows},
		{name: "mismatched", got: "other"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := classifyLocalOrganizationResult(test.got, test.err, "expected"); !errors.Is(err, ErrLocalDemoConflict) {
				t.Fatalf("classification = %v, want ErrLocalDemoConflict", err)
			}
		})
	}
}
