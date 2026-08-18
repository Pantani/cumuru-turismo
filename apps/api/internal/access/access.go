package access

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidToken = errors.New("invalid token")

const (
	DevelopmentPlatformToken            = "cumuru-local-platform-read"
	DevelopmentQuestionnaireEditorToken = "cumuru-local-questionnaire-editor"
	DevelopmentQuestionnaireReviewToken = "cumuru-local-questionnaire-reviewer"
	DevelopmentAnalyticsQualityToken    = "cumuru-local-analytics-quality"
)

type Principal struct {
	Issuer  string
	Subject string
	scopes  map[string]struct{}
}

func NewPrincipal(issuer, subject string, scopes []string) Principal {
	normalized := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			normalized[scope] = struct{}{}
		}
	}
	return Principal{
		Issuer:  strings.TrimSuffix(strings.TrimSpace(issuer), "/"),
		Subject: strings.TrimSpace(subject),
		scopes:  normalized,
	}
}

func (p Principal) HasScope(scope string) bool {
	_, ok := p.scopes[scope]
	return ok
}

type Verifier interface {
	Verify(context.Context, string) (Principal, error)
}

type FixtureCredentialVerifier interface {
	Verifier
	IsFixtureCredential(string) bool
}

type developmentFake struct {
	issuer string
}

func NewDevelopmentFake(environment, issuer string) (Verifier, error) {
	if environment != "local" && environment != "test" {
		return nil, errors.New("development OIDC verifier is not allowed in this environment")
	}
	issuer = strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return nil, errors.New("development OIDC issuer is required")
	}
	return &developmentFake{issuer: issuer}, nil
}

func (f *developmentFake) Verify(_ context.Context, token string) (Principal, error) {
	subject, scopes, ok := developmentFixture(token)
	if !ok {
		return Principal{}, ErrInvalidToken
	}
	return NewPrincipal(f.issuer, subject, scopes), nil
}

func (f *developmentFake) IsFixtureCredential(token string) bool {
	_, _, ok := developmentFixture(token)
	return ok
}

func developmentFixture(token string) (string, []string, bool) {
	switch token {
	case DevelopmentPlatformToken:
		// No accommodations:onboard here, and the token name says why: this is
		// cumuru-local-platform-read, and its three siblings below each carry a
		// single scope on purpose. Onboarding admits an establishment to the whole
		// platform — the invite queue of ADR-042 filters by state, never by
		// membership — so granting it to a bearer string would hand back over this
		// door the power the local demo operator no longer has through its
		// password account, in the very runtime where that separation is
		// demonstrated.
		return "fixture-operator", []string{
			"platform:read",
			"accommodations:manage",
			"stays:read:own",
			"stays:write",
		}, true
	case DevelopmentQuestionnaireEditorToken:
		return "fixture-questionnaire-editor", []string{"questionnaires:manage"}, true
	case DevelopmentQuestionnaireReviewToken:
		return "fixture-questionnaire-reviewer", []string{"questionnaires:approve"}, true
	case DevelopmentAnalyticsQualityToken:
		return "fixture-analytics-quality", []string{"analytics:read:internal"}, true
	default:
		return "", nil, false
	}
}
