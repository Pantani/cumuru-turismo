package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const recoveryTestRoute = "/api/v1/platform/recovery-test"

func TestBufferedRecoveryDiscardsPartialResponseAndObservesOne500(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	dependencies := Dependencies{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	}
	handler, registry := recoveryTestHandler(t, dependencies, http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "/private-success")
			writer.Header().Set("ETag", `"7"`)
			writer.Header().Set("Content-Length", "2048")
			writer.Header().Set("Idempotency-Replayed", "false")
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte("partial-success-canary"))
			panic("private-panic-canary")
		},
	))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, recoveryTestRoute, nil),
	)

	assertCleanInternalProblem(t, recorder)
	for _, header := range []string{
		"Location", "ETag", "Content-Length", "Idempotency-Replayed",
	} {
		if got := recorder.Header().Get(header); got != "" {
			t.Fatalf("%s = %q after panic", header, got)
		}
	}
	for _, canary := range []string{"partial-success-canary", "private-panic-canary"} {
		if strings.Contains(recorder.Body.String(), canary) ||
			strings.Contains(logs.String(), canary) {
			t.Fatalf("panic path leaked %q", canary)
		}
	}
	if got := strings.Count(logs.String(), `"error_code":"request_panic"`); got != 1 {
		t.Fatalf("request_panic logs = %d; logs=%s", got, logs.String())
	}
	if got := strings.Count(logs.String(), `"msg":"http request"`); got != 1 {
		t.Fatalf("normalized request logs = %d; logs=%s", got, logs.String())
	}
	assertHTTPMetricCount(t, registry, "cumuru_http_requests_total", "500", 1)
	assertHTTPMetricCount(t, registry, "cumuru_http_request_duration_seconds", "500", 1)
}

func TestBufferedRecoveryHandlesPanicBeforeWrite(t *testing.T) {
	t.Parallel()

	handler, _ := recoveryTestHandler(t, Dependencies{}, http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("panic-before-write-canary")
		},
	))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, recoveryTestRoute, nil),
	)

	assertCleanInternalProblem(t, recorder)
	if strings.Contains(recorder.Body.String(), "panic-before-write-canary") {
		t.Fatal("panic value leaked")
	}
}

func TestBufferedRecoveryFailsClosedOnOverflowEvenWhenWriteErrorIgnored(t *testing.T) {
	t.Parallel()

	handler, registry := recoveryTestHandler(t, Dependencies{}, http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "/oversized-success")
			writer.WriteHeader(http.StatusOK)
			_, _ = io.CopyN(writer, strings.NewReader(
				strings.Repeat("x", maxBufferedResponseBytes+1),
			), maxBufferedResponseBytes+1)
		},
	))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, recoveryTestRoute, nil),
	)

	assertCleanInternalProblem(t, recorder)
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q after overflow", got)
	}
	assertHTTPMetricCount(t, registry, "cumuru_http_requests_total", "500", 1)
}

func recoveryTestHandler(
	t *testing.T,
	dependencies Dependencies,
	next http.Handler,
) (http.Handler, *prometheus.Registry) {
	t.Helper()
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.DiscardHandler)
	}
	registry := prometheus.NewRegistry()
	metrics := newHTTPMetrics(registry)
	route := metrics.instrument(
		recoveryTestRoute,
		dependencies.observe(
			recoveryTestRoute,
			dependencies.recoverBuffered(next),
		),
	)
	return dependencies.withRequestID(dependencies.recoverPanic(route)), registry
}

func assertCleanInternalProblem(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("X-Request-ID is empty")
	}
	if !strings.Contains(recorder.Body.String(), `"status":500`) {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func assertHTTPMetricCount(
	t *testing.T,
	registry *prometheus.Registry,
	name string,
	status string,
	want uint64,
) {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if metricLabel(metric, "status") != status {
				continue
			}
			assertHTTPMetricValue(t, name, metric, want)
			return
		}
	}
	t.Fatalf("%s{status=%q} not found", name, status)
}

func assertHTTPMetricValue(
	t *testing.T,
	name string,
	metric *dto.Metric,
	want uint64,
) {
	t.Helper()
	if metric.Counter != nil {
		if metric.Counter.GetValue() != float64(want) {
			t.Fatalf("%s = %v, want %d", name, metric.Counter.GetValue(), want)
		}
		return
	}
	if metric.Histogram == nil {
		t.Fatalf("%s has unsupported metric type", name)
	}
	if metric.Histogram.GetSampleCount() != want {
		t.Fatalf("%s count = %d, want %d", name, metric.Histogram.GetSampleCount(), want)
	}
}

func metricLabel(metric *dto.Metric, name string) string {
	for _, label := range metric.Label {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}
