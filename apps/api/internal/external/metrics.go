package external

import "github.com/prometheus/client_golang/prometheus"

// Metrics are labelled by source_code and outcome, so a single dead source is
// visible as itself and not as a drop in an aggregate that also contains the
// healthy ones. No label carries a URL, a query or anything from a response.
type Metrics struct {
	runs      *prometheus.CounterVec
	written   *prometheus.CounterVec
	skipped   *prometheus.CounterVec
	retention prometheus.Counter
}

func NewMetrics() *Metrics {
	return &Metrics{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cumuru",
			Subsystem: "worker",
			Name:      "external_fetch_runs_total",
			Help:      "Total de ciclos de ingestão externa por fonte e resultado.",
		}, []string{"source_code", "outcome"}),
		written: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cumuru",
			Subsystem: "worker",
			Name:      "external_observations_written_total",
			Help:      "Total de observações externas gravadas por fonte.",
		}, []string{"source_code"}),
		skipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cumuru",
			Subsystem: "worker",
			Name:      "external_fetch_skipped_total",
			Help:      "Total de coletas externas puladas por fonte e motivo.",
		}, []string{"source_code", "reason"}),
		retention: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cumuru",
			Subsystem: "worker",
			Name:      "external_observations_expired_total",
			Help:      "Total de observações externas removidas por retenção.",
		}),
	}
}

func (m *Metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.runs, m.written, m.skipped, m.retention,
	}
}
