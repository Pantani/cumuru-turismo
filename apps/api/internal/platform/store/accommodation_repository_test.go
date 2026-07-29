package store

import (
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
)

func TestAccommodationMutationPolicyDistinguishesStateAndRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    accommodation.Status
		role      accommodation.Role
		operation accommodation.Operation
		want      error
	}{
		{
			name: "active manager update allowed", status: accommodation.StatusActive,
			role: accommodation.RoleManager, operation: accommodation.OperationUpdateAccommodation,
		},
		{
			name: "suspended update is conflict", status: accommodation.StatusSuspended,
			role: accommodation.RoleManager, operation: accommodation.OperationUpdateAccommodation,
			want: accommodation.ErrConflict,
		},
		{
			name: "closed membership mutation is conflict", status: accommodation.StatusClosed,
			role: accommodation.RoleManager, operation: accommodation.OperationManageMemberships,
			want: accommodation.ErrConflict,
		},
		{
			name: "operator management is not found", status: accommodation.StatusActive,
			role: accommodation.RoleOperator, operation: accommodation.OperationManageMemberships,
			want: accommodation.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := accommodationMutationPolicy(tt.status, tt.role, tt.operation)
			if !errors.Is(err, tt.want) {
				t.Fatalf("accommodationMutationPolicy() error = %v, want %v", err, tt.want)
			}
		})
	}
}
