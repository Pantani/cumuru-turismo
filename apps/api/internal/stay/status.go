package stay

import "errors"

var ErrInvalidTransition = errors.New("invalid state transition")

type Status string

const (
	StatusDraft         Status = "draft"
	StatusInvited       Status = "invited"
	StatusPreRegistered Status = "pre_registered"
	StatusCheckedIn     Status = "checked_in"
	StatusCheckedOut    Status = "checked_out"
	StatusCancelled     Status = "cancelled"
	StatusNoShow        Status = "no_show"
)

type Event string

const (
	EventInvite      Event = "invite"
	EventSubmitGroup Event = "submit_group"
	EventCheckIn     Event = "check_in"
	EventCheckOut    Event = "check_out"
	EventCancel      Event = "cancel"
	EventNoShow      Event = "no_show"
)

type transition struct {
	from  Status
	event Event
}

var transitions = map[transition]Status{
	{StatusDraft, EventInvite}:          StatusInvited,
	{StatusInvited, EventInvite}:        StatusInvited,
	{StatusDraft, EventSubmitGroup}:     StatusPreRegistered,
	{StatusInvited, EventSubmitGroup}:   StatusPreRegistered,
	{StatusPreRegistered, EventCheckIn}: StatusCheckedIn,
	{StatusCheckedIn, EventCheckOut}:    StatusCheckedOut,
	{StatusDraft, EventCancel}:          StatusCancelled,
	{StatusInvited, EventCancel}:        StatusCancelled,
	{StatusPreRegistered, EventCancel}:  StatusCancelled,
	{StatusInvited, EventNoShow}:        StatusNoShow,
	{StatusPreRegistered, EventNoShow}:  StatusNoShow,
}

func (s Status) Transition(event Event) (Status, error) {
	next, ok := transitions[transition{s, event}]
	if !ok {
		return "", ErrInvalidTransition
	}
	return next, nil
}
