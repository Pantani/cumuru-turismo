package access_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

func TestDevelopmentFakeAcceptsLocalAndTest(t *testing.T) {
	t.Parallel()

	assertDevelopmentFakeAccepted(t, "local")
	assertDevelopmentFakeAccepted(t, "test")
}

func assertDevelopmentFakeAccepted(t *testing.T, environment string) {
	t.Helper()
	verifier, err := access.NewDevelopmentFake(environment, "https://oidc.invalid/local")
	if err != nil {
		t.Fatalf("NewDevelopmentFake() error = %v", err)
	}
	principal, err := verifier.Verify(context.Background(), access.DevelopmentPlatformToken)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	for _, scope := range []string{
		"platform:read",
		"accommodations:onboard",
		"accommodations:manage",
		"stays:read:own",
		"stays:write",
	} {
		if !principal.HasScope(scope) {
			t.Errorf("development principal lacks %s", scope)
		}
	}
}

func TestDevelopmentFakeRejectsNonDevelopmentEnvironments(t *testing.T) {
	t.Parallel()

	for _, environment := range []string{"", "preview", "staging", "production"} {
		t.Run("reject_"+environment, func(t *testing.T) {
			t.Parallel()
			if _, err := access.NewDevelopmentFake(environment, "https://oidc.invalid/local"); err == nil {
				t.Fatal("NewDevelopmentFake() error = nil")
			}
		})
	}
}

func TestDevelopmentFakeRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatalf("NewDevelopmentFake() error = %v", err)
	}
	_, err = verifier.Verify(context.Background(), "not-the-fixture-token")
	if !errors.Is(err, access.ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestDevelopmentFakeClassifiesOnlyItsFixtureCredentials(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatalf("NewDevelopmentFake() error = %v", err)
	}
	classifier, ok := verifier.(interface {
		IsFixtureCredential(string) bool
	})
	if !ok {
		t.Fatal("development verifier does not classify fixture credentials")
	}
	for _, token := range []string{
		access.DevelopmentPlatformToken,
		access.DevelopmentQuestionnaireEditorToken,
		access.DevelopmentQuestionnaireReviewToken,
		access.DevelopmentAnalyticsQualityToken,
	} {
		if !classifier.IsFixtureCredential(token) {
			t.Errorf("IsFixtureCredential(%q) = false", token)
		}
	}
	if classifier.IsFixtureCredential("institutional-token") {
		t.Fatal("institutional token classified as a fixture credential")
	}
}

func TestDevelopmentFakeSeparatesQuestionnaireEditorAndReviewer(t *testing.T) {
	t.Parallel()
	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatal(err)
	}
	editor, err := verifier.Verify(context.Background(), access.DevelopmentQuestionnaireEditorToken)
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := verifier.Verify(context.Background(), access.DevelopmentQuestionnaireReviewToken)
	if err != nil {
		t.Fatal(err)
	}
	if !editor.HasScope("questionnaires:manage") || editor.HasScope("questionnaires:approve") {
		t.Fatal("editor scopes are not separated")
	}
	if !reviewer.HasScope("questionnaires:approve") || reviewer.HasScope("questionnaires:manage") {
		t.Fatal("reviewer scopes are not separated")
	}
	if editor.Subject == reviewer.Subject {
		t.Fatal("editor and reviewer share a subject")
	}
}

func TestDevelopmentFakeAnalyticsTokenHasOnlyInternalQualityScope(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(
		context.Background(),
		access.DevelopmentAnalyticsQualityToken,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.HasScope("analytics:read:internal") {
		t.Fatal("analytics principal lacks analytics:read:internal")
	}
	for _, forbidden := range []string{
		"platform:read", "accommodations:manage", "stays:read:own",
		"stays:write", "questionnaires:manage", "questionnaires:approve",
	} {
		if principal.HasScope(forbidden) {
			t.Fatalf("analytics principal unexpectedly has %s", forbidden)
		}
	}
}

// The static development token used to answer with the same subject as the
// local demo operator persona, which made both indistinguishable in the audit
// trail. The subject must stay a dedicated probe identity, unique among the
// fixture credentials.
func TestDevelopmentFakePlatformTokenHasItsOwnSubject(t *testing.T) {
	t.Parallel()

	verifier, err := access.NewDevelopmentFake("test", "https://oidc.invalid/local")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, token := range []string{
		access.DevelopmentPlatformToken,
		access.DevelopmentQuestionnaireEditorToken,
		access.DevelopmentQuestionnaireReviewToken,
		access.DevelopmentAnalyticsQualityToken,
	} {
		principal, err := verifier.Verify(context.Background(), token)
		if err != nil {
			t.Fatalf("Verify(%q) error = %v", token, err)
		}
		if other, ok := seen[principal.Subject]; ok {
			t.Fatalf("%q and %q share subject %q", token, other, principal.Subject)
		}
		seen[principal.Subject] = token
	}
	if seen[access.DevelopmentPlatformSubject] != access.DevelopmentPlatformToken {
		t.Fatalf("platform token subject = %v", seen)
	}
}
