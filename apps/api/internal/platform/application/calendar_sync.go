package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/prometheus/client_golang/prometheus"
)

// calendarSyncTick is the cadence of the loop, not of a feed. Each feed is only
// re-read after calendarfeed.SyncInterval; the loop wakes more often so a feed
// registered a minute ago does not wait two hours to show its first
// reservations.
const calendarSyncTick = time.Minute

type calendarSynchronizer interface {
	SyncDue(context.Context, int32) (int, error)
}

type calendarSyncMetrics struct {
	failures prometheus.Counter
	feeds    prometheus.Counter
}

// calendarSyncWorker is nil when the feature is off. With the feature on and an
// unusable sealing key the synchronizer refuses to exist, and the worker fails
// to start rather than polling feeds it cannot decrypt.
func calendarSyncWorker(
	platformStore *store.Store,
	cfg config.Config,
) (*calendarfeed.Synchronizer, error) {
	if !cfg.CalendarFeed.Enabled {
		return nil, nil
	}
	sealer, err := calendarFeedSealer(cfg)
	if err != nil {
		return nil, err
	}
	fetcher := calendarfeed.NewHTTPFetcher(
		cfg.CalendarFeed.FetchTimeout, cfg.CalendarFeed.FetchLimit,
	)
	return calendarfeed.NewSynchronizer(
		store.NewCalendarFeedRepository(platformStore), fetcher, sealer,
	)
}

func pollCalendarSync(
	ctx context.Context,
	synchronizer calendarSynchronizer,
	batchSize int32,
	logger *slog.Logger,
	metrics calendarSyncMetrics,
) {
	runCalendarSync(ctx, synchronizer, batchSize, logger, metrics)
	ticker := time.NewTicker(calendarSyncTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCalendarSync(ctx, synchronizer, batchSize, logger, metrics)
		}
	}
}

// A failure here is the cycle failing, not a feed failing: an unreachable feed
// is written onto its own row and never reaches this path. The log carries a
// code and no address, because the address is a bearer secret.
func runCalendarSync(
	ctx context.Context,
	synchronizer calendarSynchronizer,
	batchSize int32,
	logger *slog.Logger,
	metrics calendarSyncMetrics,
) {
	count, err := synchronizer.SyncDue(ctx, batchSize)
	if err != nil {
		metrics.failures.Inc()
		logger.Error(
			"calendar feed synchronization failed",
			"error_code",
			"calendar_feed_sync_failed",
		)
		return
	}
	metrics.feeds.Add(float64(count))
}

func calendarSyncMetricSet() calendarSyncMetrics {
	return calendarSyncMetrics{
		failures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cumuru_worker_calendar_sync_failures_total",
			Help: "Ciclos de sincronização de calendário que falharam.",
		}),
		feeds: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cumuru_worker_calendar_sync_feeds_total",
			Help: "Feeds de calendário sincronizados.",
		}),
	}
}
