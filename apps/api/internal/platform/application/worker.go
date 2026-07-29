package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/database"
	"github.com/Pantani/cumuru/apps/api/internal/platform/logging"
	"github.com/Pantani/cumuru/apps/api/internal/platform/server"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/platform/telemetry"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var errWorkerPollerShutdownTimeout = errors.New("worker poller shutdown timed out")

func RunWorker(ctx context.Context, cfg config.Config, build Build, output io.Writer) error {
	if err := build.validate(); err != nil {
		return err
	}
	logger := logging.New(output, cfg.LogLevel)
	tracing, err := telemetry.New(ctx, cfg.Telemetry, cfg.ServiceName, build.Version)
	if err != nil {
		return err
	}

	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		shutdownTelemetry(tracing, cfg.ShutdownTimeout, logger)
		return err
	}
	closeResources := sync.OnceFunc(func() {
		pool.Close()
		shutdownTelemetry(tracing, cfg.ShutdownTimeout, logger)
	})
	closeResourcesOnReturn := true
	defer func() {
		if closeResourcesOnReturn {
			closeResources()
		}
	}()
	platformStore, err := store.NewPhase3(pool, cfg.DatabaseTimeout, cfg.Phase2, cfg.Phase3)
	if err != nil {
		return errors.New("worker repository initialization failed")
	}
	questionnaires := questionnaire.NewService(store.NewQuestionnaireRepository(platformStore))

	registry := prometheus.NewRegistry()
	alive := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "alive",
		Help:      "Indica que o processo worker está ativo.",
	})
	ready := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "database_ready",
		Help:      "Indica que a dependência PostgreSQL está pronta.",
	})
	cleanupFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "free_text_cleanup_failures_total",
		Help:      "Total de falhas técnicas no cleanup de texto livre.",
	})
	analyticsFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "analytics_failures_total",
		Help:      "Total de falhas técnicas nos ciclos de analytics.",
	})
	analyticsRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "analytics_runs_total",
		Help:      "Total de ciclos de analytics concluídos por tipo.",
	}, []string{"kind"})
	expiredCleanupRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "expired_record_cleanup_runs_total",
		Help:      "Total de ciclos de cleanup expirado por resultado técnico.",
	}, []string{"result"})
	expiredCleanupDeleted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "expired_record_cleanup_deleted_total",
		Help:      "Total de registros expirados removidos por tipo fixo.",
	}, []string{"record_type"})
	expiredCleanupDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "expired_record_cleanup_duration_seconds",
		Help:      "Duração de cada ciclo bounded de cleanup expirado.",
	})
	expiredCleanupBatches := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "expired_record_cleanup_batches",
		Help:      "Lotes executados por ciclo bounded de cleanup expirado.",
		Buckets:   []float64{0, 1, 2, 3},
	})
	expiredCleanupSaturated := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "expired_record_cleanup_saturated",
		Help:      "Indica que o ciclo encerrou após três lotes completos.",
	})
	outboxPending := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "outbox_pending_events",
		Help:      "Quantidade agregada de eventos pendentes na outbox.",
	})
	outboxOldestAge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "outbox_oldest_pending_age_seconds",
		Help:      "Idade agregada do evento pendente mais antigo.",
	})
	pollerShutdownTimeouts := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cumuru",
		Subsystem: "worker",
		Name:      "poller_shutdown_timeouts_total",
		Help:      "Total de deadlines excedidos ao aguardar pollers no shutdown.",
	})
	registry.MustRegister(
		alive, ready, cleanupFailures, analyticsFailures, analyticsRuns,
		expiredCleanupRuns, expiredCleanupDeleted,
		expiredCleanupDuration, expiredCleanupBatches, expiredCleanupSaturated,
		outboxPending, outboxOldestAge, pollerShutdownTimeouts,
	)
	alive.Set(1)
	defer alive.Set(0)
	updateReadiness(ctx, platformStore, ready)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	listener, err := net.Listen("tcp", cfg.OperationsAddress)
	if err != nil {
		return errors.New("worker operations listener failed")
	}
	pollers := newPollerCoordinator(ctx)
	pollers.Go(func(pollerContext context.Context) {
		pollReadiness(pollerContext, platformStore, ready)
	})
	pollers.Go(func(pollerContext context.Context) {
		pollOutboxBacklog(
			pollerContext, platformStore, outboxPending, outboxOldestAge,
		)
	})
	cleanupMetrics := expiredCleanupMetrics{
		runs: expiredCleanupRuns, deleted: expiredCleanupDeleted,
		duration: expiredCleanupDuration, batches: expiredCleanupBatches,
		saturated: expiredCleanupSaturated,
	}
	pollers.Go(func(pollerContext context.Context) {
		pollExpiredRecordCleanup(
			pollerContext, platformStore, logger, cleanupMetrics,
		)
	})
	if cfg.Phase3.Enabled {
		pollers.Go(func(pollerContext context.Context) {
			pollFreeTextCleanup(
				pollerContext, questionnaires, logger, cleanupFailures,
			)
		})
	}
	if cfg.Phase4.Enabled {
		analyticsRepository := store.NewAnalyticsRepository(platformStore, cfg.Phase4)
		pollers.Go(func(pollerContext context.Context) {
			pollAnalytics(
				pollerContext, analyticsRepository, cfg.Phase4, logger,
				analyticsFailures, analyticsRuns,
			)
		})
	}
	logger.Info("worker started", "component", "worker")
	serveErr := server.Serve(
		pollers.Context(),
		listener,
		configuredHTTPServer(mux, cfg),
		cfg.ShutdownTimeout,
		logger,
	)
	return finishWorkerShutdown(
		serveErr, pollers, cfg.ShutdownTimeout, logger,
		pollerShutdownTimeouts, closeResources, &closeResourcesOnReturn,
	)
}

func finishWorkerShutdown(
	serveErr error,
	pollers *pollerCoordinator,
	timeout time.Duration,
	logger *slog.Logger,
	pollerShutdownTimeouts prometheus.Counter,
	closeResources func(),
	closeResourcesOnReturn *bool,
) error {
	shutdownContext, shutdownCancel := context.WithTimeout(
		context.Background(), timeout,
	)
	defer shutdownCancel()
	if err := pollers.StopAndWait(shutdownContext); err != nil {
		reportPollerShutdownTimeout(logger, pollerShutdownTimeouts)
		*closeResourcesOnReturn = false
		go closeResourcesAfterPollers(pollers, closeResources)
		return errors.Join(serveErr, errWorkerPollerShutdownTimeout)
	}
	return serveErr
}

const (
	expiredRecordCleanupBatchSize  int32 = 500
	expiredRecordCleanupInterval         = time.Minute
	expiredRecordCleanupMaxBatches       = 3
)

type pollerCoordinator struct {
	ctx      context.Context
	cancel   context.CancelFunc
	group    sync.WaitGroup
	stopOnce sync.Once
	done     chan struct{}
}

func newPollerCoordinator(parent context.Context) *pollerCoordinator {
	ctx, cancel := context.WithCancel(parent)
	return &pollerCoordinator{
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
	}
}

func (c *pollerCoordinator) Context() context.Context {
	return c.ctx
}

func (c *pollerCoordinator) Go(poller func(context.Context)) {
	c.group.Add(1)
	go func() {
		defer c.group.Done()
		poller(c.ctx)
	}()
}

func (c *pollerCoordinator) StopAndWait(ctx context.Context) error {
	c.stop()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *pollerCoordinator) Wait() {
	c.stop()
	<-c.done
}

func (c *pollerCoordinator) stop() {
	c.stopOnce.Do(func() {
		c.cancel()
		go func() {
			c.group.Wait()
			close(c.done)
		}()
	})
}

func closeResourcesAfterPollers(
	coordinator *pollerCoordinator,
	closeResources func(),
) {
	coordinator.Wait()
	closeResources()
}

func reportPollerShutdownTimeout(
	logger *slog.Logger,
	timeouts prometheus.Counter,
) {
	timeouts.Inc()
	logger.Error(
		"worker poller shutdown timed out",
		"error_code", "worker_poller_shutdown_timeout",
	)
}

type expiredRecordCleaner interface {
	CleanupExpiredOperationalRecords(
		context.Context,
		time.Time,
		int32,
	) (store.ExpiredRecordCleanupResult, error)
}

type expiredCleanupMetrics struct {
	runs      *prometheus.CounterVec
	deleted   *prometheus.CounterVec
	duration  prometheus.Histogram
	batches   prometheus.Histogram
	saturated prometheus.Gauge
}

func pollExpiredRecordCleanup(
	ctx context.Context,
	cleaner expiredRecordCleaner,
	logger *slog.Logger,
	metrics expiredCleanupMetrics,
) {
	runExpiredRecordCleanup(
		ctx, cleaner, time.Now().UTC(), expiredRecordCleanupBatchSize,
		logger, metrics,
	)
	ticker := time.NewTicker(expiredRecordCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			runExpiredRecordCleanup(
				ctx, cleaner, now.UTC(), expiredRecordCleanupBatchSize,
				logger, metrics,
			)
		}
	}
}

func runExpiredRecordCleanup(
	ctx context.Context,
	cleaner expiredRecordCleaner,
	cutoff time.Time,
	batchSize int32,
	logger *slog.Logger,
	metrics expiredCleanupMetrics,
) {
	started := time.Now()
	batches := 0
	saturated := false
	defer func() {
		metrics.duration.Observe(time.Since(started).Seconds())
		metrics.batches.Observe(float64(batches))
		metrics.saturated.Set(boolMetric(saturated))
	}()
	for batches < expiredRecordCleanupMaxBatches && ctx.Err() == nil {
		result, ok := runExpiredCleanupBatch(
			ctx, cleaner, cutoff.UTC(), batchSize, logger, metrics,
		)
		if !ok {
			return
		}
		batches++
		metrics.deleted.WithLabelValues("idempotency").Add(
			float64(result.IdempotencyRecords),
		)
		metrics.deleted.WithLabelValues("rate_limit").Add(
			float64(result.RateLimitBuckets),
		)
		full := cleanupBatchFull(result, batchSize)
		saturated = full && batches == expiredRecordCleanupMaxBatches
		if !full {
			break
		}
	}
	if ctx.Err() != nil {
		recordExpiredCleanupCancellation(logger, metrics)
		return
	}
	metrics.runs.WithLabelValues("success").Inc()
	logger.Info(
		"expired record cleanup completed",
		"result_code",
		"expired_record_cleanup_completed",
	)
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func runExpiredCleanupBatch(
	ctx context.Context,
	cleaner expiredRecordCleaner,
	cutoff time.Time,
	batchSize int32,
	logger *slog.Logger,
	metrics expiredCleanupMetrics,
) (store.ExpiredRecordCleanupResult, bool) {
	result, err := cleaner.CleanupExpiredOperationalRecords(
		ctx, cutoff, batchSize,
	)
	if err == nil {
		return result, true
	}
	if ctx.Err() != nil {
		recordExpiredCleanupCancellation(logger, metrics)
		return store.ExpiredRecordCleanupResult{}, false
	}
	metrics.runs.WithLabelValues("failure").Inc()
	logger.Error(
		"expired record cleanup failed",
		"error_code", "expired_record_cleanup_failed",
	)
	return store.ExpiredRecordCleanupResult{}, false
}

func recordExpiredCleanupCancellation(
	logger *slog.Logger,
	metrics expiredCleanupMetrics,
) {
	metrics.runs.WithLabelValues("cancelled").Inc()
	logger.Info(
		"expired record cleanup cancelled",
		"result_code", "expired_record_cleanup_cancelled",
	)
}

func cleanupBatchFull(result store.ExpiredRecordCleanupResult, batchSize int32) bool {
	bound := int64(batchSize)
	return result.IdempotencyRecords >= bound || result.RateLimitBuckets >= bound
}

type outboxBacklogReader interface {
	GetOutboxBacklog(context.Context) (store.OutboxBacklog, error)
}

func pollOutboxBacklog(
	ctx context.Context,
	reader outboxBacklogReader,
	pending prometheus.Gauge,
	oldestAge prometheus.Gauge,
) {
	runOutboxBacklogObservation(ctx, reader, time.Now().UTC(), pending, oldestAge)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			runOutboxBacklogObservation(
				ctx, reader, now.UTC(), pending, oldestAge,
			)
		}
	}
}

func runOutboxBacklogObservation(
	ctx context.Context,
	reader outboxBacklogReader,
	now time.Time,
	pending prometheus.Gauge,
	oldestAge prometheus.Gauge,
) {
	backlog, err := reader.GetOutboxBacklog(ctx)
	if err != nil {
		pending.Set(math.NaN())
		oldestAge.Set(math.NaN())
		return
	}
	pending.Set(float64(backlog.PendingEvents))
	oldestAge.Set(outboxAgeSeconds(backlog, now.UTC()))
}

func outboxAgeSeconds(backlog store.OutboxBacklog, now time.Time) float64 {
	if backlog.PendingEvents == 0 || backlog.OldestPendingAt.IsZero() {
		return 0
	}
	age := now.Sub(backlog.OldestPendingAt.UTC()).Seconds()
	if age < 0 {
		return 0
	}
	return age
}

type freeTextEraser interface {
	EraseExpiredFreeText(context.Context, time.Time) (int32, error)
}

func pollFreeTextCleanup(
	ctx context.Context,
	service freeTextEraser,
	logger *slog.Logger,
	failures prometheus.Counter,
) {
	runFreeTextCleanup(ctx, service, time.Now().UTC(), logger, failures)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			runFreeTextCleanup(ctx, service, now.UTC(), logger, failures)
		}
	}
}

func runFreeTextCleanup(
	ctx context.Context,
	service freeTextEraser,
	now time.Time,
	logger *slog.Logger,
	failures prometheus.Counter,
) {
	if _, err := service.EraseExpiredFreeText(ctx, now); err != nil {
		failures.Inc()
		logger.Error(
			"free text cleanup failed",
			"error_code",
			"free_text_cleanup_failed",
		)
	}
}

func pollReadiness(ctx context.Context, platformStore *store.Store, gauge prometheus.Gauge) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateReadiness(ctx, platformStore, gauge)
		}
	}
}

func updateReadiness(ctx context.Context, platformStore *store.Store, gauge prometheus.Gauge) {
	if platformStore.CheckReadiness(ctx) != nil {
		gauge.Set(0)
		return
	}
	gauge.Set(1)
}

type analyticsWorker interface {
	Reconcile(
		context.Context,
		analytics.ReconciliationKind,
		stay.CivilDate,
	) (bool, error)
	BuildAndPublish(context.Context, stay.CivilDate) (int64, bool, error)
}

func pollAnalytics(
	ctx context.Context,
	service analyticsWorker,
	phase4 config.Phase4Config,
	logger *slog.Logger,
	failures prometheus.Counter,
	runs *prometheus.CounterVec,
) {
	runAnalyticsCycle(
		ctx, service, analytics.ReconciliationFull, time.Now().UTC(),
		logger, failures, runs,
	)
	incremental := time.NewTicker(phase4.IncrementalInterval)
	full := time.NewTicker(phase4.FullReconciliationInterval)
	defer incremental.Stop()
	defer full.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-incremental.C:
			runAnalyticsCycle(
				ctx, service, analytics.ReconciliationIncremental, now.UTC(),
				logger, failures, runs,
			)
		case now := <-full.C:
			runAnalyticsCycle(
				ctx, service, analytics.ReconciliationFull, now.UTC(),
				logger, failures, runs,
			)
		}
	}
}

func runAnalyticsCycle(
	ctx context.Context,
	service analyticsWorker,
	kind analytics.ReconciliationKind,
	now time.Time,
	logger *slog.Logger,
	failures prometheus.Counter,
	runs *prometheus.CounterVec,
) {
	asOf := stay.CivilDateFromInstant(now)
	if _, err := service.Reconcile(ctx, kind, asOf); err != nil {
		failures.Inc()
		logger.Error(
			"analytics reconciliation failed",
			"error_code", "analytics_reconciliation_failed",
			"run_kind", string(kind),
		)
		return
	}
	if _, _, err := service.BuildAndPublish(ctx, asOf); err != nil {
		failures.Inc()
		logger.Error(
			"analytics publication failed",
			"error_code", "analytics_publication_failed",
			"run_kind", string(kind),
		)
		return
	}
	runs.WithLabelValues(string(kind)).Inc()
	logger.Info("analytics cycle completed", "run_kind", string(kind))
}
