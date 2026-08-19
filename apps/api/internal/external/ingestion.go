package external

import (
	"context"
	"log/slog"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/google/uuid"
)

const (
	// A cycle deletes expired observations in one bounded batch. Raw third
	// party responses are a cache with a deadline, not an archive.
	retentionBatchSize int32 = 500

	// The breaker opens after this many consecutive failed cycles and stays
	// open for the cooldown, so a source that is down stops being asked and the
	// panel keeps serving its last valid observation.
	breakerFailureThreshold = 3
	breakerCooldown         = 12 * time.Hour
)

// Ingestion is the worker-side cycle. It exists only where the worker builds
// it: the API process never constructs one, and the handler depends on
// ContextReader, which offers no way to reach it.
type Ingestion struct {
	repository Repository
	fetcher    *Fetcher
	targets    []Target
	logger     *slog.Logger
	metrics    *Metrics
	budget     time.Duration
	now        func() time.Time
	breakers   map[string]breakerState
}

type breakerState struct {
	failures  int
	openUntil time.Time
}

func NewIngestion(
	repository Repository,
	fetcher *Fetcher,
	settings config.ExternalContextConfig,
	logger *slog.Logger,
	metrics *Metrics,
) *Ingestion {
	return &Ingestion{
		repository: repository,
		fetcher:    fetcher,
		targets:    allowedTargets(fetcher, logger),
		logger:     logger,
		metrics:    metrics,
		budget:     settings.BatchBudget,
		now:        func() time.Time { return time.Now().UTC() },
		breakers:   map[string]breakerState{},
	}
}

// A target outside the allowlist is left out of the cycle and said so once, at
// assembly. Failing once per cycle forever would be noise, and silently
// pretending the target does not exist would be worse.
func allowedTargets(fetcher *Fetcher, logger *slog.Logger) []Target {
	targets := make([]Target, 0, len(DeclaredTargets()))
	for _, declared := range DeclaredTargets() {
		target, ok := prepare(declared)
		if !ok || !fetcher.Allows(target.URL) {
			logger.Info(
				"external target disabled",
				"source_code", declared.SourceCode,
				"host", declared.Host,
				"result_code", "external_target_not_allowlisted",
			)
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

// RunCycle walks every allowed target under a budget of its own.
// DATABASE_TIMEOUT sizes a request and does not size an ingestion cycle, which
// is the lesson the two-year seed already charged once.
func (i *Ingestion) RunCycle(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, i.budget)
	defer cancel()
	i.seedCatalog(ctx)
	for _, target := range i.targets {
		i.runTarget(ctx, target)
	}
	i.sweepRetention(ctx)
}

// The catalogue is seeded before any target runs, and a failure here does not
// stop the cycle. Both properties matter: the tide card and the credited-only
// sources must exist whether or not the upstream answered, because neither of
// them is ever produced by a fetch. The upserts carry the same values every
// time, so running the cycle again rewrites nothing and duplicates nothing.
func (i *Ingestion) seedCatalog(ctx context.Context) {
	for _, source := range SeededSources() {
		i.reportSeed(source.SourceCode, i.repository.EnsureSource(ctx, source))
	}
	for _, series := range SeededSeries() {
		i.reportSeed(series.SourceCode, i.repository.EnsureSeries(ctx, series))
	}
}

func (i *Ingestion) reportSeed(sourceCode string, err error) {
	if err == nil {
		return
	}
	i.metrics.skipped.WithLabelValues(sourceCode, "catalogue_seed_failed").Inc()
	i.logger.Error(
		"external catalogue seeding failed",
		"source_code", sourceCode,
		"error_code", "external_catalogue_seed_failed",
	)
}

// One failing source never aborts the cycle, never aborts the worker and never
// erases the last valid observation: the next target runs, and the card of this
// one degrades to `unavailable` with its provenance intact.
func (i *Ingestion) runTarget(ctx context.Context, target Target) {
	if i.breakerOpen(target.SourceCode) {
		i.metrics.skipped.WithLabelValues(
			target.SourceCode, "breaker_open",
		).Inc()
		return
	}
	outcome := i.executeTarget(ctx, target)
	i.metrics.runs.WithLabelValues(target.SourceCode, outcome).Inc()
	i.recordBreaker(target.SourceCode, outcome)
}

func (i *Ingestion) executeTarget(ctx context.Context, target Target) string {
	// The catalogue and the run row are writes too, and a failure here never
	// reaches `fetch_runs`: the row could not be started. The label is still
	// `write_error`, because the metric would otherwise blame the network for
	// a database that was unreachable.
	if err := i.ensureCatalog(ctx, target); err != nil {
		return OutcomeWriteError
	}
	run := RunStart{
		ID:         uuid.New(),
		SourceCode: target.SourceCode,
		StartedAt:  i.now(),
		// Pessimistic on purpose: a run interrupted before it finishes is
		// indistinguishable from one that failed, and that is the honest
		// default for a row that decides whether a card publishes.
		Outcome: OutcomeHTTPError,
	}
	if err := i.repository.StartRun(ctx, run); err != nil {
		return OutcomeWriteError
	}
	result := i.fetchAndStore(ctx, target, run.ID)
	i.finishRun(ctx, run.ID, result)
	return result.Outcome
}

func (i *Ingestion) ensureCatalog(ctx context.Context, target Target) error {
	if err := i.repository.EnsureSource(ctx, target.Source); err != nil {
		return err
	}
	return i.repository.EnsureSeries(ctx, target.Series)
}

func (i *Ingestion) finishRun(
	ctx context.Context,
	id uuid.UUID,
	result RunResult,
) {
	result.ID = id
	result.FinishedAt = i.now()
	if err := i.repository.FinishRun(ctx, result); err != nil {
		i.logger.Error(
			"external fetch run not finalised",
			"error_code", "external_fetch_run_not_finalised",
		)
	}
}

func (i *Ingestion) fetchAndStore(
	ctx context.Context,
	target Target,
	runID uuid.UUID,
) RunResult {
	response, outcome := i.fetcher.Fetch(ctx, target)
	// Sem resposta HTTP — DNS, recusa de conexão, timeout de transporte — não
	// existe status a gravar. Persistir 0 inventaria um código que o protocolo
	// não tem e faria a trilha afirmar "o servidor respondeu 0" onde ninguém
	// respondeu. A coluna é nula justamente para distinguir os dois casos.
	result := RunResult{Outcome: outcome}
	if response.status > 0 {
		status := int32(response.status)
		result.HTTPStatus = &status
	}
	if outcome != OutcomeOK {
		return result
	}
	points, err := target.Parse(response.body)
	if err != nil {
		result.Outcome = OutcomeParseError
		return result
	}
	return i.storeResult(ctx, target, runID, points, result)
}

func (i *Ingestion) storeResult(
	ctx context.Context,
	target Target,
	runID uuid.UUID,
	points []ObservedPoint,
	result RunResult,
) RunResult {
	written, err := i.storePoints(ctx, target, runID, points)
	result.ObservationsWritten = written
	i.metrics.written.WithLabelValues(target.SourceCode).Add(float64(written))
	if err != nil {
		result.Outcome = storageOutcome(ctx)
		result.BatchBudgetExhausted = ctx.Err() != nil
		return result
	}
	result.Outcome = writeOutcome(written)
	return result
}

// A cycle that ran out of its budget says so; anything else that stopped the
// write was the database, and it is recorded as such. Naming a storage failure
// `http_error` would send whoever debugs it looking at the network for
// something that happened in persistence — the same failure of attribution
// that `fetch_runs` exists to prevent one level up.
func storageOutcome(ctx context.Context) string {
	if ctx.Err() != nil {
		return OutcomeSkippedBudget
	}
	return OutcomeWriteError
}

func writeOutcome(written int32) string {
	if written == 0 {
		return OutcomeUnchanged
	}
	return OutcomeOK
}

func (i *Ingestion) storePoints(
	ctx context.Context,
	target Target,
	runID uuid.UUID,
	points []ObservedPoint,
) (int32, error) {
	written := int32(0)
	for _, point := range points {
		stored, err := i.storePoint(ctx, target, runID, point)
		if err != nil {
			return written, err
		}
		written += storedCount(stored)
	}
	return written, nil
}

func storedCount(stored bool) int32 {
	if stored {
		return 1
	}
	return 0
}

// Idempotence is decided by the database, not by memory of a previous cycle:
// the unique index on (source, series, period, digest) turns an identical fact
// into a no-op, and the affected-row count of the insert reports whether a row
// actually appeared — zero for the repeated fact, one for a new revision.
//
// The revision still comes from a read, because it is the value being written
// (max+1). What no longer costs a second round trip is the question "did the
// row land?": the write itself answers it.
func (i *Ingestion) storePoint(
	ctx context.Context,
	target Target,
	runID uuid.UUID,
	point ObservedPoint,
) (bool, error) {
	key := ObservationKey{
		SourceCode:  target.SourceCode,
		SeriesCode:  target.Series.SeriesCode,
		PeriodStart: point.PeriodStart,
	}
	revision, err := i.repository.NextRevision(ctx, key)
	if err != nil {
		return false, err
	}
	written, err := i.insert(ctx, target, runID, point, key, revision)
	if err != nil {
		return false, err
	}
	return written == 1, nil
}

func (i *Ingestion) insert(
	ctx context.Context,
	target Target,
	runID uuid.UUID,
	point ObservedPoint,
	key ObservationKey,
	revision int32,
) (int64, error) {
	return i.repository.InsertObservation(ctx, ObservationRecord{
		Key:           key,
		PeriodKind:    target.Series.PeriodKind,
		PeriodEnd:     point.PeriodEnd,
		Revision:      revision,
		Value:         point.Value,
		QualityFlag:   "ok",
		RetrievedAt:   i.now(),
		PayloadDigest: digestOf(target, point),
		FetchRunID:    runID,
	})
}

func (i *Ingestion) sweepRetention(ctx context.Context) {
	deleted, err := i.repository.DeleteExpiredObservations(
		ctx, i.now(), retentionBatchSize,
	)
	if err != nil {
		i.logger.Error(
			"external retention sweep failed",
			"error_code", "external_retention_sweep_failed",
		)
		return
	}
	i.metrics.retention.Add(float64(deleted))
}

func (i *Ingestion) breakerOpen(sourceCode string) bool {
	return i.breakers[sourceCode].openUntil.After(i.now())
}

func failedOutcome(outcome string) bool {
	return outcome != OutcomeOK && outcome != OutcomeUnchanged
}

// The breaker records no `fetch_runs` row while it is open. Fabricating a run
// would put a value from the closed vocabulary on something that never
// happened; the previous failing run stays the latest, and the card stays
// unavailable for the reason that is actually true.
func (i *Ingestion) recordBreaker(sourceCode, outcome string) {
	if !failedOutcome(outcome) {
		delete(i.breakers, sourceCode)
		return
	}
	state := i.breakers[sourceCode]
	state.failures++
	if state.failures >= breakerFailureThreshold {
		state.openUntil = i.now().Add(breakerCooldown)
		state.failures = 0
	}
	i.breakers[sourceCode] = state
}
