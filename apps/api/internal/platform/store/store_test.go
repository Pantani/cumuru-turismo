package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeQueries struct {
	generated.Querier
	readiness int32
	err       error
	params    generated.ListActiveTenantMembershipsParams
	rows      []generated.ListActiveTenantMembershipsRow
}

func (f *fakeQueries) CheckReadiness(context.Context) (int32, error) {
	return f.readiness, f.err
}

func (f *fakeQueries) ListActiveTenantMemberships(_ context.Context, params generated.ListActiveTenantMembershipsParams) ([]generated.ListActiveTenantMembershipsRow, error) {
	f.params = params
	return f.rows, f.err
}

func TestStoreUsesGeneratedReadinessQuery(t *testing.T) {
	t.Parallel()

	queries := &fakeQueries{readiness: 1}
	subject := store.New(queries, time.Second)
	if err := subject.CheckReadiness(context.Background()); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}

	queries.readiness = 0
	if err := subject.CheckReadiness(context.Background()); !errors.Is(err, store.ErrUnavailable) {
		t.Fatalf("CheckReadiness() error = %v, want ErrUnavailable", err)
	}
}

func TestResolveTenantsUsesVerifiedIssuerAndSubject(t *testing.T) {
	t.Parallel()

	organizationID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	accommodationID := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	membershipID := uuid.MustParse("019f0000-0000-7000-8000-000000000003")
	queries := &fakeQueries{
		rows: []generated.ListActiveTenantMembershipsRow{{
			MembershipID:    pgUUID(membershipID),
			Role:            "operator",
			AccommodationID: pgUUID(accommodationID),
			OrganizationID:  pgUUID(organizationID),
		}},
	}
	subject := store.New(queries, time.Second)
	principal := access.NewPrincipal("https://issuer.invalid", "subject-a", []string{"platform:read"})

	tenants, err := subject.ResolveTenants(context.Background(), principal)
	if err != nil {
		t.Fatalf("ResolveTenants() error = %v", err)
	}
	if queries.params.OidcIssuer != "https://issuer.invalid" || queries.params.OidcSubject != "subject-a" {
		t.Fatalf("query params = %#v", queries.params)
	}
	if len(tenants) != 1 || tenants[0].OrganizationID != organizationID.String() || tenants[0].AccommodationID != accommodationID.String() {
		t.Fatalf("tenants = %#v", tenants)
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}
