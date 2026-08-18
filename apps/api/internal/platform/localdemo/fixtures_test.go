package localdemo

import (
	"strings"
	"testing"
	"time"
)

func TestFoundationFixtureUsesCanonicalCategoriesAndExplicitFakeCadastur(t *testing.T) {
	t.Parallel()

	fixture := foundationFixture()
	if len(fixture.Accommodations) != 4 {
		t.Fatalf("accommodations = %d, want 4", len(fixture.Accommodations))
	}
	wantCategories := []string{
		"formal_lodging",
		"formal_lodging",
		"seasonal_rental",
		"family_hosting",
	}
	for index, accommodation := range fixture.Accommodations {
		assertFoundationAccommodation(t, index, accommodation.Category, wantCategories[index], accommodation.CadasturID)
	}
}

func assertFoundationAccommodation(
	t *testing.T,
	index int,
	gotCategory string,
	wantCategory string,
	cadasturID *string,
) {
	t.Helper()
	if gotCategory != wantCategory {
		t.Errorf("accommodation %d category = %q, want %q", index, gotCategory, wantCategory)
	}
	if index == 0 {
		if cadasturID == nil || *cadasturID != "CADASTUR-FICTICIO-NAO-VALIDO" {
			t.Errorf("pousada Cadastur = %v", cadasturID)
		}
		return
	}
	if cadasturID != nil {
		t.Errorf("accommodation %d has Cadastur %q", index, *cadasturID)
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

func TestStayFixtureIdentityRollsHistoricalCohortsByMonth(
	t *testing.T,
) {
	t.Parallel()

	location, err := time.LoadLocation("America/Bahia")
	if err != nil {
		t.Fatal(err)
	}
	july := time.Date(2026, time.July, 29, 12, 0, 0, 0, location)
	nextDay := july.AddDate(0, 0, 1)
	august := time.Date(2026, time.August, 1, 12, 0, 0, 0, location)

	first := stayFixtures(july, location)
	sameMonth := stayFixtures(nextDay, location)
	nextMonth := stayFixtures(august, location)
	assertFixtureCounts(t, first, sameMonth, nextMonth)
	assertHistoricalFixtureKeys(t, first, sameMonth, nextMonth)
	assertCurrentFixtureKeys(t, first, sameMonth, nextMonth)
}

func TestFixtureSummaryIsDerivedFromFoundationAndResponseFixtures(t *testing.T) {
	t.Parallel()

	foundation := foundationFixture()
	foundation.Accommodations = foundation.Accommodations[:2]
	fixtures := []stayFixture{
		{responseCategory: "first_visit"},
		{responseCategory: ""},
		{responseCategory: "returning"},
	}
	got := fixtureSummary(foundation, fixtures)
	want := "organizations=1 accommodations=2 responses=2"
	if got != want {
		t.Fatalf("fixtureSummary() = %q, want %q", got, want)
	}
}

func TestCurrentStayFixturesCoverForecastHorizon(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("America/Bahia")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, location)
	today := civilDay(now, location)
	horizonEnd := today.AddDate(0, 0, 30)
	for _, fixture := range currentStayFixtures(now, today) {
		if !fixture.departure.After(horizonEnd) {
			t.Fatalf(
				"current stay %q ends before forecast horizon: %s",
				fixture.key,
				fixture.departure,
			)
		}
	}
}

func assertFixtureCounts(
	t *testing.T,
	fixtures ...[]stayFixture,
) {
	t.Helper()
	for _, fixture := range fixtures {
		if len(fixture) == 27 {
			continue
		}
		t.Fatalf(
			"fixture count = %d, want 27",
			len(fixture),
		)
	}
}

func assertHistoricalFixtureKeys(
	t *testing.T,
	first []stayFixture,
	sameMonth []stayFixture,
	nextMonth []stayFixture,
) {
	t.Helper()
	for index := 0; index < 24; index++ {
		assertSameMonthHistoricalFixture(t, first[index], sameMonth[index], index)
		assertRolledHistoricalKey(t, first[index], nextMonth[index], index)
		assertPreviousMonthIdentity(t, first[index].key)
	}
}

func assertSameMonthHistoricalFixture(
	t *testing.T,
	first stayFixture,
	sameMonth stayFixture,
	index int,
) {
	t.Helper()
	if first.key != sameMonth.key {
		t.Fatalf("historical key changed inside one civil month: %d", index)
	}
	if !first.arrival.Equal(sameMonth.arrival) ||
		!first.departure.Equal(sameMonth.departure) {
		t.Fatalf("historical schedule changed inside one civil month: %d", index)
	}
}

func assertRolledHistoricalKey(
	t *testing.T,
	first stayFixture,
	nextMonth stayFixture,
	index int,
) {
	t.Helper()
	if first.key == nextMonth.key {
		t.Fatalf("historical key did not roll at month boundary: %d", index)
	}
}

func assertPreviousMonthIdentity(t *testing.T, key string) {
	t.Helper()
	if !strings.Contains(key, "2026-06") {
		t.Fatalf("historical key lacks previous-month identity: %q", key)
	}
}

func assertCurrentFixtureKeys(
	t *testing.T,
	first []stayFixture,
	sameMonth []stayFixture,
	nextMonth []stayFixture,
) {
	t.Helper()
	for index := 24; index < 27; index++ {
		if first[index].key != sameMonth[index].key ||
			first[index].key != nextMonth[index].key {
			t.Fatalf("current stay identity must remain stable: %d", index)
		}
	}
}

// The client gates the self-service panels on stays:approve. Without it in the demo
// operator's set, local-demo-e2e walks a journey that never opens the approval
// queue or the poster panel and passes by absence of error — the failure mode
// D-01 already showed one layer down.
//
// The two vetoed scopes stay vetoed: they belong to other roles.
func TestDemoOperatorCanReachTheSelfServicePanels(t *testing.T) {
	t.Parallel()

	granted := demoOperatorScopes(t)
	for _, required := range []string{
		"accommodations:manage", "stays:read:own", "stays:write", "stays:approve",
	} {
		if _, ok := granted[required]; !ok {
			t.Fatalf("the demo operator lacks %s: %v", required, granted)
		}
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
