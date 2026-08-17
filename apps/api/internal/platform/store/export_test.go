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
	rollbackProvisioning(tx, time.Second)
	if tx.contextErr != nil {
		t.Fatalf("rollback context error = %v, want fresh context", tx.contextErr)
	}
}

// RateLimitDigestForTest exposes the bucket key so an integration test can
// identify the exact rows a poster produced. Counting a whole scope instead
// passes on rows nobody in the test created — which is precisely how the first
// version of the N-23 assertion came to be unable to fail.
func RateLimitDigestForTest(key []byte, scope, token, subject string) []byte {
	return rateLimitDigest(key, scope, token, subject)
}
