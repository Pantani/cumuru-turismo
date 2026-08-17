package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	approvalExpiryInterval   = time.Minute
	approvalExpiryMaxBatches = 3
)

// approvalExpirer is the narrow view the poller needs. Keeping it an interface
// is what lets the sweep be tested without a database.
type approvalExpirer interface {
	ExpireApprovals(context.Context) (int, error)
}

type approvalExpiryMetrics struct {
	runs    prometheus.Counter
	expired prometheus.Counter
	failed  prometheus.Counter
}

func newApprovalExpiryMetrics() approvalExpiryMetrics {
	return approvalExpiryMetrics{
		runs: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cumuru", Subsystem: "worker",
			Name: "approval_expiry_runs_total",
			Help: "Varreduras de expiração de autocadastro pendente.",
		}),
		expired: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cumuru", Subsystem: "worker",
			Name: "approval_expiry_stays_total",
			Help: "Estadias autocadastradas expiradas e purgadas.",
		}),
		failed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cumuru", Subsystem: "worker",
			Name: "approval_expiry_failures_total",
			Help: "Falhas da varredura de expiração.",
		}),
	}
}

func (m approvalExpiryMetrics) collectors() []prometheus.Collector {
	return []prometheus.Collector{m.runs, m.expired, m.failed}
}

// pollApprovalExpiry runs the sweep that makes the retention policy real.
// Erasing only on rejection would leave doing nothing as the cheapest way to
// keep a stranger's submission forever, so the expiry performs the same purge
// the rejection performs (E-05, N-30).
func pollApprovalExpiry(
	ctx context.Context,
	expirer approvalExpirer,
	logger *slog.Logger,
	metrics approvalExpiryMetrics,
) {
	runApprovalExpiry(ctx, expirer, logger, metrics)
	ticker := time.NewTicker(approvalExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runApprovalExpiry(ctx, expirer, logger, metrics)
		}
	}
}

// The cycle is bounded so one run can never monopolise the worker, and it stops
// as soon as the context is cancelled or a batch comes back empty.
func runApprovalExpiry(
	ctx context.Context,
	expirer approvalExpirer,
	logger *slog.Logger,
	metrics approvalExpiryMetrics,
) {
	metrics.runs.Inc()
	for batch := 0; batch < approvalExpiryMaxBatches && ctx.Err() == nil; batch++ {
		expired, err := expirer.ExpireApprovals(ctx)
		if err != nil {
			metrics.failed.Inc()
			// The log carries a code, never an identifier: nothing about a
			// pending submission may reach a log line.
			logger.WarnContext(ctx, "approval expiry sweep failed")
			return
		}
		metrics.expired.Add(float64(expired))
		if expired == 0 {
			return
		}
	}
}
