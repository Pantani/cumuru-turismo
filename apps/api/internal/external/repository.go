package external

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistence the ingestion needs, under `external_runtime`
// and its own pool. Every method here writes inside the `external` schema and
// none of them can reach the protected series, because the role behind the pool
// holds no privilege there.
type Repository interface {
	EnsureSource(context.Context, SourceRecord) error
	EnsureSeries(context.Context, SeriesRecord) error
	StartRun(context.Context, RunStart) error
	FinishRun(context.Context, RunResult) error
	NextRevision(context.Context, ObservationKey) (int32, error)
	// Devolve quantas linhas entraram: 0 quando o digest já existia
	// (no-op idempotente), 1 quando a revisão é nova.
	InsertObservation(context.Context, ObservationRecord) (int64, error)
	DeleteExpiredObservations(context.Context, time.Time, int32) (int64, error)
}

type RunStart struct {
	ID         uuid.UUID
	SourceCode string
	StartedAt  time.Time
	Outcome    string
}

// RunResult always lands in `fetch_runs`. Without that row, "the source is
// unavailable" and "the cycle never ran" are the same silence, and the public
// card reads the outcome of the last run rather than the absence of rows.
type RunResult struct {
	ID                   uuid.UUID
	FinishedAt           time.Time
	Outcome              string
	HTTPStatus           *int32
	ObservationsWritten  int32
	BatchBudgetExhausted bool
}

type ObservationKey struct {
	SourceCode  string
	SeriesCode  string
	PeriodStart time.Time
}

type ObservationRecord struct {
	Key                 ObservationKey
	PeriodKind          string
	PeriodEnd           time.Time
	Revision            int32
	Value               float64
	QualityFlag         string
	RetrievedAt         time.Time
	SourceRevisionLabel string
	PayloadDigest       string
	FetchRunID          uuid.UUID
}
