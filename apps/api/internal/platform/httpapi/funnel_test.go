package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

type funnelReaderStub struct {
	err    error
	window string
}

func (f *funnelReaderStub) Funnel(
	_ context.Context,
	window string,
) (analytics.Funnel, error) {
	f.window = window
	return analytics.Funnel{Window: window}, f.err
}

func funnelHandler(t *testing.T, stub *funnelReaderStub) http.Handler {
	t.Helper()
	verifier, err := access.NewDevelopmentFake("test", "https://issuer.invalid")
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := mustNew(t, Dependencies{Verifier: verifier, AnalyticsFunnel: stub})
	return handler
}

func funnelRequest(target, token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.RemoteAddr = "127.0.0.1:4312"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func TestAnalyticsFunnelRequiresTheInternalScopeAndNoStore(t *testing.T) {
	t.Parallel()

	handler := funnelHandler(t, &funnelReaderStub{})
	target := "/api/v1/analytics/funnel?window=last_30_days"

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, funnelRequest(target, ""))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d", anonymous.Code)
	}

	// stays:read:own abre a área da hospedagem; o funil é da administração.
	wrongScope := httptest.NewRecorder()
	handler.ServeHTTP(
		wrongScope, funnelRequest(target, access.DevelopmentPlatformToken),
	)
	if wrongScope.Code != http.StatusForbidden {
		t.Fatalf("wrong scope status = %d", wrongScope.Code)
	}

	authorized := httptest.NewRecorder()
	handler.ServeHTTP(
		authorized,
		funnelRequest(target, access.DevelopmentAnalyticsQualityToken),
	)
	if authorized.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", authorized.Code, authorized.Body.String())
	}
	if authorized.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", authorized.Header().Get("Cache-Control"))
	}
}

func TestAnalyticsFunnelKeepsTheWindowSelectorClosed(t *testing.T) {
	t.Parallel()

	handler := funnelHandler(t, &funnelReaderStub{})
	for _, target := range []string{
		"/api/v1/analytics/funnel",
		"/api/v1/analytics/funnel?window=last_90_days",
		"/api/v1/analytics/funnel?window=last_30_days&stage=invite",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			funnelRequest(target, access.DevelopmentAnalyticsQualityToken),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", target, recorder.Code)
		}
	}
}

func TestAnalyticsFunnelHidesTheFailureDetail(t *testing.T) {
	t.Parallel()

	handler := funnelHandler(
		t, &funnelReaderStub{err: errors.New("sql: secret detail")},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		funnelRequest(
			"/api/v1/analytics/funnel?window=last_30_days",
			access.DevelopmentAnalyticsQualityToken,
		),
	)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret detail") {
		t.Fatal("the transport detail must not reach the client")
	}
}
