package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
)

var (
	questionnaireID = uuid.MustParse("019f0000-0000-7000-8000-000000000031")
	versionID       = uuid.MustParse("019f0000-0000-7000-8000-000000000032")
	questionID      = uuid.MustParse("019f0000-0000-7000-8000-000000000033")
)

type questionnaireRepositoryStub struct {
	questionnaire.Repository
	published     questionnaire.Published
	submitErr     error
	submitCalls   *atomic.Int32
	submitCommand *questionnaire.SubmissionCommand
}

func (s questionnaireRepositoryStub) GetPublished(
	context.Context,
	string,
) (questionnaire.Published, error) {
	return s.published, nil
}

func (s questionnaireRepositoryStub) Submit(
	_ context.Context,
	command questionnaire.SubmissionCommand,
) (questionnaire.SubmissionAccepted, bool, error) {
	if s.submitCalls != nil {
		s.submitCalls.Add(1)
	}
	if s.submitCommand != nil {
		*s.submitCommand = command
	}
	return questionnaire.SubmissionAccepted{
		ResponseID:    questionnaireID,
		Participation: questionnaire.ParticipationDeclined,
		Status:        "accepted",
	}, false, s.submitErr
}

func TestSurveyRateLimitIgnoresForgedForwardedAddress(t *testing.T) {
	t.Parallel()
	var captured questionnaire.SubmissionCommand
	handler := questionnaireHandler(questionnaireRepositoryStub{
		submitErr: questionnaire.ErrRateLimited, submitCommand: &captured,
	})
	body := `{"questionnaire_version_id":"` + versionID.String() +
		`","client_submission_id":"019f0000-0000-7000-8000-000000000035",` +
		`"participation":"declined","answers":[],"consent_decisions":[]}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/survey-responses",
		strings.NewReader(body),
	)
	request.RemoteAddr = "203.0.113.123:4321"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "survey-rate-limit-key-1234")
	request.Header.Set("Survey-Capability", "payload.signature")
	request.Header.Set("X-Forwarded-For", "198.51.100.44")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body=%s", recorder.Code, recorder.Body)
	}
	if captured.RateSubject != "203.0.113.0/24" {
		t.Fatalf("rate subject = %q, want untrusted remote prefix", captured.RateSubject)
	}
	if strings.Contains(recorder.Body.String(), "198.51.100.44") {
		t.Fatalf("problem leaked forged address: %s", recorder.Body)
	}
}

func TestQuestionnaireAdministrativeScopesAreSeparated(t *testing.T) {
	t.Parallel()

	handler := questionnaireHandler(questionnaireRepositoryStub{})
	tests := []struct {
		name   string
		token  string
		method string
		target string
		body   string
		want   int
	}{
		{
			name: "missing credential", method: http.MethodGet,
			target: "/api/v1/questionnaires", want: http.StatusUnauthorized,
		},
		{
			name: "editor cannot approve", token: "editor", method: http.MethodPost,
			target: "/api/v1/questionnaire-versions/" + versionID.String() + "/approve",
			body:   `{}`, want: http.StatusForbidden,
		},
		{
			name: "reviewer cannot edit", token: "reviewer", method: http.MethodPut,
			target: "/api/v1/questionnaire-versions/" + versionID.String(),
			body:   `{}`, want: http.StatusForbidden,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.want, recorder.Body)
			}
		})
	}
}

func TestPublishedQuestionnaireExposesOnlyPublicProjection(t *testing.T) {
	t.Parallel()

	handler := questionnaireHandler(questionnaireRepositoryStub{
		published: questionnaire.Published{
			ID: versionID, QuestionnaireID: questionnaireID,
			StableKey: "tourism_profile", VersionNumber: 1, Revision: 3,
			Title: "Pesquisa turística", PrivacyNoticeVersion: "notice-v1",
			Questions: []questionnaire.PublicQuestion{{
				ID: questionID, StableKey: "first_visit", Prompt: "Primeira visita?",
				AnswerType: questionnaire.AnswerBoolean, DisplayOrder: 1,
			}},
		},
	})
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/questionnaires/tourism_profile/active",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{
		"data_classification", "retention_policy_code",
		"analytics_key", "public_aggregation_allowed", "minimum_public_cell",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public projection contains %q: %s", forbidden, body)
		}
	}
	if recorder.Header().Get("Cache-Control") != "no-store" ||
		recorder.Header().Get("ETag") != `"3"` {
		t.Fatalf("security headers = %v", recorder.Header())
	}
}

func TestSurveyCapabilityFailuresDoNotReachRepositoryOrRevealAuthority(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	handler := questionnaireHandler(questionnaireRepositoryStub{submitCalls: &calls})
	assertSurveyFailure(t, handler, "", http.StatusNotFound)
	if calls.Load() != 0 {
		t.Fatalf("repository calls for malformed authority = %d", calls.Load())
	}

	handler = questionnaireHandler(questionnaireRepositoryStub{
		submitErr: questionnaire.ErrCapabilityInvalid, submitCalls: &calls,
	})
	assertSurveyFailure(t, handler, "payload.signature", http.StatusNotFound)
	if calls.Load() != 1 {
		t.Fatalf("repository calls for shaped authority = %d, want 1", calls.Load())
	}
}

func assertSurveyFailure(
	t *testing.T,
	handler http.Handler,
	capability string,
	wantStatus int,
) {
	t.Helper()
	body := `{"questionnaire_version_id":"` + versionID.String() +
		`","client_submission_id":"019f0000-0000-7000-8000-000000000034",` +
		`"participation":"declined","answers":[],"consent_decisions":[]}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/survey-responses",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "survey-submit-key-1234")
	request.Header.Set("Survey-Capability", capability)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body)
	}
	if capability != "" && strings.Contains(recorder.Body.String(), capability) {
		t.Fatalf("problem response leaked capability: %s", recorder.Body)
	}
}

func questionnaireHandler(repository questionnaire.Repository) http.Handler {
	verifier := verifierFunc(func(_ context.Context, token string) (access.Principal, error) {
		scopes := map[string][]string{
			"editor":   {"questionnaires:manage"},
			"reviewer": {"questionnaires:approve"},
		}
		return access.NewPrincipal("https://issuer.invalid", token, scopes[token]), nil
	})
	handler, _ := httpapi.New(httpapi.Dependencies{
		Verifier:       verifier,
		Questionnaires: questionnaire.NewService(repository),
		CursorKeys: config.KeyringConfig{
			CurrentVersion: "cursor-v1",
			Keys: map[string][]byte{
				"cursor-v1": []byte("cursor-test-key-is-at-least-32-bytes"),
			},
		},
	})
	return handler
}
