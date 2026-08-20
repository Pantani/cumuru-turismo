package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenExternalPool opens the ingestion pool of the external context layer,
// under `cumuru_external` and therefore under `external_runtime`.
//
// It is a pool of its own, and that is the whole point: `worker_runtime`
// reconciles the protected series and holds no privilege in `external`, while
// `external_runtime` reaches neither `core`, `survey`, `analytics` nor
// `public_data`. The two live in the same worker process and never in the same
// connection, so the direction of ADR-045 §1 is enforced by PostgreSQL rather
// than by which function a future change happens to call.
//
// Only the worker opens it. The API reaches the layer through the public pool,
// which sees the view in `public_data` and nothing else, so the read path never
// holds a connection that could write here.
func OpenExternalPool(
	ctx context.Context,
	databaseURL string,
	timeout time.Duration,
) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil || timeout <= 0 {
		return nil, errors.New("external database configuration invalid")
	}
	// The ingestion is one bounded cycle every few hours, not a request path.
	// A wide pool here would only hold idle connections against a database that
	// is sized for the surfaces people actually wait on.
	poolConfig.MaxConns = 2
	poolConfig.MinConns = 0
	poolConfig.MaxConnIdleTime = time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("external database initialization failed")
	}
	pingContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, errors.New("external database unavailable")
	}
	return pool, nil
}
