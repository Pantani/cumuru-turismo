package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
)

type expiredCleanupQueriesStub struct {
	generated.Querier
	params generated.CleanupExpiredOperationalRecordsParams
	row    generated.CleanupExpiredOperationalRecordsRow
	err    error
}

func (s *expiredCleanupQueriesStub) CleanupExpiredOperationalRecords(
	_ context.Context,
	params generated.CleanupExpiredOperationalRecordsParams,
) (generated.CleanupExpiredOperationalRecordsRow, error) {
	s.params = params
	return s.row, s.err
}

func TestCleanupExpiredOperationalRecordsUsesCutoffAndBound(t *testing.T) {
	t.Parallel()
	queries := &expiredCleanupQueriesStub{
		row: generated.CleanupExpiredOperationalRecordsRow{
			IdempotencyRecords: 3,
			RateLimitBuckets:   5,
		},
	}
	subject := New(queries, time.Second)
	cutoff := time.Date(
		2026, 7, 28, 12, 0, 0, 0,
		time.FixedZone("test", -3*60*60),
	)

	result, err := subject.CleanupExpiredOperationalRecords(
		context.Background(),
		cutoff,
		100,
	)
	if err != nil {
		t.Fatalf("CleanupExpiredOperationalRecords() error = %v", err)
	}
	if result.IdempotencyRecords != 3 || result.RateLimitBuckets != 5 {
		t.Fatalf("result = %#v", result)
	}
	assertCleanupParams(t, queries.params, cutoff, 100)
}

func TestCleanupExpiredOperationalRecordsFailsClosed(t *testing.T) {
	t.Parallel()
	canary := errors.New("private-hmac-canary")
	queries := &expiredCleanupQueriesStub{err: canary}
	subject := New(queries, time.Second)

	result, err := subject.CleanupExpiredOperationalRecords(
		context.Background(),
		time.Now(),
		100,
	)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if result != (ExpiredRecordCleanupResult{}) {
		t.Fatalf("result = %#v", result)
	}
}

func TestCleanupExpiredOperationalRecordsRejectsUnboundedBatch(t *testing.T) {
	t.Parallel()
	queries := &expiredCleanupQueriesStub{}
	subject := New(queries, time.Second)

	_, err := subject.CleanupExpiredOperationalRecords(
		context.Background(),
		time.Now(),
		maxExpiredRecordCleanupBatch+1,
	)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if queries.params.BatchSize != 0 {
		t.Fatalf("cleanup query ran: %#v", queries.params)
	}
}

func assertCleanupParams(
	t *testing.T,
	params generated.CleanupExpiredOperationalRecordsParams,
	cutoff time.Time,
	batchSize int32,
) {
	t.Helper()
	if params.BatchSize != batchSize ||
		!params.ExpiredBefore.Valid ||
		!params.ExpiredBefore.Time.Equal(cutoff.UTC()) {
		t.Fatalf("idempotency params = %#v", params)
	}
}
