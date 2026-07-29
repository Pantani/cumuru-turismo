package store

import (
	"context"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const publicRuntimeSearchPath = "pg_catalog, public_data"

func OpenPublicPool(
	ctx context.Context,
	databaseURL string,
	timeout time.Duration,
) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil || timeout <= 0 {
		return nil, errors.New("public database configuration invalid")
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 0
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.AfterConnect = publicSessionBootstrap(timeout)
	poolConfig.PrepareConn = publicSessionValidation(timeout)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("public database initialization failed")
	}
	pingContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, errors.New("public database unavailable")
	}
	return pool, nil
}

func publicSessionBootstrap(timeout time.Duration) func(context.Context, *pgx.Conn) error {
	return func(ctx context.Context, connection *pgx.Conn) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		queries := generated.New(connection)
		if err := queries.AssumePublicRuntimeRole(ctx); err != nil {
			return ErrUnavailable
		}
		if err := queries.SetPublicRuntimeSearchPath(ctx); err != nil {
			return ErrUnavailable
		}
		row, err := queries.ValidatePublicRuntimeSession(ctx)
		if err != nil || !validPublicSession(row) {
			return ErrUnavailable
		}
		return nil
	}
}

func publicSessionValidation(
	timeout time.Duration,
) func(context.Context, *pgx.Conn) (bool, error) {
	return func(ctx context.Context, connection *pgx.Conn) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		row, err := generated.New(connection).ValidatePublicRuntimeSession(ctx)
		if err != nil || !validPublicSession(row) {
			return false, ErrUnavailable
		}
		return true, nil
	}
}

func validPublicSession(row generated.ValidatePublicRuntimeSessionRow) bool {
	return row.CurrentUserName == "public_runtime" &&
		row.SessionUserName != "" &&
		row.SessionUserName != row.CurrentUserName &&
		row.SearchPath == publicRuntimeSearchPath
}
