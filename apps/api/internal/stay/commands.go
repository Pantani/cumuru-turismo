package stay

import (
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
)

var (
	ErrInvalidCancellation = errors.New("invalid cancellation")
	ErrNoShowBeforeArrival = errors.New("no-show before arrival")
)

type CancelReason string

const (
	CancelReasonGuestRequest         CancelReason = "guest_request"
	CancelReasonAccommodationRequest CancelReason = "accommodation_request"
	CancelReasonDuplicate            CancelReason = "duplicate"
	CancelReasonCorrection           CancelReason = "correction"
)

func (r CancelReason) Valid() bool {
	return r == CancelReasonGuestRequest ||
		r == CancelReasonAccommodationRequest ||
		r == CancelReasonDuplicate ||
		r == CancelReasonCorrection
}

type CancelCommand struct {
	Role       accommodation.Role
	Correction bool
	Reason     CancelReason
}

func (c CancelCommand) Validate(current Status) error {
	if !c.Role.Valid() || !c.Reason.Valid() {
		return ErrInvalidCancellation
	}
	if current == StatusCheckedIn {
		return c.validateCheckedIn()
	}
	if c.Correction {
		return ErrInvalidCancellation
	}
	_, err := current.Transition(EventCancel)
	return err
}

func (c CancelCommand) validateCheckedIn() error {
	if c.Role != accommodation.RoleManager ||
		!c.Correction ||
		c.Reason != CancelReasonCorrection {
		return ErrInvalidCancellation
	}
	return nil
}

func ValidateNoShowTime(arrival CivilDate, occurredAt time.Time) error {
	if occurredAt.UTC().Before(arrival.Start().UTC()) {
		return ErrNoShowBeforeArrival
	}
	return nil
}
