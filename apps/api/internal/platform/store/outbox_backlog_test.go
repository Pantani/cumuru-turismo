package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type outboxBacklogQueriesStub struct {
	generated.Querier
	row generated.GetOutboxBacklogRow
	err error
}

func (s outboxBacklogQueriesStub) GetOutboxBacklog(
	context.Context,
) (generated.GetOutboxBacklogRow, error) {
	return s.row, s.err
}

func TestGetOutboxBacklogMapsAggregateWithoutIdentifiers(t *testing.T) {
	t.Parallel()

	oldest := time.Date(2026, 7, 29, 11, 58, 30, 0, time.UTC)
	subject := New(outboxBacklogQueriesStub{row: generated.GetOutboxBacklogRow{
		PendingEvents: 7,
		OldestPendingAt: pgtype.Timestamptz{
			Time: oldest, Valid: true,
		},
	}}, time.Second)

	got, err := subject.GetOutboxBacklog(context.Background())

	if err != nil {
		t.Fatalf("GetOutboxBacklog() error = %v", err)
	}
	if got.PendingEvents != 7 || !got.OldestPendingAt.Equal(oldest) {
		t.Fatalf("GetOutboxBacklog() = %#v", got)
	}
}

func TestGetOutboxBacklogAllowsEmptyQueue(t *testing.T) {
	t.Parallel()

	subject := New(outboxBacklogQueriesStub{row: generated.GetOutboxBacklogRow{
		PendingEvents:   0,
		OldestPendingAt: pgtype.Timestamptz{},
	}}, time.Second)

	got, err := subject.GetOutboxBacklog(context.Background())

	if err != nil || got.PendingEvents != 0 || !got.OldestPendingAt.IsZero() {
		t.Fatalf("GetOutboxBacklog() = %#v, %v", got, err)
	}
}

func TestGetOutboxBacklogRejectsInconsistentAggregate(t *testing.T) {
	t.Parallel()

	subject := New(outboxBacklogQueriesStub{row: generated.GetOutboxBacklogRow{
		PendingEvents: 1,
	}}, time.Second)

	if _, err := subject.GetOutboxBacklog(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetOutboxBacklog() error = %v", err)
	}
}

func TestGetOutboxBacklogFailsClosed(t *testing.T) {
	t.Parallel()

	subject := New(outboxBacklogQueriesStub{
		err: errors.New("private-outbox-error"),
	}, time.Second)

	if _, err := subject.GetOutboxBacklog(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetOutboxBacklog() error = %v", err)
	}
}
