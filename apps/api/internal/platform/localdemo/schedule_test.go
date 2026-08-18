package localdemo

import (
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

func fixtureLocation(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("America/Bahia")
	if err != nil {
		t.Fatal(err)
	}
	return location
}

// The seed is re-applied whenever somebody brings the stack up. A schedule
// derived from the run date would move every arrival by a day each morning, so
// the second run would rewrite the history instead of extending it.
func TestScheduleKeepsEveryStayItAlreadyWroteWhenTheWindowSlides(t *testing.T) {
	t.Parallel()

	location := fixtureLocation(t)
	now := time.Date(2026, time.August, 18, 15, 0, 0, 0, time.UTC)
	today := historicalByKey(stayFixtures(now, location))
	tomorrow := historicalByKey(stayFixtures(now.AddDate(0, 0, 1), location))

	for key, first := range today {
		if second, carried := tomorrow[key]; carried {
			assertSameStay(t, key, first, second)
		}
	}
	if len(tomorrow) < len(today) {
		t.Fatalf("history shrank: %d then %d", len(today), len(tomorrow))
	}
}

func assertSameStay(t *testing.T, key string, first, second stayFixture) {
	t.Helper()
	if !first.arrival.Equal(second.arrival) ||
		!first.departure.Equal(second.departure) {
		t.Fatalf("stay %q was rescheduled by a later run", key)
	}
	if first.guestCount != second.guestCount ||
		first.responseCategory != second.responseCategory {
		t.Fatalf("stay %q changed its group by a later run", key)
	}
}

// Every window the cover offers reads the same published series, so a hole
// anywhere in the two years is a hole in some window the reader can select.
func TestScheduleCoversThePublishedHistoryDenselyEnoughToPublish(t *testing.T) {
	t.Parallel()

	location := fixtureLocation(t)
	now := time.Date(2026, time.August, 18, 15, 0, 0, 0, time.UTC)
	occupancy := dailyOccupancy(stayFixtures(now, location))
	today := civilDay(now, location)
	publishable := 0
	for day := 1; day <= analytics.PresenceHistoryDays; day++ {
		date := today.AddDate(0, 0, -day).Format(time.DateOnly)
		measured := occupancy[date]
		if measured.guests == 0 {
			t.Fatalf("no visitor on %s", date)
		}
		if measured.guests >= 10 && len(measured.accommodations) >= 3 {
			publishable++
		}
	}
	// The thin days are wanted — they are what shows a protected cell — but the
	// series has to be publishable almost everywhere or the chart is empty.
	if publishable < analytics.PresenceHistoryDays*95/100 {
		t.Fatalf("publishable days = %d of %d", publishable, analytics.PresenceHistoryDays)
	}
}

func TestScheduleVariesSeasonAndAnswerAcrossTheHistory(t *testing.T) {
	t.Parallel()

	location := fixtureLocation(t)
	now := time.Date(2026, time.August, 18, 15, 0, 0, 0, time.UTC)
	fixtures := stayFixtures(now, location)
	answers := map[string]int{}
	for _, fixture := range fixtures {
		answers[fixture.responseCategory]++
	}
	for _, category := range []string{"first_visit", "returning", ""} {
		if answers[category] == 0 {
			t.Fatalf("no stay with response %q", category)
		}
	}
	assertSeasonalAmplitude(t, dailyOccupancy(fixtures))
}

// January is high season and May is low season on this coast. Without that
// contrast the published chart is a flat line and the demo teaches nothing.
func assertSeasonalAmplitude(t *testing.T, occupancy map[string]dayOccupancy) {
	t.Helper()
	high := monthGuests(occupancy, "2026-01")
	low := monthGuests(occupancy, "2026-05")
	if high < low*2 {
		t.Fatalf("season amplitude too flat: january=%d may=%d", high, low)
	}
}

func TestCurrentStayFixturesCoverForecastHorizon(t *testing.T) {
	t.Parallel()

	location := fixtureLocation(t)
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, location)
	today := civilDay(now, location)
	horizonEnd := today.AddDate(0, 0, analytics.PresenceForecastDays)
	current := currentStayFixtures(now, today, foundationFixture().Accommodations)
	if len(current) != len(foundationFixture().Accommodations) {
		t.Fatalf("current stays = %d", len(current))
	}
	for _, fixture := range current {
		if !fixture.departure.After(horizonEnd) {
			t.Fatalf(
				"current stay %q ends before forecast horizon: %s",
				fixture.key,
				fixture.departure,
			)
		}
	}
}

// The group is what the FNRH record is made of, so an invalid one would fail
// mid-seed instead of at the boundary that owns the rule.
func TestFixtureVisitorsFormValidGroupsOfTheDeclaredSize(t *testing.T) {
	t.Parallel()

	location := fixtureLocation(t)
	now := time.Date(2026, time.August, 18, 15, 0, 0, 0, time.UTC)
	countries := map[string]int{}
	bands := map[stay.AgeBand]int{}
	for _, fixture := range stayFixtures(now, location) {
		visitors := fixtureVisitors(fixture)
		if int32(len(visitors)) != fixture.guestCount {
			t.Fatalf("stay %q has %d visitors, want %d",
				fixture.key, len(visitors), fixture.guestCount)
		}
		if err := stay.ValidateGroup(visitors); err != nil {
			t.Fatalf("stay %q group invalid: %v", fixture.key, err)
		}
		for _, visitor := range visitors {
			countries[visitor.ResidenceCountry]++
			bands[visitor.AgeBand]++
		}
	}
	if len(countries) < 5 {
		t.Fatalf("residence countries = %d, want at least 5", len(countries))
	}
	if len(bands) != len(adultAgeBands)+len(minorAgeBands) {
		t.Fatalf("age bands = %d, want every band", len(bands))
	}
}

type dayOccupancy struct {
	guests         int
	accommodations map[string]bool
}

func dailyOccupancy(fixtures []stayFixture) map[string]dayOccupancy {
	result := make(map[string]dayOccupancy, analytics.PresenceHistoryDays)
	for _, fixture := range fixtures {
		for day := fixture.arrival; day.Before(fixture.departure); day = day.AddDate(0, 0, 1) {
			result[day.Format(time.DateOnly)] = withStay(
				result[day.Format(time.DateOnly)], fixture,
			)
		}
	}
	return result
}

func withStay(measured dayOccupancy, fixture stayFixture) dayOccupancy {
	if measured.accommodations == nil {
		measured.accommodations = map[string]bool{}
	}
	measured.guests += int(fixture.guestCount)
	measured.accommodations[fixture.accommodationID.String()] = true
	return measured
}

func monthGuests(occupancy map[string]dayOccupancy, month string) int {
	total := 0
	for date, measured := range occupancy {
		if date[:len(month)] == month {
			total += measured.guests
		}
	}
	return total
}

func historicalByKey(fixtures []stayFixture) map[string]stayFixture {
	result := make(map[string]stayFixture, len(fixtures))
	for _, fixture := range fixtures {
		if fixture.keepCheckedIn {
			continue
		}
		result[fixture.key] = fixture
	}
	return result
}
