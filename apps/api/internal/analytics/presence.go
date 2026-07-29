package analytics

import (
	"errors"
	"sort"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

var ErrInvalidPresenceSource = errors.New("invalid presence source")

type ReconciliationKind string

const (
	ReconciliationIncremental ReconciliationKind = "incremental"
	ReconciliationFull        ReconciliationKind = "full"
)

type PresenceSource struct {
	StayID           uuid.UUID
	Status           stay.Status
	PlannedArrival   stay.CivilDate
	PlannedDeparture stay.CivilDate
	CheckedInAt      *time.Time
	CheckedOutAt     *time.Time
	Version          int64
	VisitorIDs       []uuid.UUID
}

type PresenceFact struct {
	StayID            uuid.UUID
	VisitorID         uuid.UUID
	PresenceOn        stay.CivilDate
	Kind              stay.PresenceKind
	Weight            float64
	SourceStayVersion int64
	AsOfOn            stay.CivilDate
}

func MaterializePresence(
	source PresenceSource,
	asOf stay.CivilDate,
	preRegisteredWeight float64,
) ([]PresenceFact, error) {
	if !validPresenceSource(source, preRegisteredWeight) {
		return nil, ErrInvalidPresenceSource
	}
	days, err := stay.PresenceDaysAt(stay.Stay{
		Status: source.Status, PlannedArrival: source.PlannedArrival,
		PlannedDeparture: source.PlannedDeparture,
		CheckedInAt:      source.CheckedInAt, CheckedOutAt: source.CheckedOutAt,
	}, asOf)
	if err != nil {
		return nil, ErrInvalidPresenceSource
	}
	visitors := append([]uuid.UUID(nil), source.VisitorIDs...)
	sort.Slice(visitors, func(first, second int) bool {
		return visitors[first].String() < visitors[second].String()
	})
	result := make([]PresenceFact, 0, len(visitors)*len(days))
	for _, visitorID := range visitors {
		for _, day := range days {
			result = append(result, presenceFact(source, visitorID, day, asOf, preRegisteredWeight))
		}
	}
	return result, nil
}

func validPresenceSource(source PresenceSource, preRegisteredWeight float64) bool {
	if source.StayID == uuid.Nil || source.Version < 1 {
		return false
	}
	if preRegisteredWeight <= 0 || preRegisteredWeight > 1 {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(source.VisitorIDs))
	for _, visitorID := range source.VisitorIDs {
		if visitorID == uuid.Nil {
			return false
		}
		if _, exists := seen[visitorID]; exists {
			return false
		}
		seen[visitorID] = struct{}{}
	}
	return true
}

func presenceFact(
	source PresenceSource,
	visitorID uuid.UUID,
	day stay.PresenceDay,
	asOf stay.CivilDate,
	preRegisteredWeight float64,
) PresenceFact {
	weight := 1.0
	if source.Status == stay.StatusPreRegistered {
		weight = preRegisteredWeight
	}
	return PresenceFact{
		StayID: source.StayID, VisitorID: visitorID, PresenceOn: day.Date,
		Kind: day.Kind, Weight: weight, SourceStayVersion: source.Version,
		AsOfOn: asOf,
	}
}
