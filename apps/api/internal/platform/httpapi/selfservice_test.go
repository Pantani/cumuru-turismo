package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/activation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

const (
	posterToken          = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"
	selfServiceStayID    = "019f0000-0000-7000-8000-000000000001"
	selfServiceAccommoID = "019f0000-0000-7000-8000-000000000002"
)

type selfServiceStayStub struct {
	stay.Repository
	approvalCommand  *stay.ApprovalCommand
	rejectionCommand *stay.RejectionCommand
	page             *stay.PageRequest
	selfRegistration *stay.SelfRegistrationCommand
	posterRequest    *stay.InviteRequest
}

func (s selfServiceStayStub) Approve(
	_ context.Context,
	command stay.ApprovalCommand,
) (stay.MutationResult, bool, error) {
	if s.approvalCommand != nil {
		*s.approvalCommand = command
	}
	return stay.MutationResult{
		ID: command.StayID, Status: stay.StatusPreRegistered, Version: 4,
	}, false, nil
}

func (s selfServiceStayStub) Reject(
	_ context.Context,
	command stay.RejectionCommand,
) (stay.MutationResult, bool, error) {
	if s.rejectionCommand != nil {
		*s.rejectionCommand = command
	}
	return stay.MutationResult{
		ID: command.StayID, Status: stay.StatusCancelled, Version: 4,
	}, false, nil
}

func (s selfServiceStayStub) List(
	_ context.Context,
	_ access.Principal,
	page stay.PageRequest,
) (stay.Page, error) {
	if s.page != nil {
		*s.page = page
	}
	return stay.Page{Items: []stay.Record{}}, nil
}

func (s selfServiceStayStub) SubmitSelfRegistration(
	_ context.Context,
	command stay.SelfRegistrationCommand,
) (stay.SelfRegistrationAccepted, bool, error) {
	if s.selfRegistration != nil {
		*s.selfRegistration = command
	}
	return stay.SelfRegistrationAccepted{
		SubmissionID: uuid.MustParse(selfServiceStayID), StayID: uuid.MustParse(selfServiceStayID),
		Status: "accepted", StayStatus: stay.StatusPreRegistered,
		ApprovalState: stay.ApprovalPending, Version: 1,
	}, false, nil
}

func (s selfServiceStayStub) GetAccommodationInviteContext(
	_ context.Context,
	request stay.InviteRequest,
) (stay.AccommodationInviteContext, error) {
	if s.posterRequest != nil {
		*s.posterRequest = request
	}
	return stay.AccommodationInviteContext{
		AccommodationName: "Pousada", PrivacyNoticeVersion: "2026-08",
		ProofOfWork: stay.ProofOfWorkChallenge{
			Algorithm: "sha256-leading-zero-bits", Challenge: strings.Repeat("c", 98),
			DifficultyBits: 12, ExpiresAt: time.Now().UTC(),
		},
	}, nil
}

type selfServiceActivationStub struct {
	command *activation.CompleteCommand
}

func (selfServiceActivationStub) Create(
	_ context.Context,
	command activation.CreateCommand,
) (activation.Created, bool, error) {
	return activation.Created{
		ActivationID: uuid.MustParse(selfServiceStayID), AccountID: uuid.MustParse(selfServiceAccommoID),
		URL: "http://127.0.0.1:4173/ativacao#" + posterToken, Version: 1,
	}, false, nil
}

func (selfServiceActivationStub) Context(
	context.Context,
	activation.Request,
) (activation.Context, error) {
	return activation.Context{AccommodationName: "Pousada", DisplayName: "Operador"}, nil
}

func (s selfServiceActivationStub) Complete(
	_ context.Context,
	command activation.CompleteCommand,
) error {
	if s.command != nil {
		*s.command = command
	}
	return nil
}

func selfServiceHandler(
	t *testing.T,
	stays stay.Repository,
	activations activation.Repository,
) http.Handler {
	t.Helper()
	verifier := verifierFunc(func(_ context.Context, token string) (access.Principal, error) {
		scopes := map[string][]string{
			"approver": {"stays:approve", "stays:read:own"},
			"writer":   {"stays:write", "stays:read:own"},
			"manager":  {"accommodations:manage"},
			// The real set an activated account receives, read from the domain
			// rather than copied, so widening or narrowing it there is felt here.
			"activated": activation.Scopes(),
		}
		return access.NewPrincipal("https://issuer.invalid", token, scopes[token]), nil
	})
	var activationService *activation.Service
	if activations != nil {
		activationService = activation.NewService(activations)
	}
	handler, _, err := httpapi.New(httpapi.Dependencies{
		Readiness:          readinessFunc(func(context.Context) error { return nil }),
		Verifier:           verifier,
		Accommodations:     accommodation.NewService(accommodationRepositoryStub{}),
		Stays:              stay.NewService(stays),
		SelfServiceEnabled: true,
		Activation:         activationService,
		CORSAllowedOrigins: []string{"https://allowed.invalid"},
		CursorKeys: config.KeyringConfig{
			CurrentVersion: "cursor-v1",
			Keys: map[string][]byte{
				"cursor-v1": []byte("cursor-test-key-is-at-least-32-bytes"),
			},
		},
	})
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return handler
}

func selfServiceRequest(method, target, token, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Request-ID", "request-000000000001")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

// N-25: the operator holding stays:write and not the approval scope is refused.
// This is the regression the feature exists to prevent, and it must fail at the
// transport, before any repository is reached.
func TestApprovalRefusesTheEditScope(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	for _, route := range []string{"approve", "reject"} {
		request := selfServiceRequest(
			http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/"+route, "writer", "{}",
		)
		request.Header.Set("If-Match", `"3"`)
		request.Header.Set("Idempotency-Key", "approval-key-0000000001")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s with stays:write = %d, want 403", route, recorder.Code)
		}
	}
}

func TestApprovalAcceptsItsOwnScopeAndCarriesTheVersion(t *testing.T) {
	t.Parallel()

	var captured stay.ApprovalCommand
	handler := selfServiceHandler(t, selfServiceStayStub{approvalCommand: &captured}, nil)
	request := selfServiceRequest(
		http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/approve", "approver", "{}",
	)
	request.Header.Set("If-Match", `"3"`)
	request.Header.Set("Idempotency-Key", "approval-key-0000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("approve = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if captured.ExpectedVersion != 3 || captured.StayID.String() != selfServiceStayID {
		t.Fatalf("approval command = %#v", captured)
	}
	if recorder.Header().Get("ETag") != `"4"` {
		t.Fatalf("ETag = %q", recorder.Header().Get("ETag"))
	}
}

// If-Match is mandatory: a blind approval would race an edit it never saw.
func TestApprovalRequiresIfMatch(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	request := selfServiceRequest(
		http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/approve", "approver", "{}",
	)
	request.Header.Set("Idempotency-Key", "approval-key-0000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("approve without If-Match = %d, want 428", recorder.Code)
	}
}

// N-28: a reason outside the closed list is refused before it can reach an
// append-only audit table.
func TestRejectionRefusesFreeText(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	bodies := map[string]int{
		`{"reason_code":"not_a_guest"}`:            http.StatusOK,
		`{"reason_code":"CPF falso"}`:              http.StatusUnprocessableEntity,
		`{"reason_code":""}`:                       http.StatusUnprocessableEntity,
		`{"reason_code":"not_a_guest","note":"x"}`: http.StatusBadRequest,
	}
	for body, want := range bodies {
		request := selfServiceRequest(
			http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/reject", "approver", body,
		)
		request.Header.Set("If-Match", `"3"`)
		request.Header.Set("Idempotency-Key", "rejection-key-000000001")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Fatalf("reject %s = %d, want %d", body, recorder.Code, want)
		}
	}
}

// The approval queue is the existing listing with a filter, so the two new query
// parameters must actually reach the repository.
func TestListStaysCarriesTheApprovalFilters(t *testing.T) {
	t.Parallel()

	var captured stay.PageRequest
	handler := selfServiceHandler(t, selfServiceStayStub{page: &captured}, nil)
	target := "/api/v1/stays?accommodation_id=" + selfServiceAccommoID +
		"&approval_state=pending&provenance=self_service"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, selfServiceRequest(http.MethodGet, target, "approver", ""))

	if recorder.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", recorder.Code, recorder.Body.String())
	}
	if captured.ApprovalState != stay.ApprovalPending ||
		captured.Provenance != stay.ProvenanceSelfService {
		t.Fatalf("page request = %#v", captured)
	}
}

func TestListStaysRefusesAnUnknownApprovalFilter(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	for _, query := range []string{
		"approval_state=not_required", "approval_state=whatever", "provenance=operator",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, selfServiceRequest(
			http.MethodGet, "/api/v1/stays?"+query, "approver", "",
		))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("list ?%s = %d, want 422", query, recorder.Code)
		}
	}
}

// The token travels in a header read from the fragment, never in the path, so
// it reaches no request line and no access log.
func TestPosterContextReadsTheCapabilityHeader(t *testing.T) {
	t.Parallel()

	var captured stay.InviteRequest
	handler := selfServiceHandler(t, selfServiceStayStub{posterRequest: &captured}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accommodation-invite", nil)
	request.Header.Set("X-Cumuru-Invite-Token", posterToken)
	request.Header.Set("X-Request-ID", "request-000000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("poster context = %d: %s", recorder.Code, recorder.Body.String())
	}
	if captured.Token != posterToken {
		t.Fatal("the capability header did not reach the service")
	}
	if strings.Contains(recorder.Body.String(), posterToken) {
		t.Fatal("the response echoed the capability")
	}
}

// The open channel has nowhere to put an identity field, and the strict decoder
// makes that a 400 rather than a silently dropped value.
func TestSelfRegistrationRefusesIdentityFields(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	for _, field := range []string{
		`"name":"Fulano"`, `"document":"12345678901"`,
		`"email":"a@b.invalid"`, `"phone":"+5573999999999"`,
	} {
		body := `{"client_submission_id":"` + selfServiceStayID + `",` + field + `}`
		request := selfServiceRequest(
			http.MethodPost, "/api/v1/accommodation-invite/submit", "writer", body,
		)
		request.Header.Set("X-Cumuru-Invite-Token", posterToken)
		request.Header.Set("Idempotency-Key", "self-registration-000001")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("submit with %s = %d, want 400", field, recorder.Code)
		}
	}
}

func TestSelfRegistrationAcceptsGeneralizedDataAndAnswersPending(t *testing.T) {
	t.Parallel()

	var captured stay.SelfRegistrationCommand
	handler := selfServiceHandler(t, selfServiceStayStub{selfRegistration: &captured}, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, selfRegistrationRequest(selfRegistrationBody("companion")))

	if recorder.Code != http.StatusOK {
		t.Fatalf("submit = %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["approval_state"] != "pending" || body["stay_status"] != "pre_registered" {
		t.Fatalf("body = %#v", body)
	}
	if captured.ProofOfWork.Solution == "" || captured.Token != posterToken {
		t.Fatalf("command = %#v", captured)
	}
}

// N-25's sibling on the open side: a submission about a child by an
// unauthenticated stranger is refused with 422.
func TestSelfRegistrationRefusesMinors(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, selfRegistrationRequest(selfRegistrationBody("minor")))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("submit with a minor = %d, want 422", recorder.Code)
	}
}

func selfRegistrationBody(role string) string {
	return `{
		"client_submission_id":"019f0000-0000-7000-8000-000000000010",
		"privacy_notice_version":"2026-08",
		"planned_arrival_on":"2026-12-10",
		"planned_departure_on":"2026-12-12",
		"visitors":[
			{"client_id":"019f0000-0000-7000-8000-000000000011","role":"responsible",
			 "age_band":"25_34","residence_country":"BR","residence_state":"BA",
			 "residence_city_code":"2925303"},
			{"client_id":"019f0000-0000-7000-8000-000000000012","role":"` + role + `",
			 "age_band":"25_34","residence_country":"PT",
			 "residence_state":null,"residence_city_code":null}
		],
		"proof_of_work":{"challenge":"` + strings.Repeat("c", 98) + `","solution":"AAAAAAAAAAA"}
	}`
}

func selfRegistrationRequest(body string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/accommodation-invite/submit", strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cumuru-Invite-Token", posterToken)
	request.Header.Set("Idempotency-Key", "self-registration-000001")
	request.Header.Set("X-Request-ID", "request-000000000001")
	return request
}

// The activation answers 204 with no body and no session: no response may echo
// the secret, and the capability must not become a long-lived credential.
func TestActivationCompletionReturnsNoContentAndNoSession(t *testing.T) {
	t.Parallel()

	var captured activation.CompleteCommand
	handler := selfServiceHandler(t, selfServiceStayStub{}, selfServiceActivationStub{command: &captured})
	body := `{"password":"uma-senha-bem-longa"}`
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/activation/complete", strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cumuru-Activation-Token", posterToken)
	request.Header.Set("X-Request-ID", "request-000000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
		t.Fatalf("complete = %d with %d bytes", recorder.Code, recorder.Body.Len())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
	if captured.Password == "" {
		t.Fatal("the password did not reach the service")
	}
}

// With the feature off the surfaces do not exist at all, which is a 404 rather
// than a half-configured route.
func TestDisabledSelfServiceDoesNotRegisterTheOpenChannel(t *testing.T) {
	t.Parallel()

	handler, _, err := httpapi.New(httpapi.Dependencies{
		Readiness: readinessFunc(func(context.Context) error { return nil }),
		Verifier: verifierFunc(func(context.Context, string) (access.Principal, error) {
			return access.NewPrincipal("https://issuer.invalid", "s", []string{"stays:approve"}), nil
		}),
		Stays:              stay.NewService(selfServiceStayStub{}),
		SelfServiceEnabled: false,
		CORSAllowedOrigins: []string{"https://allowed.invalid"},
		CursorKeys: config.KeyringConfig{
			CurrentVersion: "cursor-v1",
			Keys: map[string][]byte{
				"cursor-v1": []byte("cursor-test-key-is-at-least-32-bytes"),
			},
		},
	})
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	for _, target := range []string{
		"/api/v1/accommodation-invite", "/api/v1/activation",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, selfServiceRequest(http.MethodGet, target, "approver", ""))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s with the feature off = %d, want 404", target, recorder.Code)
		}
	}
}

// Consent lives on the survey path, not here. The open channel records the
// acceptance of the versioned notice in group_submissions.privacy_notice_version,
// exactly as the assisted and invite channels do; survey.consent_decisions
// cannot be written without fabricating a questionnaire response.
//
// The assertion is on the emitted keys, so the handler cannot start inventing a
// consent field even if an optional one reappears in the contract.
func TestPosterContextEmitsNoConsentField(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, selfServiceStayStub{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accommodation-invite", nil)
	request.Header.Set("X-Cumuru-Invite-Token", posterToken)
	request.Header.Set("X-Request-ID", "request-000000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]bool{
		"accommodation_name": true, "privacy_notice_version": true, "proof_of_work": true,
	}
	for key := range body {
		if !want[key] {
			t.Fatalf("unexpected key %q in the poster context", key)
		}
	}
	if len(body) != len(want) {
		t.Fatalf("poster context = %#v, want exactly %v", body, want)
	}
}

// The positive mirror of N-25. The negative alone would pass even if the whole
// approval route were unreachable, so the pair is what proves the transport
// layer admits exactly one scope set and refuses the other.
//
// This is the flow the feature exists for: the accommodation that received the
// activation link approves its own queue, instead of approval staying with the
// principal that self-provisioned it.
func TestActivatedAccountCanApproveItsOwnQueue(t *testing.T) {
	t.Parallel()

	var captured stay.ApprovalCommand
	handler := selfServiceHandler(t, selfServiceStayStub{approvalCommand: &captured}, nil)
	for _, route := range []string{"approve", "reject"} {
		body := "{}"
		if route == "reject" {
			body = `{"reason_code":"not_a_guest"}`
		}
		request := selfServiceRequest(
			http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/"+route, "activated", body,
		)
		request.Header.Set("If-Match", `"3"`)
		request.Header.Set("Idempotency-Key", "approval-key-0000000001")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s as an activated account = %d, want 200: %s",
				route, recorder.Code, recorder.Body.String())
		}
	}
	if captured.StayID.String() != selfServiceStayID {
		t.Fatalf("approval command = %#v", captured)
	}
}

// The two layers are independent and must be provable in isolation. The scope
// is the transport gate; accommodation.allowedOperations plus the manager role
// is the domain gate. Holding the scope reaches the handler and nothing more —
// the domain still decides, and its own refusals are pinned by
// TestApprovalNeedsItsOwnOperationAndAManagerRole in the store package and by
// TestApprovalIsAnOperationOfItsOwnAndOnlyWhenActive in the accommodation one.
func TestApprovalScopeDoesNotGrantTheDomainOperation(t *testing.T) {
	t.Parallel()

	if !accommodation.StatusActive.Allows(accommodation.OperationApproveStay) {
		t.Fatal("approve_stay disappeared from the active accommodation")
	}
	for _, status := range []accommodation.Status{
		accommodation.StatusSuspended,
		accommodation.StatusClosed,
		accommodation.StatusPendingReview,
	} {
		if status.Allows(accommodation.OperationApproveStay) {
			t.Fatalf("%s allowed approval; the scope would be the only gate", status)
		}
	}
}

// stateStayStub answers with a chosen error so the transport mapping can be
// exercised on its own. The states below are where the Wave B2 plumbing was
// written without a test first, and where the QA found the gap: the layer had
// coverage of shape and none of state.
type stateStayStub struct {
	stay.Repository
	err      error
	replayed bool
}

func (s stateStayStub) Approve(
	_ context.Context,
	command stay.ApprovalCommand,
) (stay.MutationResult, bool, error) {
	return stay.MutationResult{
		ID: command.StayID, Status: stay.StatusPreRegistered, Version: 4,
	}, s.replayed, s.err
}

func (s stateStayStub) SubmitSelfRegistration(
	_ context.Context,
	_ stay.SelfRegistrationCommand,
) (stay.SelfRegistrationAccepted, bool, error) {
	return stay.SelfRegistrationAccepted{
		SubmissionID: uuid.MustParse(selfServiceStayID), StayID: uuid.MustParse(selfServiceStayID),
		Status: "accepted", StayStatus: stay.StatusPreRegistered,
		ApprovalState: stay.ApprovalPending, Version: 1,
	}, s.replayed, s.err
}

func (s stateStayStub) GetAccommodationInviteContext(
	_ context.Context,
	_ stay.InviteRequest,
) (stay.AccommodationInviteContext, error) {
	return stay.AccommodationInviteContext{}, s.err
}

func approveRequest(token string) *http.Request {
	request := selfServiceRequest(
		http.MethodPost, "/api/v1/stays/"+selfServiceStayID+"/approve", token, "{}",
	)
	request.Header.Set("If-Match", `"3"`)
	request.Header.Set("Idempotency-Key", "approval-key-0000000001")
	return request
}

// N-18 and N-22: the open channel answers 429 with Retry-After, on both the
// challenge route and the submission route. Without the header a client has
// nothing to back off against and hammers the bucket that just refused it.
func TestOpenChannelRateLimitAnswersRetryAfter(t *testing.T) {
	t.Parallel()

	handler := selfServiceHandler(t, stateStayStub{err: stay.ErrRateLimited}, nil)
	targets := map[string]*http.Request{
		"challenge": func() *http.Request {
			request := httptest.NewRequest(
				http.MethodGet, "/api/v1/accommodation-invite", nil,
			)
			request.Header.Set("X-Cumuru-Invite-Token", posterToken)
			request.Header.Set("X-Request-ID", "request-000000000001")
			return request
		}(),
		"submission": selfRegistrationRequest(selfRegistrationBody("companion")),
	}
	for name, request := range targets {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusTooManyRequests {
				t.Fatalf("%s = %d, want 429: %s", name, recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("Retry-After") == "" {
				t.Fatalf("%s answered 429 without Retry-After", name)
			}
		})
	}
}

// N-27, first half: a stale If-Match is 412 and a divergent body under the same
// key is 409. Both were unreachable before, because only the presence of the
// header was asserted.
func TestApprovalMapsPreconditionAndIdempotencyConflicts(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want int
	}{
		"stale If-Match":                {stay.ErrPreconditionFailed, http.StatusPreconditionFailed},
		"same key, divergent body":      {stay.ErrConflict, http.StatusConflict},
		"decision already taken":        {stay.ErrConflict, http.StatusConflict},
		"stay of another accommodation": {stay.ErrNotFound, http.StatusNotFound},
		"operator without the role":     {stay.ErrForbidden, http.StatusForbidden},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := selfServiceHandler(t, stateStayStub{err: test.err}, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, approveRequest("approver"))
			if recorder.Code != test.want {
				t.Fatalf("%s = %d, want %d", name, recorder.Code, test.want)
			}
		})
	}
}

// N-27, second half: the replay reproduces the stored answer and says so. A
// client that cannot tell a replay from a fresh write has no way to know
// whether its retry did something twice.
func TestApprovalReplayIsAnnouncedAndReproducesTheAnswer(t *testing.T) {
	t.Parallel()

	fresh := httptest.NewRecorder()
	selfServiceHandler(t, stateStayStub{}, nil).ServeHTTP(fresh, approveRequest("approver"))
	replay := httptest.NewRecorder()
	selfServiceHandler(t, stateStayStub{replayed: true}, nil).
		ServeHTTP(replay, approveRequest("approver"))

	if fresh.Code != http.StatusOK || replay.Code != http.StatusOK {
		t.Fatalf("fresh = %d, replay = %d", fresh.Code, replay.Code)
	}
	if fresh.Header().Get("Idempotency-Replayed") != "false" {
		t.Fatalf("fresh Idempotency-Replayed = %q", fresh.Header().Get("Idempotency-Replayed"))
	}
	if replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay Idempotency-Replayed = %q", replay.Header().Get("Idempotency-Replayed"))
	}
	if fresh.Body.String() != replay.Body.String() {
		t.Fatalf("replay body = %s, want the stored answer %s",
			replay.Body.String(), fresh.Body.String())
	}
	if fresh.Header().Get("ETag") != replay.Header().Get("ETag") {
		t.Fatal("the replay reported a different version")
	}
}

// The open channel replays too, and its answer must be identical: the submitter
// retrying on a flaky connection must not create a second pending stay.
func TestSelfRegistrationReplayReproducesTheAnswer(t *testing.T) {
	t.Parallel()

	fresh := httptest.NewRecorder()
	selfServiceHandler(t, stateStayStub{}, nil).
		ServeHTTP(fresh, selfRegistrationRequest(selfRegistrationBody("companion")))
	replay := httptest.NewRecorder()
	selfServiceHandler(t, stateStayStub{replayed: true}, nil).
		ServeHTTP(replay, selfRegistrationRequest(selfRegistrationBody("companion")))

	if fresh.Body.String() != replay.Body.String() || fresh.Code != replay.Code {
		t.Fatalf("replay = %d %s, want %d %s",
			replay.Code, replay.Body.String(), fresh.Code, fresh.Body.String())
	}
	if replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("the open channel did not announce the replay")
	}
}
