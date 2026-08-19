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

type Category string

const (
	CategoryFormalLodging  Category = "formal_lodging"
	CategorySeasonalRental Category = "seasonal_rental"
	CategoryFamilyHosting  Category = "family_hosting"
	CategoryCamping        Category = "camping"
	CategoryRegularizing   Category = "regularizing"
	CategoryOther          Category = "other"
	CategoryUnclassified   Category = "unclassified"
)

func (c Category) ValidInput() bool {
	switch c {
	case CategoryFormalLodging,
		CategorySeasonalRental,
		CategoryFamilyHosting,
		CategoryCamping,
		CategoryRegularizing,
		CategoryOther:
		return true
	default:
		return false
	}
}

// Valid aceita também unclassified, que ValidInput recusa: o vocabulário de
// entrada é menor que o de armazenamento, porque unclassified é estado de linha
// herdada e nunca escolha de quem cadastra.
func (c Category) Valid() bool {
	return c.ValidInput() || c == CategoryUnclassified
}

type Operation string

const (
	OperationRead                Operation = "read"
	OperationUpdateAccommodation Operation = "update_accommodation"
	OperationManageMemberships   Operation = "manage_memberships"
	OperationCreateStay          Operation = "create_stay"
	OperationUpdateStay          Operation = "update_stay"
	OperationIssueInvite         Operation = "issue_invite"
	OperationIssueActivation     Operation = "issue_activation"
	OperationSubmitGroup         Operation = "submit_group"
	// OperationApproveStay is deliberately not update_stay. Reusing the edit
	// permission would hand approval to every operator who may correct a date,
	// and approval is the one control standing between an anonymous submission
	// and a statistic (ADR-040).
	OperationApproveStay Operation = "approve_stay"
	OperationCheckIn     Operation = "check_in"
	OperationCheckOut    Operation = "check_out"
	OperationCancel      Operation = "cancel"
	OperationNoShow      Operation = "no_show"
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
		OperationIssueActivation:     true,
		OperationSubmitGroup:         true,
		OperationApproveStay:         true,
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

// An accommodation must always keep at least one active manager, otherwise
// nobody could ever restore access to it.
func (c MembershipChange) Validate(activeManagerCount int) error {
	if !c.validRoles() || activeManagerCount < 0 {
		return ErrInvalidMembership
	}
	if c.removesActiveManager() && activeManagerCount <= 1 {
		return ErrLastActiveManager
	}
	return nil
}

func (c MembershipChange) validRoles() bool {
	return c.CurrentRole.Valid() && c.NextRole.Valid()
}

func (c MembershipChange) removesActiveManager() bool {
	return c.CurrentActive &&
		c.CurrentRole == RoleManager &&
		(!c.NextActive || c.NextRole != RoleManager)
}
