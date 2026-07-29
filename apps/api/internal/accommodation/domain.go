package accommodation

import "errors"

var (
	ErrInvalidMembership = errors.New("invalid membership")
	ErrLastActiveManager = errors.New("last active manager")
)

type Status string

const (
	StatusPendingReview Status = "pending_review"
	StatusActive        Status = "active"
	StatusSuspended     Status = "suspended"
	StatusClosed        Status = "closed"
)

type Operation string

const (
	OperationRead                Operation = "read"
	OperationUpdateAccommodation Operation = "update_accommodation"
	OperationManageMemberships   Operation = "manage_memberships"
	OperationCreateStay          Operation = "create_stay"
	OperationUpdateStay          Operation = "update_stay"
	OperationIssueInvite         Operation = "issue_invite"
	OperationSubmitGroup         Operation = "submit_group"
	OperationCheckIn             Operation = "check_in"
	OperationCheckOut            Operation = "check_out"
	OperationCancel              Operation = "cancel"
	OperationNoShow              Operation = "no_show"
)

var allowedOperations = map[Status]map[Operation]bool{
	StatusPendingReview: {
		OperationRead: true,
	},
	StatusActive: {
		OperationRead:                true,
		OperationUpdateAccommodation: true,
		OperationManageMemberships:   true,
		OperationCreateStay:          true,
		OperationUpdateStay:          true,
		OperationIssueInvite:         true,
		OperationSubmitGroup:         true,
		OperationCheckIn:             true,
		OperationCheckOut:            true,
		OperationCancel:              true,
		OperationNoShow:              true,
	},
	StatusSuspended: {
		OperationRead:     true,
		OperationCheckOut: true,
		OperationCancel:   true,
		OperationNoShow:   true,
	},
	StatusClosed: {
		OperationRead: true,
	},
}

func (s Status) Allows(operation Operation) bool {
	return allowedOperations[s][operation]
}

type Role string

const (
	RoleOperator Role = "operator"
	RoleManager  Role = "manager"
)

func (r Role) Valid() bool {
	return r == RoleOperator || r == RoleManager
}

func (r Role) CanManageMemberships() bool {
	return r == RoleManager
}

type MembershipChange struct {
	CurrentRole   Role
	CurrentActive bool
	NextRole      Role
	NextActive    bool
}

func (c MembershipChange) Validate(activeManagerCount int) error {
	if !c.CurrentRole.Valid() || !c.NextRole.Valid() || activeManagerCount < 0 {
		return ErrInvalidMembership
	}
	if !c.removesActiveManager() {
		return nil
	}
	if activeManagerCount <= 1 {
		return ErrLastActiveManager
	}
	return nil
}

func (c MembershipChange) removesActiveManager() bool {
	return c.CurrentActive &&
		c.CurrentRole == RoleManager &&
		(!c.NextActive || c.NextRole != RoleManager)
}
