package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

type analyticsReaderStub struct {
	err         error
	methodology analytics.PublicMethodology
}

func (f analyticsReaderStub) Summary(context.Context) (analytics.PublicSummary, error) {
	return analytics.PublicSummary{}, f.err
}

func (f analyticsReaderStub) Presence(
	context.Context,
	string,
) (analytics.PublicPresence, error) {
	return analytics.PublicPresence{Window: "recent_30_days"}, f.err
}

func (f analyticsReaderStub) Preferences(
	context.Context,
	string,
) (analytics.PublicPreferences, error) {
	return analytics.PublicPreferences{Period: "last_complete_month"}, f.err
}

func (f analyticsReaderStub) Methodology(context.Context) (analytics.PublicMethodology, error) {
	return f.methodology, f.err
}

type qualityReaderStub struct {
	err error
}

type analyticsVerifierStub struct {
	principal access.Principal
	err       error
}

func (f analyticsVerifierStub) Verify(
	context.Context,
	string,
) (access.Principal, error) {
	return f.principal, f.err
}

func (f qualityReaderStub) Quality(context.Context, string) (analytics.QualitySnapshot, error) {
	return analytics.QualitySnapshot{Window: "last_30_days"}, f.err
}

func TestPublicAnalyticsUsesStrongVariantETagAndCache(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{PublicAnalytics: analyticsReaderStub{}})
	first := httptest.NewRecorder()
	handler.ServeHTTP(
		first,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/public/presence?window=recent_30_days",
			nil,
		),
	)
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if !strongAnalyticsETag.MatchString(etag) {
		t.Fatalf("ETag = %q", etag)
	}
	if first.Header().Get("Cache-Control") != publicAnalyticsCache {
		t.Fatalf("Cache-Control = %q", first.Header().Get("Cache-Control"))
	}

	second := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/public/presence?window=recent_30_days",
		nil,
	)
	request.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(second, request)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", second.Code, second.Body.String())
	}
}

func TestPublicMethodologyEmitsFallbackForecastBounds(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{PublicAnalytics: analyticsReaderStub{
		methodology: analytics.PublicMethodology{
			ForecastBoundsPercent:         []int32{85, 115},
			ForecastFallbackBoundsPercent: []int32{70, 130},
		},
	}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/public/methodology", nil),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"forecast_bounds_percent":[85,115]`) ||
		!strings.Contains(body, `"forecast_fallback_bounds_percent":[70,130]`) {
		t.Fatalf("methodology body = %s", body)
	}
}

func TestPublicAnalyticsRejectsOpenEndedSelectorsAndInvalidETag(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{PublicAnalytics: analyticsReaderStub{}})
	cases := []string{
		"/api/v1/public/presence?window=custom",
		"/api/v1/public/presence?window=recent_30_days&dimension=hotel",
	}
	for _, target := range cases {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d", target, recorder.Code)
		}
		if recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s Cache-Control = %q", target, recorder.Header().Get("Cache-Control"))
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/summary", nil)
	request.Header.Set("If-None-Match", `"not-a-valid-tag"`)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid ETag status = %d", recorder.Code)
	}
}

func TestPublicAnalyticsUnavailableIsSanitizedAndNotCached(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicAnalytics: analyticsReaderStub{err: errors.New("sql: secret detail")},
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/public/summary", nil),
	)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("body leaked internal error: %s", recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
}

func TestAnalyticsQualityRequiresInternalScopeAndNoStore(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://issuer.invalid")
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := mustNew(t, Dependencies{
		Verifier:         verifier,
		AnalyticsQuality: qualityReaderStub{},
	})
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(
		unauthorized,
		httptest.NewRequest(
			http.MethodGet, "/api/v1/analytics/quality?window=last_30_days", nil,
		),
	)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet, "/api/v1/analytics/quality?window=last_30_days", nil,
	)
	request.RemoteAddr = "127.0.0.1:4312"
	request.Header.Set(
		"Authorization",
		"Bearer "+access.DevelopmentAnalyticsQualityToken,
	)
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d body=%s", authorized.Code, authorized.Body.String())
	}
	if authorized.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", authorized.Header().Get("Cache-Control"))
	}

	forbiddenHandler, _ := mustNew(t, Dependencies{
		Verifier:         verifier,
		AnalyticsQuality: qualityReaderStub{},
	})
	forbidden := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(
		http.MethodGet, "/api/v1/analytics/quality?window=last_30_days", nil,
	)
	forbiddenRequest.RemoteAddr = "127.0.0.1:4312"
	forbiddenRequest.Header.Set(
		"Authorization",
		"Bearer "+access.DevelopmentPlatformToken,
	)
	forbiddenHandler.ServeHTTP(forbidden, forbiddenRequest)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d", forbidden.Code)
	}
}

func TestAnalyticsAuthenticationNeverLogsBearerToken(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	handler, _ := mustNew(t, Dependencies{
		Verifier:         analyticsVerifierStub{err: access.ErrInvalidToken},
		AnalyticsQuality: qualityReaderStub{},
		Logger:           slog.New(slog.NewJSONHandler(&output, nil)),
	})
	request := httptest.NewRequest(
		http.MethodGet, "/api/v1/analytics/quality?window=last_30_days", nil,
	)
	request.Header.Set("Authorization", "Bearer private-token-canary")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if strings.Contains(output.String(), "private-token-canary") {
		t.Fatalf("bearer token leaked to logs: %s", output.String())
	}
}
