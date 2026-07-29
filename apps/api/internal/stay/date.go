package stay

import (
	"errors"
	"time"
)

const civilDateLayout = "2006-01-02"

var (
	bahiaLocation  = mustBahiaLocation()
	ErrInvalidDate = errors.New("invalid civil date")
)

type CivilDate struct {
	value time.Time
}

func ParseCivilDate(value string) (CivilDate, error) {
	parsed, err := time.ParseInLocation(civilDateLayout, value, bahiaLocation)
	if err != nil || parsed.Format(civilDateLayout) != value {
		return CivilDate{}, ErrInvalidDate
	}
	return CivilDate{value: parsed}, nil
}

func MustCivilDate(value string) CivilDate {
	parsed, err := ParseCivilDate(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func CivilDateFromInstant(value time.Time) CivilDate {
	local := value.In(bahiaLocation)
	return CivilDate{
		value: time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bahiaLocation),
	}
}

func (d CivilDate) String() string {
	return d.value.Format(civilDateLayout)
}

func (d CivilDate) Before(other CivilDate) bool {
	return d.value.Before(other.value)
}

func (d CivilDate) Equal(other CivilDate) bool {
	return d.value.Equal(other.value)
}

func (d CivilDate) AddDays(days int) CivilDate {
	return CivilDate{value: d.value.AddDate(0, 0, days)}
}

func (d CivilDate) Start() time.Time {
	return d.value
}

func mustBahiaLocation() *time.Location {
	location, err := time.LoadLocation("America/Bahia")
	if err != nil {
		panic(err)
	}
	return location
}
