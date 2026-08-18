package localdemo

import (
	"maps"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
)

func TestFoundationFixtureSpansManyDistinctAccommodations(t *testing.T) {
	t.Parallel()

	fixture := foundationFixture()
	if len(fixture.Accommodations) != 16 {
		t.Fatalf("accommodations = %d, want 16", len(fixture.Accommodations))
	}
	seenIDs := make(map[string]bool, len(fixture.Accommodations))
	seenCategories := make(map[string]bool, len(fixture.Accommodations))
	for index, accommodation := range fixture.Accommodations {
		assertFoundationAccommodation(t, index, accommodation)
		if seenIDs[accommodation.ID.String()] {
			t.Fatalf("accommodation %d repeats identifier %s", index, accommodation.ID)
		}
		seenIDs[accommodation.ID.String()] = true
		seenCategories[accommodation.Category] = true
	}
	// A demo where every house is a pousada cannot show what the cover does
	// when categories of different sizes report on the same day.
	assertReportableCategories(t, fixture.Accommodations)
}

// The coverage panel needs a minimum of reporting accommodations per category,
// so a category the catalogue declares only once can never be published.
func assertReportableCategories(
	t *testing.T,
	accommodations []store.LocalDemoAccommodation,
) {
	t.Helper()
	counts := make(map[string]int, len(accommodations))
	for _, accommodation := range accommodations {
		counts[accommodation.Category]++
	}
	if len(counts) < 5 {
		t.Fatalf("categories = %d, want at least 5", len(counts))
	}
	for category, count := range counts {
		if count < 3 {
			t.Fatalf("category %q has %d accommodations, want at least 3", category, count)
		}
	}
}

var fixtureCategories = map[string]bool{
	"formal_lodging": true, "seasonal_rental": true, "family_hosting": true,
	"camping": true, "regularizing": true, "other": true, "unclassified": true,
}

func assertFoundationAccommodation(
	t *testing.T,
	index int,
	accommodation store.LocalDemoAccommodation,
) {
	t.Helper()
	if !fixtureCategories[accommodation.Category] {
		t.Errorf("accommodation %d category = %q", index, accommodation.Category)
	}
	if accommodation.Capacity <= 0 {
		t.Errorf("accommodation %d capacity = %d", index, accommodation.Capacity)
	}
	if index == 0 {
		if accommodation.CadasturID == nil ||
			*accommodation.CadasturID != "CADASTUR-FICTICIO-NAO-VALIDO" {
			t.Errorf("pousada Cadastur = %v", accommodation.CadasturID)
		}
		return
	}
	if accommodation.CadasturID != nil {
		t.Errorf("accommodation %d has Cadastur %q", index, *accommodation.CadasturID)
	}
}

func TestQuestionnaireDefinitionAcceptsOnlyKnownLegacyRetentionAlias(
	t *testing.T,
) {
	t.Parallel()

	expected := questionnaireDefinition()
	legacy := questionnaireDefinition()
	legacy.Questions[0].RetentionPolicyCode = "prototype-aggregate-only"
	if !definitionsEqual(legacy, expected) {
		t.Fatal("known pre-release retention alias should converge")
	}

	unknown := questionnaireDefinition()
	unknown.Questions[0].RetentionPolicyCode = "different-policy"
	if definitionsEqual(unknown, expected) {
		t.Fatal("unknown published definition must fail closed")
	}
}

func TestFixtureSummaryIsDerivedFromFoundationAndResponseFixtures(t *testing.T) {
	t.Parallel()

	foundation := foundationFixture()
	foundation.Accommodations = foundation.Accommodations[:2]
	fixtures := []stayFixture{
		{responseCategory: "first_visit", guestCount: 4},
		{responseCategory: "", guestCount: 2},
		{responseCategory: "returning", guestCount: 3},
	}
	got := fixtureSummary(foundation, fixtures)
	want := "organizations=1 accommodations=2 stays=3 visitors=9 responses=2"
	if got != want {
		t.Fatalf("fixtureSummary() = %q, want %q", got, want)
	}
}

func TestDemoOperatorCanReachTheSelfServicePanels(t *testing.T) {
	t.Parallel()

	// The whole set, not the required members: a containment check stays green
	// while a privileged scope is added beside the ones listed, and this is the
	// account a person actually signs into on the demo. Anything granted here
	// has to be justified here.
	want := map[string]struct{}{
		"platform:read":           {},
		"accommodations:manage":   {},
		"stays:read:own":          {},
		"stays:write":             {},
		"stays:approve":           {},
		"questionnaires:manage":   {},
		"questionnaires:approve":  {},
		"analytics:read:internal": {},
	}
	granted := demoOperatorScopes(t)
	if !maps.Equal(granted, want) {
		t.Fatalf("demo operator scopes = %v, want %v", granted, want)
	}
}

// accommodations:onboard is the only scope separating this fixture from the
// seeded administrator, and nothing else in the codebase would fail if it came
// back: the demo would simply let a lodging operator admit establishments to the
// whole platform, because the invite queue of ADR-042 is not scoped by
// membership. This assertion is that missing failure.
func TestDemoOperatorCannotOnboardAccommodations(t *testing.T) {
	t.Parallel()

	if _, ok := demoOperatorScopes(t)["accommodations:onboard"]; ok {
		t.Fatal("the demo operator must not hold accommodations:onboard")
	}
}

func demoOperatorScopes(t *testing.T) map[string]struct{} {
	t.Helper()

	account, err := accountFixture(func(string) (string, bool) {
		return "senha-fictícia-do-demo", true
	})
	if err != nil {
		t.Fatalf("accountFixture() error = %v", err)
	}
	result := make(map[string]struct{}, len(account.Scopes))
	for _, scope := range account.Scopes {
		result[scope] = struct{}{}
	}
	return result
}
