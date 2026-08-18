//go:build integration

// The seeder runs against a database that already holds real data, so its
// safety rests entirely on the ON CONFLICT clauses: the account upsert must not
// write password_hash, and the accommodation upsert is scoped to the declared
// organization. Neither claim is observable without executing the statements,
// and nothing else in the suite does.
//
// SQL inline via pgxpool é convenção deliberada nestes testes de integração
// (não migrar para sqlc); ver AGENTS.md, seção "Padrões de backend".
package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const seedIssuer = "https://sessao.local.invalid"

// The seeder holds the provisioning role: writing auth.accounts is exactly the
// privilege the runtime role must not have.
func newSeedRepository(t *testing.T, adminPool *pgxpool.Pool) *store.SeedRepository {
	t.Helper()
	return store.NewSeedRepository(adminPool, 10*time.Second)
}

func seedCatalog(t *testing.T, name string) store.SeedOrganization {
	t.Helper()
	organizationID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate organization id: %v", err)
	}
	accommodationID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate accommodation id: %v", err)
	}
	return store.SeedOrganization{
		ID:   organizationID,
		Name: name,
		Accommodations: []store.SeedAccommodation{
			// The category set is closed by accommodations_category_valid.
			{ID: accommodationID, Name: name + " Sede", Category: "formal_lodging"},
		},
	}
}

func seedAccountFixture(t *testing.T, email string) store.SeedAccount {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate account id: %v", err)
	}
	return store.SeedAccount{
		ID:                 id,
		Email:              email,
		DisplayName:        "Administração Semeada",
		PasswordHash:       seedHash("inicial"),
		Scopes:             []string{"stays:write"},
		PasswordMustChange: true,
	}
}

// The hash is only ever compared for equality here, so it needs the argon2id
// prefix the accounts_password_hash_algorithm constraint demands and nothing
// more; no plaintext is involved at this layer by design.
func seedHash(variant string) string {
	return "$argon2id$v=19$m=65536,t=3,p=2$c2VtZW50ZQ$" + variant
}

func cleanupSeedOrganization(
	t *testing.T,
	adminPool *pgxpool.Pool,
	organization store.SeedOrganization,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	statements := []string{
		`DELETE FROM core.memberships WHERE accommodation_id IN (
		   SELECT id FROM core.accommodations WHERE organization_id = $1)`,
		`DELETE FROM core.accommodations WHERE organization_id = $1`,
		`DELETE FROM core.organizations WHERE id = $1`,
	}
	for _, statement := range statements {
		if _, err := adminPool.Exec(ctx, statement, organization.ID); err != nil {
			t.Errorf("clean seeded organization: %v", err)
		}
	}
}

func cleanupSeedAccount(t *testing.T, adminPool *pgxpool.Pool, email string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := adminPool.Exec(
		ctx, `DELETE FROM auth.accounts WHERE email = $1`, email,
	); err != nil {
		t.Errorf("clean seeded account: %v", err)
	}
}

func readAccountHash(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	email string,
) string {
	t.Helper()
	var hash string
	err := adminPool.QueryRow(
		ctx, `SELECT password_hash FROM auth.accounts WHERE email = $1`, email,
	).Scan(&hash)
	if err != nil {
		t.Fatalf("read seeded password hash: %v", err)
	}
	return hash
}

func readAccommodationOwner(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	id uuid.UUID,
) uuid.UUID {
	t.Helper()
	var owner uuid.UUID
	err := adminPool.QueryRow(
		ctx, `SELECT organization_id FROM core.accommodations WHERE id = $1`, id,
	).Scan(&owner)
	if err != nil {
		t.Fatalf("read accommodation owner: %v", err)
	}
	return owner
}

// A deploy re-runs the seeder, and the administrator has by then rotated the
// bootstrap secret. Restoring the seeded hash would silently hand the
// provisional credential back.
func TestEnsureAccountKeepsARotatedPassword(t *testing.T) {
	ctx := context.Background()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	repository := newSeedRepository(t, adminPool)

	email := "semeadura@integracao.invalid"
	account := seedAccountFixture(t, email)
	t.Cleanup(func() { cleanupSeedAccount(t, adminPool, email) })

	if _, err := repository.EnsureAccount(ctx, account); err != nil {
		t.Fatalf("first EnsureAccount error = %v", err)
	}
	rotated := seedHash("rotacionada")
	if _, err := adminPool.Exec(
		ctx,
		`UPDATE auth.accounts
		 SET password_hash = $1, password_must_change = false
		 WHERE email = $2`,
		rotated, email,
	); err != nil {
		t.Fatalf("simulate rotation: %v", err)
	}

	account.DisplayName = "Administração Renomeada"
	if _, err := repository.EnsureAccount(ctx, account); err != nil {
		t.Fatalf("second EnsureAccount error = %v", err)
	}
	if stored := readAccountHash(t, ctx, adminPool, email); stored != rotated {
		t.Fatal("re-seeding replaced the rotated password hash")
	}
}

// A disabled account under the seeded address is a decision an operator made,
// not a row to reuse.
func TestEnsureAccountRefusesADisabledAccount(t *testing.T) {
	ctx := context.Background()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	repository := newSeedRepository(t, adminPool)

	email := "desativada@integracao.invalid"
	account := seedAccountFixture(t, email)
	t.Cleanup(func() { cleanupSeedAccount(t, adminPool, email) })

	if _, err := repository.EnsureAccount(ctx, account); err != nil {
		t.Fatalf("first EnsureAccount error = %v", err)
	}
	if _, err := adminPool.Exec(
		ctx, `UPDATE auth.accounts SET status = 'disabled' WHERE email = $1`, email,
	); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	if _, err := repository.EnsureAccount(ctx, account); !errors.Is(
		err, store.ErrSeedConflict,
	) {
		t.Fatalf("EnsureAccount error = %v, want ErrSeedConflict", err)
	}
}

// Re-running the seeder is the normal case, not the exception: a second run
// must converge on the same rows instead of duplicating the catalog.
func TestEnsureCatalogAndMembershipsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	repository := newSeedRepository(t, adminPool)

	organization := seedCatalog(t, "Pousada Integração")
	t.Cleanup(func() { cleanupSeedOrganization(t, adminPool, organization) })
	accountID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate account id: %v", err)
	}

	for round := 1; round <= 2; round++ {
		if err := repository.EnsureCatalog(ctx, organization); err != nil {
			t.Fatalf("EnsureCatalog round %d error = %v", round, err)
		}
		if err := repository.EnsureMemberships(
			ctx, seedIssuer, accountID, organization.Accommodations,
		); err != nil {
			t.Fatalf("EnsureMemberships round %d error = %v", round, err)
		}
	}

	if total := countSeedAccommodations(t, ctx, adminPool, organization.ID); total != 1 {
		t.Fatalf("accommodations after two runs = %d, want 1", total)
	}
	if total := countSeedMemberships(
		t, ctx, adminPool, organization.Accommodations[0].ID,
	); total != 1 {
		t.Fatalf("memberships after two runs = %d, want 1", total)
	}
}

// The upsert is scoped to the declared organization so a catalog cannot move an
// establishment between tenants. The seeder must not report success for an
// entry it did not apply, otherwise the operator reads a green run and believes
// the catalog landed.
func TestEnsureCatalogRefusesToTakeOverAnotherTenantsAccommodation(t *testing.T) {
	ctx := context.Background()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	repository := newSeedRepository(t, adminPool)

	incumbent := seedCatalog(t, "Pousada Titular")
	t.Cleanup(func() { cleanupSeedOrganization(t, adminPool, incumbent) })
	if err := repository.EnsureCatalog(ctx, incumbent); err != nil {
		t.Fatalf("seed incumbent catalog error = %v", err)
	}

	// The same accommodation identifier, declared under a different owner.
	intruder := seedCatalog(t, "Pousada Invasora")
	intruder.Accommodations[0].ID = incumbent.Accommodations[0].ID
	t.Cleanup(func() { cleanupSeedOrganization(t, adminPool, intruder) })

	err := repository.EnsureCatalog(ctx, intruder)
	owner := readAccommodationOwner(
		t, ctx, adminPool, incumbent.Accommodations[0].ID,
	)
	if owner != incumbent.ID {
		t.Fatalf("accommodation owner = %s, want the incumbent %s", owner, incumbent.ID)
	}
	if !errors.Is(err, store.ErrSeedConflict) {
		t.Fatalf("EnsureCatalog error = %v, want ErrSeedConflict", err)
	}
}

func countSeedAccommodations(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	organizationID uuid.UUID,
) int {
	t.Helper()
	var total int
	err := adminPool.QueryRow(
		ctx,
		`SELECT count(*) FROM core.accommodations WHERE organization_id = $1`,
		organizationID,
	).Scan(&total)
	if err != nil {
		t.Fatalf("count seeded accommodations: %v", err)
	}
	return total
}

func countSeedMemberships(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	accommodationID uuid.UUID,
) int {
	t.Helper()
	var total int
	err := adminPool.QueryRow(
		ctx,
		`SELECT count(*) FROM core.memberships WHERE accommodation_id = $1`,
		accommodationID,
	).Scan(&total)
	if err != nil {
		t.Fatalf("count seeded memberships: %v", err)
	}
	return total
}
