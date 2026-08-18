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
	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/google/uuid"
)

const (
	accessRequestPath = "/api/v1/accommodation-access-requests"
	accessRequestUUID = "019f0000-0000-7000-8000-0000000000c1"
)

type accessRequestStub struct {
	create   *accessrequest.CreateCommand
	page     *accessrequest.PageRequest
	approval *accessrequest.ApprovalCommand
	reject   *accessrequest.RejectionCommand
	err      error
}

func (s accessRequestStub) Context(
	_ context.Context,
	_ accessrequest.ContextRequest,
) (accessrequest.Context, error) {
	return accessrequest.Context{
		ProofOfWork: accessrequest.ProofOfWorkChallenge{
			Algorithm: "sha256-leading-zero-bits", Challenge: strings.Repeat("c", 98),
			DifficultyBits: 12, ExpiresAt: time.Now().UTC(),
		},
		PrivacyNoticeVersion: accessrequest.PrivacyNoticeVersion,
	}, s.err
}

func (s accessRequestStub) Create(
	_ context.Context,
	command accessrequest.CreateCommand,
) (accessrequest.Created, bool, error) {
	if s.create != nil {
		*s.create = command
	}
	return accessrequest.Created{
		ID: uuid.MustParse(accessRequestUUID), CreatedAt: time.Now().UTC(),
	}, false, s.err
}

func (s accessRequestStub) List(
	_ context.Context,
	page accessrequest.PageRequest,
) (accessrequest.Page, error) {
	if s.page != nil {
		*s.page = page
	}
	return accessrequest.Page{Items: []accessrequest.Request{}}, s.err
}

func (s accessRequestStub) Approve(
	_ context.Context,
	command accessrequest.ApprovalCommand,
) (accessrequest.Request, bool, error) {
	if s.approval != nil {
		*s.approval = command
	}
	return decidedRequest(accessrequest.StateApproved), false, s.err
}

func (s accessRequestStub) Reject(
	_ context.Context,
	command accessrequest.RejectionCommand,
) (accessrequest.Request, bool, error) {
	if s.reject != nil {
		*s.reject = command
	}
	return decidedRequest(accessrequest.StateRejected), false, s.err
}

func decidedRequest(state accessrequest.ApprovalState) accessrequest.Request {
	return accessrequest.Request{
		ID: uuid.MustParse(accessRequestUUID), AccommodationName: "Pousada",
		Category: "formal_lodging", Capacity: 12, CityLabel: "Prado",
		StateCode: "BA", ApprovalState: state, Version: 2,
	}
}

func accessRequestHandler(t *testing.T, repository accessrequest.Repository) http.Handler {
	t.Helper()
	verifier := verifierFunc(func(_ context.Context, token string) (access.Principal, error) {
		scopes := map[string][]string{
			"onboarder": {"accommodations:onboard"},
			"manager":   {"accommodations:manage"},
			"approver":  {"stays:approve"},
		}
		return access.NewPrincipal("https://issuer.invalid", token, scopes[token]), nil
	})
	handler, _, err := httpapi.New(httpapi.Dependencies{
		Readiness:          readinessFunc(func(context.Context) error { return nil }),
		Verifier:           verifier,
		AccessRequests:     accessrequest.NewService(repository),
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

func accessRequestBody() string {
	return `{"client_submission_id":"019f0000-0000-7000-8000-0000000000d1",` +
		`"accommodation_name":"Pousada do Descobrimento","category":"formal_lodging",` +
		`"capacity":12,"contact_name":"Responsavel",` +
		`"contact_email":"Contato@Pousada.INVALID","contact_phone":null,` +
		`"city_label":"Prado","state_code":"BA",` +
		`"privacy_notice_version":"` + accessrequest.PrivacyNoticeVersion + `",` +
		`"proof_of_work":{"challenge":"` + strings.Repeat("c", 98) + `","solution":"abc"}}`
}

func publicRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Request-ID", "019f0000-0000-7000-8000-0000000000e1")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

// The creation answer is a minimal receipt: no submitted field comes back,
// because the route is open and the echo would turn creation into a lookup of
// somebody else's contact details.
func TestCreateAnswersOnlyTheReceipt(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	request := publicRequest(http.MethodPost, accessRequestPath, accessRequestBody())
	request.Header.Set("Idempotency-Key", "access-request-key-0001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	for field := range body {
		if field != "id" && field != "created_at" {
			t.Fatalf("the receipt echoed %q", field)
		}
	}
}

// The address reaches the domain already normalized, which is what the partial
// unique index of one pending request per e-mail needs to turn a re-send into a
// conflict.
func TestCreateNormalizesTheContactEmail(t *testing.T) {
	t.Parallel()

	var captured accessrequest.CreateCommand
	handler := accessRequestHandler(t, accessRequestStub{create: &captured})
	request := publicRequest(http.MethodPost, accessRequestPath, accessRequestBody())
	request.Header.Set("Idempotency-Key", "access-request-key-0001")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if captured.ContactEmail != "contato@pousada.invalid" {
		t.Fatalf("ContactEmail = %q", captured.ContactEmail)
	}
	if captured.RateSubject == "" {
		t.Fatal("the rate limit subject never reached the domain")
	}
}

// The 409 of the duplicate pending e-mail carries no Retry-After on purpose:
// re-sending does not resolve it, the request is already in the queue and only a
// decision takes it out. The header stays reserved for what waiting does
// resolve.
func TestDuplicatePendingEmailAnswersConflictWithoutRetryAfter(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{err: accessrequest.ErrConflict})
	request := publicRequest(http.MethodPost, accessRequestPath, accessRequestBody())
	request.Header.Set("Idempotency-Key", "access-request-key-0001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if retry := recorder.Header().Get("Retry-After"); retry != "" {
		t.Fatalf("Retry-After = %q on a conflict that waiting does not resolve", retry)
	}
}

// The contract of this route has no 422: a refused challenge and a malformed
// field are both 400, and a 422 would be an answer the generated client cannot
// read.
func TestRefusedProofOfWorkAnswersBadRequest(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{err: accessrequest.ErrInvalidInput})
	request := publicRequest(http.MethodPost, accessRequestPath, accessRequestBody())
	request.Header.Set("Idempotency-Key", "access-request-key-0001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestContextAnswersTheChallengeAndTheNoticeVersion(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, publicRequest(
		http.MethodGet, accessRequestPath+"/context", "",
	))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		PrivacyNoticeVersion string `json:"privacy_notice_version"`
		ProofOfWork          struct {
			Algorithm string `json:"algorithm"`
		} `json:"proof_of_work"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body.PrivacyNoticeVersion == "" || body.ProofOfWork.Algorithm == "" {
		t.Fatalf("context = %s", recorder.Body.String())
	}
}

// Approving produces the same effect as creating the record by hand, so it must
// not cost less permission: whoever lacks accommodations:onboard is refused at
// the transport, before any repository is reached.
func TestQueueAndDecisionsRefuseTheWrongScope(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	targets := map[string]*http.Request{
		"list":    publicRequest(http.MethodGet, accessRequestPath, ""),
		"approve": decisionRequest("approve", "{}"),
		"reject":  decisionRequest("reject", `{"reason_code":"abuse"}`),
	}
	for name, request := range targets {
		for _, token := range []string{"manager", "approver"} {
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("%s as %s = %d, want 403", name, token, recorder.Code)
			}
		}
	}
}

func decisionRequest(action, body string) *http.Request {
	request := publicRequest(
		http.MethodPost, accessRequestPath+"/"+accessRequestUUID+"/"+action, body,
	)
	request.Header.Set("If-Match", `"1"`)
	request.Header.Set("Idempotency-Key", "decision-key-000000001")
	return request
}

func TestApprovalCarriesTheVersionAndTheActor(t *testing.T) {
	t.Parallel()

	var captured accessrequest.ApprovalCommand
	handler := accessRequestHandler(t, accessRequestStub{approval: &captured})
	request := decisionRequest("approve", "{}")
	request.Header.Set("Authorization", "Bearer onboarder")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.ExpectedVersion != 1 || captured.Actor.Subject != "onboarder" {
		t.Fatalf("command = %#v", captured)
	}
	if recorder.Header().Get("ETag") != `"2"` {
		t.Fatalf("ETag = %q", recorder.Header().Get("ETag"))
	}
}

// If-Match is mandatory on both decisions: without it, two screens open on the
// same queue would decide over one another.
func TestDecisionsDemandIfMatch(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	request := publicRequest(
		http.MethodPost, accessRequestPath+"/"+accessRequestUUID+"/reject",
		`{"reason_code":"abuse"}`,
	)
	request.Header.Set("Authorization", "Bearer onboarder")
	request.Header.Set("Idempotency-Key", "decision-key-000000001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428", recorder.Code)
	}
}

func TestRejectionRefusesAReasonOutsideTheClosedList(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	request := decisionRequest("reject", `{"reason_code":"nao gostei"}`)
	request.Header.Set("Authorization", "Bearer onboarder")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestQueueForwardsTheStateFilter(t *testing.T) {
	t.Parallel()

	var captured accessrequest.PageRequest
	handler := accessRequestHandler(t, accessRequestStub{page: &captured})
	request := publicRequest(
		http.MethodGet, accessRequestPath+"?approval_state=pending&limit=10", "",
	)
	request.Header.Set("Authorization", "Bearer onboarder")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.State != accessrequest.StatePending || captured.Limit != 10 {
		t.Fatalf("page = %#v", captured)
	}
}

// A state outside the enum is refused instead of returning an empty page, which
// the screen would read as "there are no requests".
func TestQueueRefusesAnUnknownStateFilter(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	request := publicRequest(http.MethodGet, accessRequestPath+"?approval_state=todos", "")
	request.Header.Set("Authorization", "Bearer onboarder")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

// The open surface presents no capability token, so the preflight needs no new
// header: what the form sends is already allowed.
func TestPreflightCoversTheFormWithoutANewHeader(t *testing.T) {
	t.Parallel()

	handler := accessRequestHandler(t, accessRequestStub{})
	request := publicRequest(http.MethodOptions, accessRequestPath, "")
	request.Header.Set("Origin", "https://allowed.invalid")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	allowed := recorder.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"Content-Type", "Idempotency-Key", "X-Request-ID"} {
		if !strings.Contains(allowed, header) {
			t.Fatalf("preflight does not allow %s: %q", header, allowed)
		}
	}
}
