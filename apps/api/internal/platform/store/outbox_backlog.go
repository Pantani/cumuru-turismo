package store

import (
	"context"
	"time"
)

type OutboxBacklog struct {
	PendingEvents   int64
	OldestPendingAt time.Time
}

func (s *Store) GetOutboxBacklog(ctx context.Context) (OutboxBacklog, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	row, err := s.queries.GetOutboxBacklog(ctx)
	if err != nil || row.PendingEvents < 0 {
		return OutboxBacklog{}, ErrUnavailable
	}
	if (row.PendingEvents == 0) == row.OldestPendingAt.Valid {
		return OutboxBacklog{}, ErrUnavailable
	}
	oldest := time.Time{}
	if row.OldestPendingAt.Valid {
		oldest = row.OldestPendingAt.Time.UTC()
	}
	return OutboxBacklog{
		PendingEvents:   row.PendingEvents,
		OldestPendingAt: oldest,
	}, nil
}
