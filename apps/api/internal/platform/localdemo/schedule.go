package localdemo

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

// fixtureEpoch anchors every chain to the civil calendar instead of to the run
// date. An arrival therefore falls on the same date on every run: seeding again
// tomorrow keeps the stays already written and only appends the days that
// elapsed, which is what makes a sliding two-year window idempotent.
//
// The anchor is resolved in the fixture location, never in UTC: a midnight
// instant in UTC is still the previous evening in America/Bahia, so an epoch
// declared as an instant would silently anchor the calendar one day off the
// date it names.
func fixtureEpoch(location *time.Location) time.Time {
	return time.Date(2023, time.January, 1, 0, 0, 0, 0, location)
}

// currentStayHorizonDays keeps the in-progress cohort past the forecast
// horizon, so the published forecast always has a source.
const currentStayHorizonDays = 35

// foreignSharePercent is the share of groups that report a residence outside
// Brazil, which is what makes the origin mix more than a single state.
const foreignSharePercent = 18

// season shapes one stretch of the year. Occupancy on the coast is seasonal, so
// a fixture calendar with a flat rate would publish a flat chart and teach the
// reader nothing about how the observatory behaves in high season.
type season struct {
	minNights       int
	nightsSpan      int
	minGap          int
	gapSpan         int
	minGuests       int
	guestsSpan      int
	firstVisitShare int
	responseShare   int
}

var (
	highSeason = season{
		minNights: 5, nightsSpan: 6, minGap: 0, gapSpan: 3,
		minGuests: 4, guestsSpan: 4,
		firstVisitShare: 62, responseShare: 78,
	}
	shoulderSeason = season{
		minNights: 4, nightsSpan: 4, minGap: 1, gapSpan: 4,
		minGuests: 3, guestsSpan: 3,
		firstVisitShare: 45, responseShare: 66,
	}
	lowSeason = season{
		minNights: 3, nightsSpan: 4, minGap: 2, gapSpan: 4,
		minGuests: 2, guestsSpan: 3,
		firstVisitShare: 31, responseShare: 54,
	}
)

// host is the accommodation side of a schedule: the chain needs its identity to
// key the stays and its capacity to size the groups.
type host struct {
	ordinal  int
	id       uuid.UUID
	capacity int32
}

type origin struct {
	state string
	city  string
}

// Fictional but well-formed residences: a Brazilian visitor carries a state and
// an IBGE municipality code, and a foreign one carries neither.
var domesticOrigins = []origin{
	{state: "BA", city: "2925509"},
	{state: "BA", city: "2927408"},
	{state: "SP", city: "3550308"},
	{state: "MG", city: "3106200"},
	{state: "RJ", city: "3304557"},
	{state: "DF", city: "5300108"},
	{state: "PR", city: "4106902"},
	{state: "RS", city: "4314902"},
	{state: "PE", city: "2611606"},
	{state: "GO", city: "5208707"},
}

var foreignOrigins = []string{"PT", "AR", "FR", "DE", "US", "CL", "UY", "IT"}

var adultAgeBands = []stay.AgeBand{
	stay.Age18To24,
	stay.Age25To34,
	stay.Age35To44,
	stay.Age45To59,
	stay.Age60Plus,
}

var minorAgeBands = []stay.AgeBand{stay.Age0To5, stay.Age6To11, stay.Age12To17}

// stayFixtures covers the whole published history — the same two years the
// public window reads — so every window of the cover, month picker included,
// has observed days instead of a hole.
func stayFixtures(now time.Time, location *time.Location) []stayFixture {
	today := civilDay(now, location)
	windowStart := today.AddDate(0, 0, -analytics.PresenceHistoryDays)
	accommodations := foundationFixture().Accommodations
	result := make([]stayFixture, 0, len(accommodations)*128)
	for index, accommodation := range accommodations {
		result = append(result, historicalStayFixtures(
			fixtureHost(index, accommodation),
			windowStart,
			today,
			location,
		)...)
	}
	return append(result, currentStayFixtures(now, today, accommodations)...)
}

func fixtureHost(index int, accommodation store.LocalDemoAccommodation) host {
	return host{
		ordinal:  index + 1,
		id:       accommodation.ID,
		capacity: accommodation.Capacity,
	}
}

// The chain walks from the epoch and only emits what the window still reads.
// Walking from the window start instead would move every arrival by a day each
// morning and rewrite the whole history on every run.
func historicalStayFixtures(
	accommodation host,
	windowStart time.Time,
	today time.Time,
	location *time.Location,
) []stayFixture {
	result := make([]stayFixture, 0, 128)
	arrival := chainStart(accommodation, location)
	for arrival.Before(today) {
		profile := seasonOf(arrival)
		departure := arrival.AddDate(0, 0, nightsOf(profile, accommodation, arrival))
		if !departure.Before(today) {
			return result
		}
		// A stay that started before the window but ends inside it still puts
		// visitors on the first days the window reads; excluding it would open
		// the history with a hole that looks like a quiet week.
		if departure.After(windowStart) {
			result = append(result, historicalStayFixture(
				accommodation, profile, arrival, departure, location,
			))
		}
		arrival = departure.AddDate(0, 0, gapOf(profile, accommodation, departure))
	}
	return result
}

func chainStart(accommodation host, location *time.Location) time.Time {
	start := fixtureEpoch(location)
	return start.AddDate(0, 0, variant(scheduleSeed(accommodation, start, "start"), 9))
}

func historicalStayFixture(
	accommodation host,
	profile season,
	arrival time.Time,
	departure time.Time,
	location *time.Location,
) stayFixture {
	key := historicalStayKey(accommodation, arrival)
	return stayFixture{
		key:             key,
		accommodationID: accommodation.id,
		arrival:         arrival,
		departure:       departure,
		// The whole journey is replayed at the moment the group leaves, so the
		// survey response lands in the month the stay actually happened.
		clock:            civilNoon(departure, location),
		guestCount:       guestsOf(profile, accommodation, key),
		responseCategory: responseCategory(profile, key),
	}
}

func historicalStayKey(accommodation host, arrival time.Time) string {
	return fmt.Sprintf(
		"stay-%02d-%s",
		accommodation.ordinal,
		arrival.Format("2006-01-02"),
	)
}

// One group per accommodation is still checked in, so the operator workspace
// opens with occupied rooms and the forecast has a source.
func currentStayFixtures(
	now time.Time,
	today time.Time,
	accommodations []store.LocalDemoAccommodation,
) []stayFixture {
	result := make([]stayFixture, 0, len(accommodations))
	for index, accommodation := range accommodations {
		current := fixtureHost(index, accommodation)
		key := fmt.Sprintf("current-%02d", current.ordinal)
		result = append(result, stayFixture{
			key:             key,
			accommodationID: current.id,
			arrival:         today.AddDate(0, 0, -2),
			departure:       today.AddDate(0, 0, currentStayHorizonDays),
			clock:           now,
			guestCount:      guestsOf(seasonOf(today), current, key),
			keepCheckedIn:   true,
		})
	}
	return result
}

// The high season follows the Brazilian holiday calendar on the coast; May,
// June and August are the quiet months between them.
func seasonOf(day time.Time) season {
	switch day.Month() {
	case time.December, time.January, time.February, time.March, time.July:
		return highSeason
	case time.May, time.June, time.August:
		return lowSeason
	default:
		return shoulderSeason
	}
}

func nightsOf(profile season, accommodation host, arrival time.Time) int {
	return profile.minNights +
		variant(scheduleSeed(accommodation, arrival, "nights"), profile.nightsSpan)
}

func gapOf(profile season, accommodation host, departure time.Time) int {
	return profile.minGap +
		variant(scheduleSeed(accommodation, departure, "gap"), profile.gapSpan)
}

// A larger house takes larger groups, so the daily total follows the catalogue
// instead of treating a camping ground and a spare room as the same size.
func guestsOf(profile season, accommodation host, key string) int32 {
	guests := int32(profile.minGuests + variant(key+":guests", profile.guestsSpan))
	guests += accommodation.capacity / 8
	if guests > accommodation.capacity {
		return accommodation.capacity
	}
	return guests
}

// Not every group answers the survey, and the ones that do answer differently
// in high season: a fixture where everybody answers the same way would publish
// a share that never moves.
func responseCategory(profile season, key string) string {
	if variant(key+":response", 100) >= profile.responseShare {
		return ""
	}
	if variant(key+":first-visit", 100) < profile.firstVisitShare {
		return "first_visit"
	}
	return "returning"
}

func fixtureVisitors(fixture stayFixture) []stay.Visitor {
	count := int(fixture.guestCount)
	result := make([]stay.Visitor, 0, count)
	for index := 1; index <= count; index++ {
		result = append(result, fixtureVisitor(fixture.key, index))
	}
	return result
}

// The group travels together, so residence is drawn once per stay and the age
// band once per visitor.
func fixtureVisitor(key string, index int) stay.Visitor {
	seed := fmt.Sprintf("%s:%d", key, index)
	age, role := visitorProfile(index, seed)
	visitor := stay.Visitor{
		ClientID: deterministicUUID("visitor-"+key, index).String(),
		Role:     role,
		AgeBand:  age,
	}
	return withResidence(visitor, key)
}

// The responsible visitor is always an adult; roughly one companion in four is
// a minor, which is what puts the youngest bands in the published mix.
func visitorProfile(index int, seed string) (stay.AgeBand, stay.VisitorRole) {
	if index == 1 {
		return adultAge(seed), stay.VisitorResponsible
	}
	if variant(seed+":minor", 4) == 0 {
		return minorAgeBands[variant(seed+":age", len(minorAgeBands))], stay.VisitorMinor
	}
	return adultAge(seed), stay.VisitorCompanion
}

func adultAge(seed string) stay.AgeBand {
	return adultAgeBands[variant(seed+":age", len(adultAgeBands))]
}

func withResidence(visitor stay.Visitor, key string) stay.Visitor {
	if variant(key+":origin", 100) < foreignSharePercent {
		visitor.ResidenceCountry = foreignOrigins[variant(key+":country", len(foreignOrigins))]
		return visitor
	}
	residence := domesticOrigins[variant(key+":city", len(domesticOrigins))]
	visitor.ResidenceCountry = "BR"
	visitor.ResidenceState = residence.state
	visitor.ResidenceCityCode = residence.city
	return visitor
}

func scheduleSeed(accommodation host, day time.Time, field string) string {
	return fmt.Sprintf(
		"%02d:%s:%s",
		accommodation.ordinal,
		day.Format("2006-01-02"),
		field,
	)
}

// variant derives a stable offset from a fixture key: the calendar has to look
// irregular without ever depending on when the seeder ran.
func variant(seed string, span int) int {
	if span <= 0 {
		return 0
	}
	digest := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint32(digest[:4]) % uint32(span))
}
