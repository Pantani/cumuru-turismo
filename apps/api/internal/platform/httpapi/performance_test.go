package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

type ownPerformanceStub struct {
	err   error
	slice analytics.PresenceSlice
}

func (f *ownPerformanceStub) Performance(
	_ context.Context,
	query analytics.OwnPerformanceQuery,
) (analytics.OwnPerformance, error) {
	f.slice = query.Slice
	return analytics.OwnPerformance{
		Window:     query.Slice.Window,
		Comparison: analytics.ComparisonAvailability{Status: analytics.ComparisonAvailable},
	}, f.err
}

const performancePath = "/api/v1/accommodations/" +
	"019f0000-0000-7000-8000-000000000001/performance"

func performanceHandler(t *testing.T, stub *ownPerformanceStub) http.Handler {
	t.Helper()
	verifier, err := access.NewDevelopmentFake("test", "https://issuer.invalid")
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := mustNew(t, Dependencies{Verifier: verifier, OwnPerformance: stub})
	return handler
}

func performanceRequest(target, token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.RemoteAddr = "127.0.0.1:4312"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func TestAccommodationPerformanceRequiresTheOwnReadScope(t *testing.T) {
	t.Parallel()

	handler := performanceHandler(t, &ownPerformanceStub{})
	target := performancePath + "?window=recent_90_days"

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, performanceRequest(target, ""))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d", anonymous.Code)
	}

	// O token de qualidade carrega analytics:read:internal e nada mais: ler o
	// painel interno não abre o comparativo de uma hospedagem.
	wrongScope := httptest.NewRecorder()
	handler.ServeHTTP(
		wrongScope,
		performanceRequest(target, access.DevelopmentAnalyticsQualityToken),
	)
	if wrongScope.Code != http.StatusForbidden {
		t.Fatalf("wrong scope status = %d", wrongScope.Code)
	}
}

func TestAccommodationPerformanceIsNeverCachedBySharedCaches(t *testing.T) {
	t.Parallel()

	handler := performanceHandler(t, &ownPerformanceStub{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		performanceRequest(
			performancePath+"?window=recent_90_days",
			access.DevelopmentPlatformToken,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
	if recorder.Header().Get("ETag") != "" {
		t.Fatal("tenant data must not carry a shared validator")
	}
}

func TestAccommodationPerformanceRejectsSelectorsOutsideTheObservedSeries(t *testing.T) {
	t.Parallel()

	handler := performanceHandler(t, &ownPerformanceStub{})
	cases := []string{
		// A previsão é publicada como intervalo e não tem contraparte própria.
		performancePath + "?window=next_30_days",
		performancePath + "?window=custom",
		performancePath + "?window=recent_90_days&dimension=hotel",
		performancePath + "?window=recent_90_days&month=2026-05",
		performancePath,
	}
	for _, target := range cases {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder, performanceRequest(target, access.DevelopmentPlatformToken),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", target, recorder.Code)
		}
	}
}

func TestAccommodationPerformanceAnswersNotFoundWithoutMembership(t *testing.T) {
	t.Parallel()

	handler := performanceHandler(
		t, &ownPerformanceStub{err: analytics.ErrOwnPerformanceNotFound},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		performanceRequest(
			performancePath+"?window=recent_90_days",
			access.DevelopmentPlatformToken,
		),
	)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAccommodationPerformanceAcceptsTheCivilMonthWindow(t *testing.T) {
	t.Parallel()

	stub := &ownPerformanceStub{}
	handler := performanceHandler(t, stub)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		performanceRequest(
			performancePath+"?window=month&month=2026-05",
			access.DevelopmentPlatformToken,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.slice.Month != "2026-05" {
		t.Fatalf("month = %q, want 2026-05", stub.slice.Month)
	}
}
