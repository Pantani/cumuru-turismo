package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

func TestAnalyticsRoutesAreAbsentWithoutAnalyticsDependencies(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/public/summary", nil),
	)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestOptionsRoutesUseNormalizedRecoveryAndMetricsChain(t *testing.T) {
	t.Parallel()

	handler, operations := mustNew(t, Dependencies{
		Stays:              stay.NewService(nil),
		CORSAllowedOrigins: []string{"https://example.invalid"},
		CursorKeys:         testCursorKeys(),
	})
	request := httptest.NewRequest(
		http.MethodOptions,
		"/api/v1/invites/test-capability",
		nil,
	)
	request.Header.Set("Origin", "https://example.invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", recorder.Code)
	}
	metrics := httptest.NewRecorder()
	operations.ServeHTTP(
		metrics,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	want := `cumuru_http_requests_total{method="OPTIONS",route="/api/v1/invites/{token}",status="204"} 1`
	if !strings.Contains(metrics.Body.String(), want) {
		t.Fatalf("OPTIONS normalized metric missing: %s", metrics.Body)
	}
}
