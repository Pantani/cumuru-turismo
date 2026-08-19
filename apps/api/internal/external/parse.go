package external

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var errUpstreamPayloadInvalid = errors.New("upstream payload is invalid")

// ObservedPoint is one daily fact as the upstream reported it. The digest that
// decides idempotence is derived at write time by digestOf, not carried here:
// keeping a second copy on the struct would let the value and its digest drift
// apart. Equal digest is a no-op; a different one enters as a new revision,
// because ERA5 backfills data that was already published and replacing the
// value in place would erase the trail.
type ObservedPoint struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	Value       float64
}

// The daily block of Open-Meteo. Unknown fields are tolerated — the upstream
// adds them freely and turning every addition into a parse_error would be a
// self-inflicted outage — but a missing required field or an unexpected type
// is a parse_error, never silence.
type openMeteoPayload struct {
	Daily *openMeteoDaily `json:"daily"`
}

type openMeteoDaily struct {
	Time           []string   `json:"time"`
	TemperatureMax []*float64 `json:"temperature_2m_max"`
}

func parseOpenMeteoDaily(body []byte) ([]ObservedPoint, error) {
	daily, err := decodeOpenMeteoDaily(body)
	if err != nil {
		return nil, err
	}
	points := make([]ObservedPoint, 0, len(daily.Time))
	for index, day := range daily.Time {
		point, ok, err := dailyPoint(day, daily.TemperatureMax[index])
		if err != nil {
			return nil, err
		}
		if ok {
			points = append(points, point)
		}
	}
	return points, nil
}

func decodeOpenMeteoDaily(body []byte) (openMeteoDaily, error) {
	payload := openMeteoPayload{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return openMeteoDaily{}, errUpstreamPayloadInvalid
	}
	if payload.Daily == nil || !alignedDaily(*payload.Daily) {
		return openMeteoDaily{}, errUpstreamPayloadInvalid
	}
	return *payload.Daily, nil
}

// A daily block with no days, or with one value fewer than it has days, is
// malformed rather than empty: pairing them by index would silently attach a
// number to the wrong date.
func alignedDaily(daily openMeteoDaily) bool {
	return len(daily.Time) > 0 &&
		len(daily.Time) == len(daily.TemperatureMax)
}

// A null inside the array is the source declaring it has no number for that
// day, which is documented upstream behaviour and not a malformation. The day
// is skipped instead of stored as a half fact.
func dailyPoint(day string, value *float64) (ObservedPoint, bool, error) {
	start, err := time.ParseInLocation(time.DateOnly, day, bahia)
	if err != nil {
		return ObservedPoint{}, false, errUpstreamPayloadInvalid
	}
	if value == nil {
		return ObservedPoint{}, false, nil
	}
	return ObservedPoint{
		PeriodStart: start,
		PeriodEnd:   start.AddDate(0, 0, 1),
		Value:       *value,
	}, true, nil
}

// Digest binds the identity of the fact — series, period and value — and
// nothing else. It is what makes a second identical cycle write nothing, and a
// revised value write a new revision.
func digestOf(target Target, point ObservedPoint) string {
	sum := sha256.Sum256([]byte(
		target.SourceCode + "\n" +
			target.Series.SeriesCode + "\n" +
			point.PeriodStart.UTC().Format(time.RFC3339) + "\n" +
			strconv.FormatFloat(point.Value, 'f', -1, 64),
	))
	return hex.EncodeToString(sum[:])
}
