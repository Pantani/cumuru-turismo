package store

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAccommodationUpdateParamsExcludeCadastur(t *testing.T) {
	t.Parallel()

	params := updateAccommodationParams(accommodation.UpdateCommand{
		Actor:           access.NewPrincipal("https://issuer.invalid", "manager", nil),
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ExpectedVersion: 2,
		Patch: accommodation.UpdatePatch{
			SetCategory: true,
			Category:    accommodation.CategoryFamilyHosting,
		},
	}, time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))
	paramsType := reflect.TypeOf(params)
	if _, exists := paramsType.FieldByName("SetCadasturID"); exists {
		t.Fatal("update params expose SetCadasturID")
	}
	if _, exists := paramsType.FieldByName("CadasturID"); exists {
		t.Fatal("update params expose CadasturID")
	}
	if params.Category != string(accommodation.CategoryFamilyHosting) {
		t.Fatalf("category = %q", params.Category)
	}
}

func TestLocalDemoReadRequiresCanonicalCategoryAndExplicitCadastur(t *testing.T) {
	t.Parallel()

	fakeCadastur := "CADASTUR-FICTICIO-NAO-VALIDO"
	organizationID := uuid.MustParse("019f0000-0000-7000-8000-000000000011")
	fixture := LocalDemoAccommodation{
		ID:   uuid.MustParse("019fae11-0000-7000-8000-000000000001"),
		Name: "Pousada Farol Fictícia", Category: "formal_lodging",
		CadasturID: &fakeCadastur, Capacity: 24, PublicAreaCode: "cumuruxatiba",
	}
	row := generated.GetLocalDemoAccommodationRow{
		OrganizationID: pgtype.UUID{Bytes: organizationID, Valid: true},
		Name:           fixture.Name, Category: "formal_lodging", Status: "active",
		CadasturID: &fakeCadastur, Capacity: &fixture.Capacity,
		PublicAreaCode: &fixture.PublicAreaCode,
	}
	if !localAccommodationMatches(row, organizationID, fixture) {
		t.Fatal("canonical local-demo row did not match fixture")
	}
	row.Category = "pousada"
	if localAccommodationMatches(row, organizationID, fixture) {
		t.Fatal("fixture verification accepted legacy category alias")
	}
	row.Category = "formal_lodging"
	row.CadasturID = nil
	if localAccommodationMatches(row, organizationID, fixture) {
		t.Fatal("fixture verification accepted missing fake Cadastur")
	}
}

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
