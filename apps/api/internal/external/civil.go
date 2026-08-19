package external

import "time"

// bahia is resolved once. A missing zone database is a broken build, not a
// runtime branch: every period this layer publishes is anchored to the civil
// calendar of America/Bahia, so silently falling back to UTC would shift every
// day boundary by three hours without anybody noticing.
var bahia = mustBahiaLocation()

func mustBahiaLocation() *time.Location {
	location, err := time.LoadLocation(PublicTimeZone)
	if err != nil {
		panic(err)
	}
	return location
}

// instant renders an absolute instant. UTC, so that two runs of the same
// document produce byte-identical JSON and the ETag stays stable.
func instant(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

// civilDayBounds is the half-open civil day of America/Bahia containing the
// given instant.
func civilDayBounds(now time.Time) (string, string) {
	local := now.In(bahia)
	start := time.Date(
		local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bahia,
	)
	return instant(start), instant(start.AddDate(0, 0, 1))
}
