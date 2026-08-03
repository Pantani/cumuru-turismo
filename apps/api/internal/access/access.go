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
	switch token {
	case DevelopmentPlatformToken:
		return NewPrincipal(f.issuer, "fixture-operator", []string{
			"platform:read",
			"accommodations:onboard",
			"accommodations:manage",
			"stays:read:own",
			"stays:write",
		}), nil
	case DevelopmentQuestionnaireEditorToken:
		return NewPrincipal(
			f.issuer,
			"fixture-questionnaire-editor",
			[]string{"questionnaires:manage"},
		), nil
	case DevelopmentQuestionnaireReviewToken:
		return NewPrincipal(
			f.issuer,
			"fixture-questionnaire-reviewer",
			[]string{"questionnaires:approve"},
		), nil
	case DevelopmentAnalyticsQualityToken:
		return NewPrincipal(
			f.issuer,
			"fixture-analytics-quality",
			[]string{"analytics:read:internal"},
		), nil
	default:
		return Principal{}, ErrInvalidToken
	}
}
