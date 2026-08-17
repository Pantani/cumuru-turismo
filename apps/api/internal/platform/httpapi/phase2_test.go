package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

const onboardingTokenCanary = "onboard-token-canary"

type accommodationRepositoryStub struct {
	accommodation.Repository
	createResult accommodation.Accommodation
	createReplay bool
	createErr    error
	createInput  *accommodation.CreateCommand
}

func (s accommodationRepositoryStub) Create(
	_ context.Context,
	command accommodation.CreateCommand,
) (accommodation.Accommodation, bool, error) {
	if s.createInput != nil {
		*s.createInput = command
	}
	return s.createResult, s.createReplay, s.createErr
}

func (accommodationRepositoryStub) CreateMembership(
	context.Context,
	accommodation.CreateMembershipCommand,
) (accommodation.MembershipCreated, bool, error) {
	return accommodation.MembershipCreated{
		ID:              uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		Role:            accommodation.RoleOperator, Active: true, Version: 1,
		CreatedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
	}, false, nil
}

type stayRepositoryStub struct {
	stay.Repository
	createResult     stay.MutationResult
	createErr        error
	transitionResult stay.MutationResult
	transitionErr    error
	submitResult     stay.SubmissionAccepted
	submitErr        error
	getInviteCalls   *atomic.Int32
	submitCalls      *atomic.Int32
}

func (s stayRepositoryStub) Create(
	context.Context,
	stay.CreateCommand,
) (stay.MutationResult, bool, error) {
	return s.createResult, false, s.createErr
}

func (s stayRepositoryStub) Transition(
	context.Context,
	stay.TransitionCommand,
) (stay.MutationResult, bool, error) {
	return s.transitionResult, false, s.transitionErr
}

func (s stayRepositoryStub) SubmitInviteGroup(
	context.Context,
	stay.InviteGroupCommand,
) (stay.SubmissionAccepted, bool, error) {
	if s.submitCalls != nil {
		s.submitCalls.Add(1)
	}
	return s.submitResult, false, s.submitErr
}

func (s stayRepositoryStub) GetInvite(
	context.Context,
	stay.InviteRequest,
) (stay.InviteContext, error) {
	if s.getInviteCalls != nil {
		s.getInviteCalls.Add(1)
	}
	return stay.InviteContext{
		AccommodationName: "Pousada fictícia",
		PlannedArrivalOn:  "2026-08-01", PlannedDepartureOn: "2026-08-02",
		ExpectedGuestCount: 1, PrivacyNoticeVersion: "v1",
	}, nil
}

func TestMembershipCreationResponseOmitsTargetIdentity(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t)
	body := `{"oidc_issuer":"https://target.invalid","oidc_subject":"private-subject","role":"operator"}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/accommodations/019f0000-0000-7000-8000-000000000001/memberships",
		strings.NewReader(body),
	)
	request.Header.Set("Authorization", "Bearer manager")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "membership-key-1234")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if strings.Contains(recorder.Body.String(), "private-subject") ||
		strings.Contains(recorder.Body.String(), "target.invalid") {
		t.Fatalf("response leaked target identity: %s", recorder.Body)
	}
}

func assertAccommodationOnboardingResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	id uuid.UUID,
) {
	t.Helper()
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if recorder.Header().Get("Location") != "/api/v1/accommodations/"+id.String() {
		t.Fatalf("Location = %q", recorder.Header().Get("Location"))
	}
	if recorder.Header().Get("ETag") != `"1"` ||
		recorder.Header().Get("Idempotency-Replayed") != "false" {
		t.Fatalf("mutation headers = %v", recorder.Header())
	}
}

func TestAccommodationOnboardingReturnsContractHeaders(t *testing.T) {
	t.Parallel()

	var captured accommodation.CreateCommand
	var logs bytes.Buffer
	id := uuid.MustParse("019f0000-0000-7000-8000-000000000021")
	organizationID := uuid.MustParse("019f0000-0000-7000-8000-000000000022")
	handler := phase2HandlerWithOptions(t, phase2HandlerOptions{
		onboardingEnabled: true,
		logger:            slog.New(slog.NewJSONHandler(&logs, nil)),
		accommodations: accommodationRepositoryStub{
			createInput: &captured,
			createResult: accommodation.Accommodation{
				ID: id, OrganizationID: organizationID, Name: "Casa fictícia",
				Category: accommodation.CategoryFamilyHosting,
				Status:   accommodation.StatusActive, Version: 1,
				CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
			},
		},
	})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/accommodations",
		strings.NewReader(`{"name":"accommodation-log-canary","category":"family_hosting","capacity":6,"client_submission_id":"019f0000-0000-7000-8000-000000000023"}`),
	)
	request.Header.Set("Authorization", "Bearer "+onboardingTokenCanary)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "aaaaaaaaaaaaaaaa")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertAccommodationOnboardingResponse(t, recorder, id)
	assertAccommodationOnboardingCommand(t, captured)
	assertAccommodationOnboardingPrivacy(t, recorder, logs.String())
}

func assertAccommodationOnboardingCommand(
	t *testing.T,
	captured accommodation.CreateCommand,
) {
	t.Helper()
	if captured.Actor.Subject != onboardingTokenCanary ||
		captured.Category != accommodation.CategoryFamilyHosting ||
		captured.ClientSubmissionID.Version() != 7 {
		t.Fatalf("captured command = %+v", captured)
	}
}

func assertAccommodationOnboardingPrivacy(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	logs string,
) {
	t.Helper()
	for _, forbidden := range []string{"cnpj", "cpf", "cadastur", "fnrh", "oidc_issuer"} {
		if strings.Contains(strings.ToLower(recorder.Body.String()), forbidden) {
			t.Fatalf("response contains forbidden %q: %s", forbidden, recorder.Body)
		}
	}
	if strings.Contains(logs, "accommodation-log-canary") ||
		strings.Contains(logs, "aaaaaaaaaaaaaaaa") ||
		strings.Contains(logs, onboardingTokenCanary) {
		t.Fatalf("HTTP log leaked onboarding body, token, or idempotency key: %s", logs)
	}
}

type accommodationOnboardingHTTPCase struct {
	name    string
	enabled bool
	token   string
	body    string
	want    int
}

func TestAccommodationOnboardingEnforcesFlagScopeAndClosedBody(t *testing.T) {
	t.Parallel()

	tests := []accommodationOnboardingHTTPCase{
		{name: "flag disabled", token: "onboard", body: validAccommodationOnboardingBody(), want: http.StatusServiceUnavailable},
		{name: "missing token", enabled: true, body: validAccommodationOnboardingBody(), want: http.StatusUnauthorized},
		{name: "missing scope", enabled: true, token: "manager", body: validAccommodationOnboardingBody(), want: http.StatusForbidden},
		{name: "unknown CNPJ", enabled: true, token: "onboard", body: `{"name":"Casa","category":"family_hosting","capacity":4,"client_submission_id":"019f0000-0000-7000-8000-000000000023","cnpj":"00.000.000/0001-00"}`, want: http.StatusBadRequest},
		{name: "unclassified", enabled: true, token: "onboard", body: `{"name":"Casa","category":"unclassified","capacity":4,"client_submission_id":"019f0000-0000-7000-8000-000000000023"}`, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertAccommodationOnboardingHTTPCase(t, tt)
		})
	}
}

func assertAccommodationOnboardingHTTPCase(
	t *testing.T,
	tt accommodationOnboardingHTTPCase,
) {
	t.Helper()
	handler := phase2HandlerWithOptions(t, phase2HandlerOptions{onboardingEnabled: tt.enabled})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/accommodations", strings.NewReader(tt.body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "aaaaaaaaaaaaaaaa")
	if tt.token != "" {
		request.Header.Set("Authorization", "Bearer "+tt.token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != tt.want {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.want, recorder.Body)
	}
	if recorder.Code != http.StatusCreated && recorder.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
}

func validAccommodationOnboardingBody() string {
	return `{"name":"Casa","category":"family_hosting","capacity":4,"client_submission_id":"019f0000-0000-7000-8000-000000000023"}`
}

func TestMutationWithoutIfMatchReturns428(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t)
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/stays/019f0000-0000-7000-8000-000000000001",
		strings.NewReader(`{"expected_guest_count":2}`),
	)
	request.Header.Set("Authorization", "Bearer writer")
	request.Header.Set("Content-Type", "application/merge-patch+json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
}

func TestInviteCORSRejectsUnknownOrigin(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/invites/"+strings.Repeat("a", 64), nil)
	request.Header.Set("Origin", "https://evil.invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
}

func TestTrustedProxyRejectsInvalidForwardedForBeforeRepository(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	var logs bytes.Buffer
	repository := stayRepositoryStub{
		getInviteCalls: &calls,
		submitCalls:    &calls,
	}
	handler := phase2HandlerWithOptions(t, phase2HandlerOptions{
		stays:   repository,
		trusted: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")},
		logger:  slog.New(slog.NewJSONHandler(&logs, nil)),
	})
	tests := []struct {
		name      string
		method    string
		forwarded []string
	}{
		{name: "GET missing", method: http.MethodGet},
		{name: "GET repeated", method: http.MethodGet, forwarded: []string{"198.51.100.8", "203.0.113.9"}},
		{name: "GET malformed", method: http.MethodGet, forwarded: []string{"forwarded-canary-invalid"}},
		{name: "POST missing", method: http.MethodPost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertForwardedRejected(t, handler, &calls, tt.method, tt.forwarded)
		})
	}
	if strings.Contains(logs.String(), "forwarded-canary-invalid") {
		t.Fatalf("HTTP log leaked rejected forwarded address: %s", &logs)
	}
}

func assertForwardedRejected(
	t *testing.T,
	handler http.Handler,
	calls *atomic.Int32,
	method string,
	forwarded []string,
) {
	t.Helper()
	target := "/api/v1/invites/" + strings.Repeat("a", 64)
	var body *strings.Reader
	if method == http.MethodPost {
		target += "/submit"
		body = strings.NewReader(
			`{"client_submission_id":"019f0000-0000-7000-8000-000000000002","privacy_notice_version":"v1","visitors":[{"client_id":"019f0000-0000-7000-8000-000000000003","role":"responsible","age_band":"25_34","residence_country":"AR"}]}`,
		)
	} else {
		body = strings.NewReader("")
	}
	request := httptest.NewRequest(method, target, body)
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", strings.Repeat("i", 16))
	for _, value := range forwarded {
		request.Header.Add("X-Forwarded-For", value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if calls.Load() != 0 {
		t.Fatalf("repository calls = %d, want 0", calls.Load())
	}
	if recorder.Header().Get("Content-Type") != "application/problem+json" ||
		!strings.Contains(recorder.Body.String(), problemType("invalid-request")) {
		t.Fatalf("problem = headers %v body %s", recorder.Header(), recorder.Body)
	}
	if strings.Contains(recorder.Body.String(), "forwarded-canary-invalid") {
		t.Fatalf("Problem leaked rejected forwarded address: %s", recorder.Body)
	}
}

func TestStayMutationResponsesAreMinimal(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("019f0000-0000-7000-8000-000000000009")
	record := stay.MutationResult{
		ID: id, Status: stay.StatusCheckedIn, Version: 7,
	}
	handler := phase2HandlerWithStay(t, stayRepositoryStub{
		createResult: record, transitionResult: record,
	})
	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"create", "/api/v1/stays", `{"accommodation_id":"019f0000-0000-7000-8000-000000000001","planned_arrival_on":"2026-08-01","planned_departure_on":"2026-08-02","expected_guest_count":2,"client_submission_id":"019f0000-0000-7000-8000-000000000002"}`, http.StatusCreated},
		{"check-in", "/api/v1/stays/" + id.String() + "/check-in", `{}`, http.StatusOK},
		{"check-out", "/api/v1/stays/" + id.String() + "/check-out", `{}`, http.StatusOK},
		{"cancel", "/api/v1/stays/" + id.String() + "/cancel", `{"reason_code":"guest_request","correction":false}`, http.StatusOK},
		{"no-show", "/api/v1/stays/" + id.String() + "/no-show", `{}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMinimalStayMutation(t, handler, tt.path, tt.body, tt.status)
		})
	}
}

func assertMinimalStayMutation(
	t *testing.T,
	handler http.Handler,
	target string,
	body string,
	wantStatus int,
) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer writer")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", strings.Repeat("m", 16))
	request.Header.Set("If-Match", `"1"`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	bodyText := recorder.Body.String()
	for _, forbidden := range []string{
		"accommodation_id", "planned_arrival_on", "planned_departure_on",
		"expected_guest_count", "checked_in_at", "created_at", "updated_at",
	} {
		if strings.Contains(bodyText, forbidden) {
			t.Errorf("response contains %q: %s", forbidden, bodyText)
		}
	}
	for _, required := range []string{`"id"`, `"status"`, `"version"`} {
		if !strings.Contains(bodyText, required) {
			t.Errorf("response misses %s: %s", required, bodyText)
		}
	}
}

func TestMergePatchRejectsNullEvenWithAnotherValidField(t *testing.T) {
	t.Parallel()

	handler := phase2Handler(t)
	tests := []struct {
		name  string
		path  string
		token string
		body  string
	}{
		{"accommodation", "/api/v1/accommodations/019f0000-0000-7000-8000-000000000001", "manager", `{"name":null,"category":"formal_lodging"}`},
		{"membership", "/api/v1/accommodations/019f0000-0000-7000-8000-000000000001/memberships/019f0000-0000-7000-8000-000000000002", "manager", `{"role":null,"active":true}`},
		{"stay", "/api/v1/stays/019f0000-0000-7000-8000-000000000001", "writer", `{"planned_arrival_on":null,"expected_guest_count":2}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNullPatchRejected(t, handler, tt.path, tt.token, tt.body)
		})
	}
}

func TestIdempotencyProcessingHasRetryAfterButHashConflictDoesNot(t *testing.T) {
	t.Parallel()

	processing := phase2HandlerWithStay(t, stayRepositoryStub{
		createErr: idempotency.NewProcessingError(3 * time.Second),
	})
	assertCreateStayConflict(t, processing, "3")

	hashConflict := phase2HandlerWithStay(t, stayRepositoryStub{
		createErr: stay.ErrConflict,
	})
	assertCreateStayConflict(t, hashConflict, "")
}

func TestConsumedInviteUsesSpecificConflictProblem(t *testing.T) {
	t.Parallel()

	handler := phase2HandlerWithStay(t, stayRepositoryStub{submitErr: stay.ErrInviteConsumed})
	body := `{"client_submission_id":"019f0000-0000-7000-8000-000000000002","privacy_notice_version":"v1","visitors":[{"client_id":"019f0000-0000-7000-8000-000000000003","role":"responsible","age_band":"25_34","residence_country":"AR"}]}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/invites/"+strings.Repeat("a", 64)+"/submit",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "invite-submit-key-1234")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if !strings.Contains(recorder.Body.String(), problemType("invite-consumed")) {
		t.Fatalf("problem type = %s", recorder.Body)
	}
	if recorder.Header().Get("Retry-After") != "" {
		t.Fatalf("Retry-After = %q", recorder.Header().Get("Retry-After"))
	}
}

func problemType(code string) string {
	return `"type":"https://turismo.prado.ba.gov.br/problems/` + code + `"`
}

func assertCreateStayConflict(t *testing.T, handler http.Handler, retryAfter string) {
	t.Helper()
	body := `{"accommodation_id":"019f0000-0000-7000-8000-000000000001","planned_arrival_on":"2026-08-01","planned_departure_on":"2026-08-02","expected_guest_count":2,"client_submission_id":"019f0000-0000-7000-8000-000000000002"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/stays", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer writer")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "processing-key-1234")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if got := recorder.Header().Get("Retry-After"); got != retryAfter {
		t.Fatalf("Retry-After = %q, want %q", got, retryAfter)
	}
}

func assertNullPatchRejected(
	t *testing.T,
	handler http.Handler,
	target string,
	token string,
	body string,
) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/merge-patch+json")
	request.Header.Set("If-Match", `"1"`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
}

func phase2Handler(t *testing.T) http.Handler {
	t.Helper()
	return phase2HandlerWithStay(t, stayRepositoryStub{})
}

func phase2HandlerWithStay(t *testing.T, stays stay.Repository) http.Handler {
	t.Helper()
	return phase2HandlerWithOptions(t, phase2HandlerOptions{stays: stays})
}

type phase2HandlerOptions struct {
	stays             stay.Repository
	accommodations    accommodation.Repository
	onboardingEnabled bool
	trusted           []netip.Prefix
	logger            *slog.Logger
}

func phase2HandlerWithOptions(t *testing.T, options phase2HandlerOptions) http.Handler {
	t.Helper()
	verifier := verifierFunc(func(_ context.Context, token string) (access.Principal, error) {
		scopes := map[string][]string{
			"manager":             {"accommodations:manage"},
			"onboard":             {"accommodations:onboard"},
			onboardingTokenCanary: {"accommodations:onboard"},
			"writer":              {"stays:write"},
		}
		return access.NewPrincipal("https://issuer.invalid", token, scopes[token]), nil
	})
	accommodations := options.accommodations
	if accommodations == nil {
		accommodations = accommodationRepositoryStub{}
	}
	handler, _, err := httpapi.New(httpapi.Dependencies{
		Readiness:                      readinessFunc(func(context.Context) error { return nil }),
		Verifier:                       verifier,
		Accommodations:                 accommodation.NewService(accommodations),
		AccommodationOnboardingEnabled: options.onboardingEnabled,
		Stays:                          stay.NewService(options.stays),
		CORSAllowedOrigins:             []string{"https://allowed.invalid"},
		TrustedProxyCIDRs:              options.trusted,
		CursorKeys: config.KeyringConfig{
			CurrentVersion: "cursor-v1",
			Keys: map[string][]byte{
				"cursor-v1": []byte("cursor-test-key-is-at-least-32-bytes"),
			},
		},
		Logger: options.logger,
	})
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return handler
}
