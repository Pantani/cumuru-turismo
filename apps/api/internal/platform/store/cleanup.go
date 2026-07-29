package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
)

const maxExpiredRecordCleanupBatch int32 = 1000

type ExpiredRecordCleanupResult struct {
	IdempotencyRecords int64
	RateLimitBuckets   int64
}

func (s *Store) CleanupExpiredOperationalRecords(
	ctx context.Context,
	cutoff time.Time,
	batchSize int32,
) (ExpiredRecordCleanupResult, error) {
	if cutoff.IsZero() || batchSize < 1 || batchSize > maxExpiredRecordCleanupBatch {
		return ExpiredRecordCleanupResult{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	result, err := s.queries.CleanupExpiredOperationalRecords(
		ctx,
		generated.CleanupExpiredOperationalRecordsParams{
			ExpiredBefore: pgTime(cutoff),
			BatchSize:     batchSize,
		},
	)
	if err != nil {
		return ExpiredRecordCleanupResult{}, ErrUnavailable
	}
	return ExpiredRecordCleanupResult{
		IdempotencyRecords: result.IdempotencyRecords,
		RateLimitBuckets:   result.RateLimitBuckets,
	}, nil
}
