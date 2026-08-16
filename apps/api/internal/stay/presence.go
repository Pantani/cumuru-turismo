package stay

import (
	"errors"
	"time"
)

var ErrInvalidPresence = errors.New("invalid presence interval")

type PresenceKind string

const (
	PresenceForecast PresenceKind = "forecast"
	PresenceObserved PresenceKind = "observed"
)

type PresenceDay struct {
	Date CivilDate
	Kind PresenceKind
}

type Stay struct {
	Status           Status
	PlannedArrival   CivilDate
	PlannedDeparture CivilDate
	CheckedInAt      *time.Time
	CheckedOutAt     *time.Time
}

func PresenceDays(value Stay) ([]PresenceDay, error) {
	switch value.Status {
	case StatusPreRegistered:
		return presenceRange(value.PlannedArrival, value.PlannedDeparture, PresenceForecast)
	case StatusCheckedIn:
		return checkedInPresence(value)
	case StatusCheckedOut:
		return checkedOutPresence(value)
	case StatusDraft, StatusInvited, StatusCancelled, StatusNoShow:
		return []PresenceDay{}, nil
	default:
		return nil, ErrInvalidPresence
	}
}

func PresenceDaysAt(value Stay, asOf CivilDate) ([]PresenceDay, error) {
	if value.Status != StatusCheckedIn {
		return PresenceDays(value)
	}
	if value.CheckedInAt == nil {
		return nil, ErrInvalidPresence
	}
	start := CivilDateFromInstant(*value.CheckedInAt)
	observedEnd := earlierDate(asOf.AddDays(1), value.PlannedDeparture)
	observed, err := presenceRange(start, observedEnd, PresenceObserved)
	if err != nil {
		return nil, err
	}
	return appendForecast(observed, observedEnd, value.PlannedDeparture)
}

func earlierDate(left, right CivilDate) CivilDate {
	if right.Before(left) {
		return right
	}
	return left
}

// Days past the observation cut-off are forecast, never observed, so a stay in
// progress never reports presence it has not yet accrued.
func appendForecast(
	observed []PresenceDay,
	observedEnd CivilDate,
	departure CivilDate,
) ([]PresenceDay, error) {
	if !observedEnd.Before(departure) {
		return observed, nil
	}
	forecast, err := presenceRange(observedEnd, departure, PresenceForecast)
	if err != nil {
		return nil, err
	}
	return append(observed, forecast...), nil
}

func checkedInPresence(value Stay) ([]PresenceDay, error) {
	if value.CheckedInAt == nil {
		return nil, ErrInvalidPresence
	}
	return PresenceDaysAt(value, value.PlannedDeparture.AddDays(-1))
}

func checkedOutPresence(value Stay) ([]PresenceDay, error) {
	if value.CheckedInAt == nil || value.CheckedOutAt == nil {
		return nil, ErrInvalidPresence
	}
	start := CivilDateFromInstant(*value.CheckedInAt)
	end := CivilDateFromInstant(*value.CheckedOutAt)
	return presenceRange(start, end, PresenceObserved)
}

func presenceRange(start, end CivilDate, kind PresenceKind) ([]PresenceDay, error) {
	if !start.Before(end) {
		return nil, ErrInvalidPresence
	}
	result := make([]PresenceDay, 0)
	for current := start; current.Before(end); current = current.AddDays(1) {
		result = append(result, PresenceDay{Date: current, Kind: kind})
	}
	return result, nil
}
