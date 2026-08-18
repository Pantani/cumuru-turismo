package access

import (
	"slices"
	"testing"
)

type fixtureScopeCase struct {
	name    string
	token   string
	subject string
	scopes  []string
}

// The external test can only ask Principal whether it holds a scope, one name at
// a time, so a scope nobody thought to deny would ride along unnoticed. Here the
// fixture table itself is in reach: the assertion is the whole slice, and any
// scope added to a fixture has to be added here too, on purpose and in review.
//
// This matters more for these tokens than for an account, because they are
// constant strings accepted by the chained verifier alongside the password
// session: whatever they carry is reachable by anyone who can read the source.
var fixtureScopeCases = []fixtureScopeCase{
	{
		name:    "platform",
		token:   DevelopmentPlatformToken,
		subject: DevelopmentPlatformSubject,
		// No accommodations:onboard: admitting an establishment is the
		// administrator's act, and this fixture must not be a second door to it.
		scopes: []string{
			"platform:read",
			"accommodations:manage",
			"stays:read:own",
			"stays:write",
		},
	},
	{
		name:    "questionnaire_editor",
		token:   DevelopmentQuestionnaireEditorToken,
		subject: "fixture-questionnaire-editor",
		scopes:  []string{"questionnaires:manage"},
	},
	{
		name:    "questionnaire_reviewer",
		token:   DevelopmentQuestionnaireReviewToken,
		subject: "fixture-questionnaire-reviewer",
		scopes:  []string{"questionnaires:approve"},
	},
	{
		name:    "analytics_quality",
		token:   DevelopmentAnalyticsQualityToken,
		subject: "fixture-analytics-quality",
		scopes:  []string{"analytics:read:internal"},
	},
}

func TestDevelopmentFixturesGrantExactlyTheseScopes(t *testing.T) {
	t.Parallel()

	for _, testCase := range fixtureScopeCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertFixtureScopes(t, testCase)
		})
	}
}

func assertFixtureScopes(t *testing.T, testCase fixtureScopeCase) {
	t.Helper()

	subject, scopes, ok := developmentFixture(testCase.token)
	if !ok {
		t.Fatalf("developmentFixture(%q) not recognised", testCase.token)
	}
	if subject != testCase.subject {
		t.Errorf("subject = %q, want %q", subject, testCase.subject)
	}
	if !slices.Equal(scopes, testCase.scopes) {
		t.Errorf("scopes = %v, want %v", scopes, testCase.scopes)
	}
}
