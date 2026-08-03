package store

import (
	"context"
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
