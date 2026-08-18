package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

type cleanupStub struct {
	err error
}

type expiredRecordCleanerStub struct {
	results        []store.ExpiredRecordCleanupResult
	errs           []error
	calls          int
	cutoffs        []time.Time
	batchSizes     []int32
	cancel         context.CancelFunc
	cancelAt       int
	accessRequests int64
	accessErr      error
	accessCalls    int
	accessCutoffs  []time.Time
	accessBatches  []int32
}

func (s *expiredRecordCleanerStub) ExpireAccommodationAccessRequests(
	_ context.Context,
	cutoff time.Time,
	batchSize int32,
) (int64, error) {
	s.accessCalls++
	s.accessCutoffs = append(s.accessCutoffs, cutoff)
	s.accessBatches = append(s.accessBatches, batchSize)
	return s.accessRequests, s.accessErr
}

type analyticsWorkerStub struct {
	reconcileErr error
	publishErr   error
	reconciled   int
	published    int
}

func (s *analyticsWorkerStub) Reconcile(
	context.Context,
	analytics.ReconciliationKind,
	stay.CivilDate,
) (bool, error) {
	s.reconciled++
	return true, s.reconcileErr
}

func (s *analyticsWorkerStub) BuildAndPublish(
	context.Context,
	stay.CivilDate,
) (int64, bool, error) {
	s.published++
	return 1, false, s.publishErr
}

func (s cleanupStub) EraseExpiredFreeText(
	context.Context,
	time.Time,
) (int32, error) {
	return 0, s.err
}

func (s *expiredRecordCleanerStub) CleanupExpiredOperationalRecords(
	_ context.Context,
	cutoff time.Time,
	batchSize int32,
) (store.ExpiredRecordCleanupResult, error) {
	s.calls++
	s.cutoffs = append(s.cutoffs, cutoff)
	s.batchSizes = append(s.batchSizes, batchSize)
	if s.cancel != nil && s.calls == s.cancelAt {
		s.cancel()
	}
	index := s.calls - 1
	var result store.ExpiredRecordCleanupResult
	if index < len(s.results) {
		result = s.results[index]
	}
	var err error
	if index < len(s.errs) {
		err = s.errs[index]
	}
	return result, err
}

func TestFreeTextCleanupFailureIsObservableWithoutErrorDetails(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "cleanup_test_total"})

	runFreeTextCleanup(
		context.Background(),
		cleanupStub{err: errors.New("private-canary")},
		time.Now(),
		logger,
		counter,
	)

	logs := output.String()
	if !strings.Contains(logs, "free_text_cleanup_failed") {
		t.Fatalf("technical error code missing: %s", logs)
	}
	if strings.Contains(logs, "private-canary") {
		t.Fatalf("underlying error leaked: %s", logs)
	}
}

func TestExpiredRecordCleanupReportsBoundedSuccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.FixedZone("test", -3*60*60))
	service := &expiredRecordCleanerStub{results: []store.ExpiredRecordCleanupResult{{
		IdempotencyRecords: 3,
		RateLimitBuckets:   5,
	}}}
	metrics := newExpiredCleanupTestMetrics("bounded")

	runExpiredRecordCleanup(
		context.Background(), service, now, 100,
		slog.New(slog.DiscardHandler), metrics,
	)

	if service.calls != 1 || service.batchSizes[0] != 100 {
		t.Fatalf("calls=%d batch_size=%v", service.calls, service.batchSizes)
	}
	if !service.cutoffs[0].Equal(now.UTC()) || service.cutoffs[0].Location() != time.UTC {
		t.Fatalf("cutoff=%s", service.cutoffs[0])
	}
	// The retention sweep shares the cycle, so it must share the instant and
	// the bound: a second clock or a second batch size would be a second place
	// where retention can drift without anybody noticing.
	if service.accessCalls != 1 || service.accessBatches[0] != 100 {
		t.Fatalf("access calls=%d batch=%v", service.accessCalls, service.accessBatches)
	}
	if !service.accessCutoffs[0].Equal(now.UTC()) {
		t.Fatalf("access cutoff=%s", service.accessCutoffs[0])
	}
	assertCounterValue(t, metrics.runs.WithLabelValues("success"), 1)
	assertCounterValue(t, metrics.runs.WithLabelValues("failure"), 0)
	assertCounterValue(t, metrics.deleted.WithLabelValues("idempotency"), 3)
	assertCounterValue(t, metrics.deleted.WithLabelValues("rate_limit"), 5)
	assertGaugeValue(t, metrics.saturated, 0)
}

func TestExpiredRecordCleanupFailureIsSanitized(t *testing.T) {
	t.Parallel()
	service := &expiredRecordCleanerStub{errs: []error{errors.New("private-hmac-canary")}}
	metrics := newExpiredCleanupTestMetrics("error")
	var output bytes.Buffer

	runExpiredRecordCleanup(
		context.Background(), service, time.Now(), 100,
		slog.New(slog.NewJSONHandler(&output, nil)), metrics,
	)

	logs := output.String()
	if !strings.Contains(logs, "expired_record_cleanup_failed") {
		t.Fatalf("technical error code missing: %s", logs)
	}
	if strings.Contains(logs, "private-hmac-canary") {
		t.Fatalf("underlying error leaked: %s", logs)
	}
	assertCounterValue(t, metrics.runs.WithLabelValues("failure"), 1)
	assertCounterValue(t, metrics.deleted.WithLabelValues("idempotency"), 0)
	assertCounterValue(t, metrics.deleted.WithLabelValues("rate_limit"), 0)
}

func TestExpiredRecordCleanupCatchesUpAtMostThreeBatches(t *testing.T) {
	t.Parallel()

	const batchSize int32 = 500
	service := &expiredRecordCleanerStub{results: []store.ExpiredRecordCleanupResult{
		{IdempotencyRecords: 500, RateLimitBuckets: 500},
		{IdempotencyRecords: 500, RateLimitBuckets: 201},
		{IdempotencyRecords: 201, RateLimitBuckets: 0},
		{IdempotencyRecords: 1, RateLimitBuckets: 1},
	}}
	metrics := newExpiredCleanupTestMetrics("catchup")

	runExpiredRecordCleanup(
		context.Background(), service, time.Now(), batchSize,
		slog.New(slog.DiscardHandler), metrics,
	)

	if service.calls != 3 {
		t.Fatalf("cleanup calls = %d, want 3", service.calls)
	}
	assertCounterValue(t, metrics.deleted.WithLabelValues("idempotency"), 1201)
	assertCounterValue(t, metrics.deleted.WithLabelValues("rate_limit"), 701)
	assertGaugeValue(t, metrics.saturated, 0)
	assertHistogramCount(t, metrics.batches, 1)
	assertHistogramCount(t, metrics.duration, 1)
}

func TestExpiredRecordCleanupReportsSaturationAtThreeFullBatches(t *testing.T) {
	t.Parallel()

	service := &expiredRecordCleanerStub{results: []store.ExpiredRecordCleanupResult{
		{IdempotencyRecords: 500},
		{IdempotencyRecords: 500},
		{IdempotencyRecords: 500},
		{IdempotencyRecords: 1},
	}}
	metrics := newExpiredCleanupTestMetrics("saturated")

	runExpiredRecordCleanup(
		context.Background(), service, time.Now(), 500,
		slog.New(slog.DiscardHandler), metrics,
	)

	if service.calls != 3 {
		t.Fatalf("cleanup calls = %d, want hard cap 3", service.calls)
	}
	assertGaugeValue(t, metrics.saturated, 1)
}

func TestExpiredRecordCleanupStopsAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	service := &expiredRecordCleanerStub{
		results: []store.ExpiredRecordCleanupResult{
			{IdempotencyRecords: 500},
			{IdempotencyRecords: 500},
		},
		cancel: cancel, cancelAt: 1,
	}
	metrics := newExpiredCleanupTestMetrics("cancelled")
	var output bytes.Buffer

	runExpiredRecordCleanup(
		ctx, service, time.Now(), 500,
		slog.New(slog.NewJSONHandler(&output, nil)), metrics,
	)

	if service.calls != 1 {
		t.Fatalf("cleanup calls after cancellation = %d, want 1", service.calls)
	}
	assertGaugeValue(t, metrics.saturated, 0)
	assertCounterValue(t, metrics.runs.WithLabelValues("success"), 0)
	assertCounterValue(t, metrics.runs.WithLabelValues("cancelled"), 1)
	if strings.Contains(output.String(), "expired_record_cleanup_completed") {
		t.Fatalf("cancelled cleanup logged completion: %s", output.String())
	}
}

func TestPollerCoordinatorCancelsAndWaitsForBlockedPoller(t *testing.T) {
	t.Parallel()

	coordinator := newPollerCoordinator(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	coordinator.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		<-release
		close(exited)
	})
	<-started

	stopped := make(chan struct{})
	go func() {
		waitContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = coordinator.StopAndWait(waitContext)
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("StopAndWait returned before blocked poller exited")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("StopAndWait did not return after poller exit")
	}
	select {
	case <-exited:
	default:
		t.Fatal("poller exit was not observed")
	}
}

func TestPollerCoordinatorTimeoutDefersResourceCloseUntilPollerExit(t *testing.T) {
	t.Parallel()

	coordinator := newPollerCoordinator(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	coordinator.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		<-release
	})
	<-started

	waitContext, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := coordinator.StopAndWait(waitContext); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("StopAndWait() error = %v, want deadline exceeded", err)
	}

	resourceClosed := make(chan struct{})
	var closeCalls atomic.Int32
	closeResources := sync.OnceFunc(func() {
		closeCalls.Add(1)
		close(resourceClosed)
	})
	go closeResourcesAfterPollers(coordinator, closeResources)
	select {
	case <-resourceClosed:
		t.Fatal("resources closed before blocked poller exited")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-resourceClosed:
	case <-time.After(time.Second):
		t.Fatal("resources were not closed after poller exit")
	}
	closeResources()
	if got := closeCalls.Load(); got != 1 {
		t.Fatalf("resource close calls = %d, want 1", got)
	}
}

func TestPollerShutdownTimeoutIsObservableAndSanitized(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "poller_shutdown_timeouts_test_total",
	})
	reportPollerShutdownTimeout(
		slog.New(slog.NewJSONHandler(&output, nil)),
		counter,
	)

	assertCounterValue(t, counter, 1)
	if !strings.Contains(output.String(), "worker_poller_shutdown_timeout") {
		t.Fatalf("timeout error code missing: %s", output.String())
	}
}

type outboxBacklogStub struct {
	backlog store.OutboxBacklog
	err     error
}

func (s outboxBacklogStub) GetOutboxBacklog(context.Context) (store.OutboxBacklog, error) {
	return s.backlog, s.err
}

func TestOutboxBacklogObservationExportsOnlyCountAndAge(t *testing.T) {
	t.Parallel()

	pending := prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending_test"})
	age := prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_age_test"})
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	runOutboxBacklogObservation(
		context.Background(),
		outboxBacklogStub{backlog: store.OutboxBacklog{
			PendingEvents:   7,
			OldestPendingAt: now.Add(-90 * time.Second),
		}},
		now,
		pending,
		age,
	)

	assertGaugeValue(t, pending, 7)
	assertGaugeValue(t, age, 90)
}

func TestOutboxBacklogObservationMarksReadFailureUnavailable(t *testing.T) {
	t.Parallel()

	pending := prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending_error_test"})
	age := prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_age_error_test"})
	runOutboxBacklogObservation(
		context.Background(),
		outboxBacklogStub{err: errors.New("private-outbox-canary")},
		time.Now().UTC(),
		pending,
		age,
	)

	if !math.IsNaN(readGaugeValue(t, pending)) || !math.IsNaN(readGaugeValue(t, age)) {
		t.Fatal("outbox gauges must be NaN when the read is unavailable")
	}
}

func newExpiredCleanupTestMetrics(suffix string) expiredCleanupMetrics {
	return expiredCleanupMetrics{
		runs: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "expired_cleanup_runs_" + suffix + "_test_total"},
			[]string{"result"},
		),
		deleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "expired_cleanup_deleted_" + suffix + "_test_total"},
			[]string{"record_type"},
		),
		duration: prometheus.NewHistogram(
			prometheus.HistogramOpts{Name: "expired_cleanup_duration_" + suffix + "_test"},
		),
		batches: prometheus.NewHistogram(
			prometheus.HistogramOpts{Name: "expired_cleanup_batches_" + suffix + "_test"},
		),
		saturated: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "expired_cleanup_saturated_" + suffix + "_test"},
		),
	}
}

func TestAnalyticsCycleReconcilesBeforePublishing(t *testing.T) {
	t.Parallel()

	service := &analyticsWorkerStub{}
	failures := prometheus.NewCounter(prometheus.CounterOpts{Name: "analytics_failures_test"})
	runs := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "analytics_runs_test"},
		[]string{"kind"},
	)
	runAnalyticsCycle(
		context.Background(), service, analytics.ReconciliationFull,
		time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		slog.New(slog.DiscardHandler), failures, runs,
	)
	if service.reconciled != 1 || service.published != 1 {
		t.Fatalf("reconciled=%d published=%d", service.reconciled, service.published)
	}
	metric := &dto.Metric{}
	if err := runs.WithLabelValues("full").Write(metric); err != nil {
		t.Fatal(err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Fatalf("runs = %v", metric.Counter.GetValue())
	}
}

func assertCounterValue(
	t *testing.T,
	counter prometheus.Counter,
	want float64,
) {
	t.Helper()
	metric := &dto.Metric{}
	if err := counter.Write(metric); err != nil {
		t.Fatal(err)
	}
	if metric.Counter.GetValue() != want {
		t.Fatalf("counter = %v, want %v", metric.Counter.GetValue(), want)
	}
}

func assertGaugeValue(t *testing.T, gauge prometheus.Gauge, want float64) {
	t.Helper()
	got := readGaugeValue(t, gauge)
	if got != want {
		t.Fatalf("gauge = %v, want %v", got, want)
	}
}

func readGaugeValue(t *testing.T, gauge prometheus.Gauge) float64 {
	t.Helper()
	metric := &dto.Metric{}
	if err := gauge.Write(metric); err != nil {
		t.Fatal(err)
	}
	return metric.Gauge.GetValue()
}

func assertHistogramCount(
	t *testing.T,
	observer prometheus.Observer,
	want uint64,
) {
	t.Helper()
	metric := &dto.Metric{}
	writable, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("observer is not a writable metric")
	}
	if err := writable.Write(metric); err != nil {
		t.Fatal(err)
	}
	if got := metric.Histogram.GetSampleCount(); got != want {
		t.Fatalf("histogram count = %d, want %d", got, want)
	}
}

func TestAnalyticsCycleStopsBeforePublicationAndSanitizesFailure(t *testing.T) {
	t.Parallel()

	service := &analyticsWorkerStub{reconcileErr: errors.New("private-stay-canary")}
	failures := prometheus.NewCounter(prometheus.CounterOpts{Name: "analytics_failures_error_test"})
	runs := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "analytics_runs_error_test"},
		[]string{"kind"},
	)
	var output bytes.Buffer
	runAnalyticsCycle(
		context.Background(), service, analytics.ReconciliationIncremental,
		time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		slog.New(slog.NewJSONHandler(&output, nil)), failures, runs,
	)
	if service.published != 0 {
		t.Fatalf("published=%d after reconciliation failure", service.published)
	}
	if strings.Contains(output.String(), "private-stay-canary") {
		t.Fatalf("underlying error leaked: %s", output.String())
	}
	if !strings.Contains(output.String(), "analytics_reconciliation_failed") {
		t.Fatalf("technical error code missing: %s", output.String())
	}
}
