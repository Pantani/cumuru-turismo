package accommodation_test

import (
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
)

func TestStatusPolicyAllowsOnlyDocumentedOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status    accommodation.Status
		operation accommodation.Operation
		allowed   bool
	}{
		{accommodation.StatusActive, accommodation.OperationCreateStay, true},
		{accommodation.StatusActive, accommodation.OperationUpdateAccommodation, true},
		{accommodation.StatusActive, accommodation.OperationManageMemberships, true},
		{accommodation.StatusActive, accommodation.OperationUpdateStay, true},
		{accommodation.StatusActive, accommodation.OperationCheckIn, true},
		{accommodation.StatusSuspended, accommodation.OperationCreateStay, false},
		{accommodation.StatusSuspended, accommodation.OperationUpdateAccommodation, false},
		{accommodation.StatusSuspended, accommodation.OperationManageMemberships, false},
		{accommodation.StatusSuspended, accommodation.OperationUpdateStay, false},
		{accommodation.StatusSuspended, accommodation.OperationIssueInvite, false},
		{accommodation.StatusSuspended, accommodation.OperationSubmitGroup, false},
		{accommodation.StatusSuspended, accommodation.OperationCheckIn, false},
		{accommodation.StatusSuspended, accommodation.OperationRead, true},
		{accommodation.StatusSuspended, accommodation.OperationCheckOut, true},
		{accommodation.StatusSuspended, accommodation.OperationCancel, true},
		{accommodation.StatusSuspended, accommodation.OperationNoShow, true},
		{accommodation.StatusClosed, accommodation.OperationRead, true},
		{accommodation.StatusClosed, accommodation.OperationUpdateAccommodation, false},
		{accommodation.StatusClosed, accommodation.OperationManageMemberships, false},
		{accommodation.StatusClosed, accommodation.OperationUpdateStay, false},
		{accommodation.StatusClosed, accommodation.OperationCheckOut, false},
		{accommodation.StatusPendingReview, accommodation.OperationRead, true},
		{accommodation.StatusPendingReview, accommodation.OperationCreateStay, false},
	}
	for _, tt := range tests {
		if got := tt.status.Allows(tt.operation); got != tt.allowed {
			t.Errorf("%s.Allows(%s) = %t, want %t", tt.status, tt.operation, got, tt.allowed)
		}
	}
}

func TestMembershipChangeProtectsLastActiveManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		change        accommodation.MembershipChange
		activeManager int
		wantErr       error
	}{
		{
			name: "manager can remain active manager",
			change: accommodation.MembershipChange{
				CurrentRole:   accommodation.RoleManager,
				CurrentActive: true,
				NextRole:      accommodation.RoleManager,
				NextActive:    true,
			},
			activeManager: 1,
		},
		{
			name: "last manager cannot become operator",
			change: accommodation.MembershipChange{
				CurrentRole:   accommodation.RoleManager,
				CurrentActive: true,
				NextRole:      accommodation.RoleOperator,
				NextActive:    true,
			},
			activeManager: 1,
			wantErr:       accommodation.ErrLastActiveManager,
		},
		{
			name: "last manager cannot become inactive",
			change: accommodation.MembershipChange{
				CurrentRole:   accommodation.RoleManager,
				CurrentActive: true,
				NextRole:      accommodation.RoleManager,
				NextActive:    false,
			},
			activeManager: 1,
			wantErr:       accommodation.ErrLastActiveManager,
		},
		{
			name: "one of two managers can become operator",
			change: accommodation.MembershipChange{
				CurrentRole:   accommodation.RoleManager,
				CurrentActive: true,
				NextRole:      accommodation.RoleOperator,
				NextActive:    true,
			},
			activeManager: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.change.Validate(tt.activeManager)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRolesHaveClosedPermissions(t *testing.T) {
	t.Parallel()

	if !accommodation.RoleManager.CanManageMemberships() {
		t.Fatal("manager must manage memberships")
	}
	if accommodation.RoleOperator.CanManageMemberships() {
		t.Fatal("operator must not manage memberships")
	}
	if accommodation.Role("owner").Valid() {
		t.Fatal("unknown role must be invalid")
	}
}
