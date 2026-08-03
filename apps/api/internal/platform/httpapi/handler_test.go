package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/prometheus/client_golang/prometheus"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

type verifierFunc func(context.Context, string) (access.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token string) (access.Principal, error) {
	return f(ctx, token)
}

type readinessFunc func(context.Context) error

func (f readinessFunc) CheckReadiness(ctx context.Context) error {
	return f(ctx)
}

const testBuildRevision = "source-9c521fed28d7e090cad55609e59e1a4aa2e2225ba7d4d27a96835917b32f5e58"

type endpointExpectation struct {
	name       string
	path       string
	auth       string
	wantStatus int
	wantType   string
	wantBody   string
}

func TestOperationalEndpoints(t *testing.T) {
	t.Parallel()

	handler, _ := newHandlers(t, readinessFunc(func(context.Context) error { return nil }), validVerifier(), nil)

	tests := []endpointExpectation{
		{"health", "/api/v1/platform/health", "", http.StatusOK, "application/json", `{"status":"ok"}`},
		{"readiness", "/api/v1/platform/readiness", "", http.StatusOK, "application/json", `{"status":"ready"}`},
		{"build", "/api/v1/platform/build", "Bearer valid", http.StatusOK, "application/json", `"revision":"` + testBuildRevision + `"`},
		{"build missing token", "/api/v1/platform/build", "", http.StatusUnauthorized, "application/problem+json", `"status":401`},
		{"build malformed scheme", "/api/v1/platform/build", "Basic abc", http.StatusUnauthorized, "application/problem+json", `"status":401`},
		{"build invalid token", "/api/v1/platform/build", "Bearer invalid", http.StatusUnauthorized, "application/problem+json", `"status":401`},
		{"build missing scope", "/api/v1/platform/build", "Bearer no-scope", http.StatusForbidden, "application/problem+json", `"status":403`},
		{"future route absent", "/api/v1/stays", "Bearer valid", http.StatusNotFound, "text/plain", "404 page not found"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertOperationalEndpoint(t, handler, tt)
		})
	}
}

func TestDevelopmentFixtureCredentialRequiresResolvedLoopback(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatal(err)
	}
	trusted := netip.MustParsePrefix("172.30.0.10/32")
	handler, _ := httpapi.New(httpapi.Dependencies{
		Verifier:          verifier,
		TrustedProxyCIDRs: []netip.Prefix{trusted},
		Logger:            slog.New(slog.DiscardHandler),
		Registry:          prometheus.NewRegistry(),
		Tracer:            noop.NewTracerProvider().Tracer("test"),
		Build: httpapi.BuildInfo{
			Version: "0.2.0", Revision: testBuildRevision,
			BuiltAt: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		},
	})
	tests := []fixtureCredentialExpectation{
		{name: "direct IPv4 loopback", remote: "127.0.0.1:4312", want: http.StatusOK},
		{name: "direct IPv6 loopback", remote: "[::1]:4312", want: http.StatusOK},
		{name: "direct remote", remote: "203.0.113.9:4312", want: http.StatusUnauthorized},
		{
			name: "untrusted spoofed loopback", remote: "203.0.113.9:4312",
			forwarded: "127.0.0.1", realIP: "127.0.0.1", want: http.StatusUnauthorized,
		},
		{
			name: "trusted proxy loopback", remote: "172.30.0.10:4312",
			forwarded: "127.0.0.1", realIP: "127.0.0.1", want: http.StatusOK,
		},
		{
			name: "trusted proxy real IP only", remote: "172.30.0.10:4312",
			realIP: "::1", want: http.StatusOK,
		},
		{
			name: "trusted proxy remote", remote: "172.30.0.10:4312",
			forwarded: "198.51.100.44", realIP: "198.51.100.44",
			want: http.StatusUnauthorized,
		},
		{
			name: "trusted proxy disagreement", remote: "172.30.0.10:4312",
			forwarded: "127.0.0.1", realIP: "198.51.100.44",
			want: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertFixtureCredentialExpectation(t, handler, tt)
		})
	}
}

type fixtureCredentialExpectation struct {
	name      string
	remote    string
	forwarded string
	realIP    string
	want      int
}

func assertFixtureCredentialExpectation(
	t *testing.T,
	handler http.Handler,
	want fixtureCredentialExpectation,
) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/build", nil)
	request.RemoteAddr = want.remote
	request.Header.Set("Authorization", "Bearer "+access.DevelopmentPlatformToken)
	if want.forwarded != "" {
		request.Header.Set("X-Forwarded-For", want.forwarded)
	}
	if want.realIP != "" {
		request.Header.Set("X-Real-IP", want.realIP)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != want.want {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, want.want, recorder.Body)
	}
}

func assertOperationalEndpoint(t *testing.T, handler http.Handler, want endpointExpectation) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, want.path, nil)
	request.Header.Set("Authorization", want.auth)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != want.wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, want.wantStatus, recorder.Body)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, want.wantType) {
		t.Fatalf("Content-Type = %q, want prefix %q", got, want.wantType)
	}
	if !strings.Contains(strings.TrimSpace(recorder.Body.String()), want.wantBody) {
		t.Fatalf("body = %q, want substring %q", recorder.Body, want.wantBody)
	}
	if strings.HasPrefix(want.path, "/api/v1/platform/") {
		assertOperationalHeaders(t, recorder)
	}
}

func assertOperationalHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := recorder.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestReadinessFailureIsSanitized(t *testing.T) {
	t.Parallel()

	const secret = "postgres://user:private-password@database.internal/cumuru"
	handler, _ := newHandlers(t, readinessFunc(func(context.Context) error {
		return errors.New(secret)
	}), validVerifier(), nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/platform/readiness", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), secret) || strings.Contains(recorder.Body.String(), "private-password") {
		t.Fatalf("response leaked dependency error: %s", recorder.Body)
	}
}

func TestLogsMetricsAndResponsesDoNotLeakRequestCanaries(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler, operations := newHandlers(t, readinessFunc(func(context.Context) error { return nil }), validVerifier(), logger)
	const token = "token-canary-483672"
	const cookie = "cookie-canary-195044"
	const query = "query-canary-990381"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/build?debug="+query, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Cookie", "session="+cookie)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	for _, canary := range []string{token, cookie, query} {
		if strings.Contains(logs.String(), canary) {
			t.Fatalf("logs leaked %q: %s", canary, logs.String())
		}
	}

	metrics := httptest.NewRecorder()
	operations.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, canary := range []string{token, cookie, query} {
		if strings.Contains(metrics.Body.String(), canary) {
			t.Fatalf("metrics leaked %q", canary)
		}
	}
	if !strings.Contains(metrics.Body.String(), `route="/api/v1/platform/build"`) {
		t.Fatalf("metrics lack normalized route: %s", metrics.Body)
	}
}

func TestRequestIDRejectsUnsafeClientValue(t *testing.T) {
	t.Parallel()

	handler, _ := newHandlers(t, readinessFunc(func(context.Context) error { return nil }), validVerifier(), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/health", nil)
	request.Header.Set("X-Request-ID", "unsafe\nvalue")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got == "" || strings.Contains(got, "unsafe") {
		t.Fatalf("X-Request-ID = %q, want generated safe value", got)
	}
}

func TestTraceUsesOnlyNormalizedAllowlistedAttributes(t *testing.T) {
	t.Parallel()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})
	handler, _ := httpapi.New(httpapi.Dependencies{
		Readiness: readinessFunc(func(context.Context) error { return nil }),
		Verifier:  validVerifier(),
		Logger:    slog.New(slog.DiscardHandler),
		Registry:  prometheus.NewRegistry(),
		Tracer:    provider.Tracer("test"),
		Build: httpapi.BuildInfo{
			Version:  "0.2.0",
			Revision: testBuildRevision,
			BuiltAt:  time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		},
	})
	const queryCanary = "trace-query-canary"
	const tokenCanary = "trace-token-canary"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/build?value="+queryCanary, nil)
	request.Header.Set("Authorization", "Bearer "+tokenCanary)
	recorderHTTP := httptest.NewRecorder()

	handler.ServeHTTP(recorderHTTP, request)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	serialized, err := json.Marshal(spans[0].Attributes())
	if err != nil {
		t.Fatalf("marshal span attributes: %v", err)
	}
	if strings.Contains(string(serialized), queryCanary) || strings.Contains(string(serialized), tokenCanary) {
		t.Fatalf("trace leaked request canary: %s", serialized)
	}
	if spans[0].Name() != "/api/v1/platform/build" {
		t.Fatalf("span name = %q", spans[0].Name())
	}
}

func TestProblemSchemaIsClosedByConstruction(t *testing.T) {
	t.Parallel()

	handler, _ := newHandlers(t, readinessFunc(func(context.Context) error { return nil }), validVerifier(), nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/platform/build", nil))

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	allowed := map[string]bool{
		"type": true, "title": true, "status": true, "detail": true, "request_id": true,
	}
	for key := range body {
		if !allowed[key] {
			t.Fatalf("unexpected Problem field %q", key)
		}
	}
	if _, ok := body["instance"]; ok {
		t.Fatal("Problem must not expose a raw request path")
	}
}

func newHandlers(t *testing.T, readiness httpapi.ReadinessChecker, verifier access.Verifier, logger *slog.Logger) (http.Handler, http.Handler) {
	t.Helper()
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	}
	return httpapi.New(httpapi.Dependencies{
		Readiness: readiness,
		Verifier:  verifier,
		Logger:    logger,
		Registry:  prometheus.NewRegistry(),
		Tracer:    noop.NewTracerProvider().Tracer("test"),
		Build: httpapi.BuildInfo{
			Version:  "0.2.0",
			Revision: testBuildRevision,
			BuiltAt:  time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		},
	})
}

func validVerifier() access.Verifier {
	return verifierFunc(func(_ context.Context, token string) (access.Principal, error) {
		switch token {
		case "valid":
			return access.NewPrincipal("https://issuer.invalid", "subject", []string{"platform:read"}), nil
		case "no-scope":
			return access.NewPrincipal("https://issuer.invalid", "subject", nil), nil
		default:
			return access.Principal{}, access.ErrInvalidToken
		}
	})
}
