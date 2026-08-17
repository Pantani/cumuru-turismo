package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"
)

type stubAuthenticator struct {
	grant         store.SessionGrant
	authError     error
	describeError error
	revoked       [][]byte
	revokeError   error
	rotations     []passwordRotation
	rotateError   error
}

type passwordRotation struct {
	current string
	next    string
}

func (s *stubAuthenticator) Authenticate(
	_ context.Context, _ string, _ string,
) (store.SessionGrant, error) {
	if s.authError != nil {
		return store.SessionGrant{}, s.authError
	}
	return s.grant, nil
}

func (s *stubAuthenticator) DescribeSession(
	_ context.Context, _ []byte,
) (store.SessionGrant, error) {
	if s.describeError != nil {
		return store.SessionGrant{}, s.describeError
	}
	return s.grant, nil
}

func (s *stubAuthenticator) RevokeSession(_ context.Context, digest []byte) error {
	s.revoked = append(s.revoked, digest)
	return s.revokeError
}

func (s *stubAuthenticator) RotatePassword(
	_ context.Context, _ []byte, current string, next string,
) error {
	s.rotations = append(s.rotations, passwordRotation{current: current, next: next})
	return s.rotateError
}

func demoGrant() store.SessionGrant {
	return store.SessionGrant{
		Token:       access.SessionTokenPrefix + "fixture-session-token-value",
		ExpiresAt:   time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
		AccountID:   "019fae14-0000-7000-8000-000000000001",
		Email:       "operador@cumuru.local",
		DisplayName: "Operadora fictícia da hospedagem",
		Scopes:      []string{"stays:read:own", "stays:write"},
	}
}

func newAuthHandler(t *testing.T, authenticator httpapi.Authenticator) http.Handler {
	t.Helper()
	handler, _, err := httpapi.New(httpapi.Dependencies{
		Readiness:       readinessFunc(func(context.Context) error { return nil }),
		Verifier:        validVerifier(),
		Auth:            authenticator,
		LockoutDuration: 15 * time.Minute,
		Logger:          slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
		Registry:        prometheus.NewRegistry(),
		Tracer:          noop.NewTracerProvider().Tracer("test"),
		Build: httpapi.BuildInfo{
			Version:  "0.2.0",
			Revision: testBuildRevision,
			BuiltAt:  time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return handler
}

func postLogin(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/auth/login", strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestLoginReturnsSession(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{grant: demoGrant()})
	recorder := postLogin(
		handler, `{"email":"operador@cumuru.local","password":"fixture-fixture-"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var payload struct {
		Token   string `json:"token"`
		Account struct {
			Email  string   `json:"email"`
			Scopes []string `json:"scopes"`
		} `json:"account"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.HasPrefix(payload.Token, access.SessionTokenPrefix) {
		t.Fatalf("token = %q, want the session prefix", payload.Token)
	}
	if payload.Account.Email != "operador@cumuru.local" || len(payload.Account.Scopes) != 2 {
		t.Fatalf("account = %+v, want the seeded operator", payload.Account)
	}
}

// A rejected credential must not disclose whether the e-mail exists, so the
// body carries the generic problem and no account detail.
func TestLoginRejectsWithoutDisclosingTheAccount(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{authError: store.ErrAuthRejected})
	recorder := postLogin(
		handler, `{"email":"desconhecido@cumuru.local","password":"fixture-fixture-"}`,
	)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if strings.Contains(recorder.Body.String(), "desconhecido") {
		t.Fatalf("body echoed the submitted e-mail: %s", recorder.Body.String())
	}
}

func TestLoginSignalsLockoutWithRetryAfter(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{authError: store.ErrAuthLocked})
	recorder := postLogin(
		handler, `{"email":"operador@cumuru.local","password":"fixture-fixture-"}`,
	)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if got := recorder.Header().Get("Retry-After"); got != "900" {
		t.Fatalf("Retry-After = %q, want 900", got)
	}
}

func TestLoginRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{grant: demoGrant()})
	recorder := postLogin(
		handler,
		`{"email":"operador@cumuru.local","password":"fixture-fixture-","role":"admin"}`,
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLogoutRevokesAndIsIdempotent(t *testing.T) {
	t.Parallel()
	authenticator := &stubAuthenticator{grant: demoGrant()}
	handler := newAuthHandler(t, authenticator)
	token, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
	}
	if len(authenticator.revoked) != 2 {
		t.Fatalf("revoked calls = %d, want 2", len(authenticator.revoked))
	}
}

func TestLogoutRequiresACredential(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{grant: demoGrant()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSessionRejectsAForeignCredential(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{grant: demoGrant()})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.Header.Set("Authorization", "Bearer valid")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSessionEchoesThePresentedToken(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, &stubAuthenticator{grant: demoGrant()})
	token, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload.Token != token {
		t.Fatalf("token = %q, want the presented token", payload.Token)
	}
}

// Without an Authenticator the routes must stay absent rather than answering
// with a misleading 401, so an OIDC-only deployment has no local login surface.
func TestAuthRoutesAreAbsentWhenDisabled(t *testing.T) {
	t.Parallel()
	handler := newAuthHandler(t, nil)
	recorder := postLogin(handler, `{"email":"a@b.co","password":"fixture-fixture-"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func postRotation(
	handler http.Handler,
	token string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/auth/password", strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestRotatePasswordAnswersWithoutBody(t *testing.T) {
	t.Parallel()
	authenticator := &stubAuthenticator{grant: demoGrant()}
	handler := newAuthHandler(t, authenticator)
	token, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	recorder := postRotation(
		handler,
		token,
		`{"current_password":"senha-provisoria","new_password":"senha-definitiva"}`,
	)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
	if len(authenticator.rotations) != 1 {
		t.Fatalf("rotations = %d, want 1", len(authenticator.rotations))
	}
	if authenticator.rotations[0].next != "senha-definitiva" {
		t.Fatalf("next = %q", authenticator.rotations[0].next)
	}
}

// A rejected new secret and a wrong current secret must not share a status, so
// the operator can tell a weak password from a typo.
func TestRotatePasswordSeparatesRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		failure    error
		wantStatus int
	}{
		{
			name:       "reused secret",
			failure:    store.ErrPasswordReused,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "secret below the policy",
			failure:    access.ErrInvalidPassword,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "wrong current secret",
			failure:    store.ErrAuthRejected,
			wantStatus: http.StatusUnauthorized,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := newAuthHandler(t, &stubAuthenticator{
				grant: demoGrant(), rotateError: test.failure,
			})
			token, _, err := access.NewSessionToken()
			if err != nil {
				t.Fatalf("NewSessionToken() error = %v", err)
			}
			recorder := postRotation(
				handler,
				token,
				`{"current_password":"senha-provisoria","new_password":"senha-definitiva"}`,
			)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}

func TestRotatePasswordRequiresACredential(t *testing.T) {
	t.Parallel()
	authenticator := &stubAuthenticator{grant: demoGrant()}
	handler := newAuthHandler(t, authenticator)
	recorder := postRotation(
		handler,
		"",
		`{"current_password":"senha-provisoria","new_password":"senha-definitiva"}`,
	)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if len(authenticator.rotations) != 0 {
		t.Fatalf("rotations = %d, want none", len(authenticator.rotations))
	}
}

// The flag is what routes the client to the rotation screen, so it has to
// survive the transport.
func TestLoginExposesTheRotationRequirement(t *testing.T) {
	t.Parallel()
	grant := demoGrant()
	grant.MustChangePassword = true
	handler := newAuthHandler(t, &stubAuthenticator{grant: grant})
	recorder := postLogin(
		handler, `{"email":"a@b.co","password":"fixture-fixture-"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload struct {
		Account struct {
			MustChangePassword bool `json:"must_change_password"`
		} `json:"account"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !payload.Account.MustChangePassword {
		t.Fatal("must_change_password = false, want true")
	}
}
