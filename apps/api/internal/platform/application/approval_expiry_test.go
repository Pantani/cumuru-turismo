package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type expirerStub struct {
	batches []int
	calls   int
	err     error
}

func (s *expirerStub) ExpireApprovals(context.Context) (int, error) {
	if s.err != nil {
		s.calls++
		return 0, s.err
	}
	if s.calls >= len(s.batches) {
		s.calls++
		return 0, nil
	}
	expired := s.batches[s.calls]
	s.calls++
	return expired, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// The sweep drains until a batch comes back empty, so a backlog does not wait a
// whole minute per batch.
func TestApprovalSweepDrainsUntilEmpty(t *testing.T) {
	t.Parallel()

	expirer := &expirerStub{batches: []int{200, 40, 0}}
	metrics := newApprovalExpiryMetrics()
	runApprovalExpiry(context.Background(), expirer, discardLogger(), metrics)
	if expirer.calls != 3 {
		t.Fatalf("calls = %d, want three batches then a stop", expirer.calls)
	}
}

// One run must never monopolise the worker: the cycle is bounded even when
// every batch comes back full.
func TestApprovalSweepIsBounded(t *testing.T) {
	t.Parallel()

	expirer := &expirerStub{batches: []int{200, 200, 200, 200, 200}}
	runApprovalExpiry(
		context.Background(), expirer, discardLogger(), newApprovalExpiryMetrics(),
	)
	if expirer.calls != approvalExpiryMaxBatches {
		t.Fatalf("calls = %d, want %d", expirer.calls, approvalExpiryMaxBatches)
	}
}

// A failure stops the cycle instead of retrying in a tight loop, and it never
// logs an identifier.
func TestApprovalSweepStopsOnFailure(t *testing.T) {
	t.Parallel()

	expirer := &expirerStub{err: errors.New("database unavailable")}
	runApprovalExpiry(
		context.Background(), expirer, discardLogger(), newApprovalExpiryMetrics(),
	)
	if expirer.calls != 1 {
		t.Fatalf("calls = %d, want a single attempt", expirer.calls)
	}
}

func TestApprovalSweepHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	expirer := &expirerStub{batches: []int{200, 200, 200}}
	runApprovalExpiry(ctx, expirer, discardLogger(), newApprovalExpiryMetrics())
	if expirer.calls != 0 {
		t.Fatalf("calls = %d, want none after cancellation", expirer.calls)
	}
}
