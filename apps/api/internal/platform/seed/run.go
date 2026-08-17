// Package seed writes the bootstrap rows an environment needs before anyone can
// sign in: the administrator account and, when a catalog is supplied, the
// establishments it lists. It is a separate binary so no long running process
// holds the provisioning role or the bootstrap secret.
package seed

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/database"
	"github.com/Pantani/cumuru/apps/api/internal/platform/localdemo"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// adminScopes is the set the administrator receives at seed time. It is not a
// separate authorization tier, because the API has none.
//
// It is deliberately not the full set of scopes the application enforces. A
// scope gated by a feature flag is granted by that feature's own activation path,
// never by the seed, because the seed runs against a deployment where the flag
// may be off: deploy/runtime.env.example ships SELF_SERVICE_ENABLED=false. Granting
// such a scope here would make the client render panels for routes the API does
// not register, which is a visible break rather than a missing feature.
//
// stays:approve is the case today. It reaches an account through the self-service
// activation capability (internal/activation.Scopes), and through the local
// demo fixture, where compose.local.yaml pins SELF_SERVICE_ENABLED to true.
var adminScopes = []string{
	"platform:read",
	"accommodations:onboard",
	"accommodations:manage",
	"stays:read:own",
	"stays:write",
	"questionnaires:manage",
	"questionnaires:approve",
	"analytics:read:internal",
}

func Run(ctx context.Context, cfg config.SeedConfig, output io.Writer) error {
	if cfg.Profile == config.SeedProfileNone {
		report(output, "seed disabled by SEED_ENABLED")
		return nil
	}
	if err := runAdminSeed(ctx, cfg, output); err != nil {
		return err
	}
	return runDemoFixtures(ctx, cfg, output)
}

func runAdminSeed(
	ctx context.Context,
	cfg config.SeedConfig,
	output io.Writer,
) error {
	pool, err := database.Open(
		ctx, cfg.ProvisioningDatabaseURL, cfg.Application.DatabaseTimeout,
	)
	if err != nil {
		return err
	}
	defer pool.Close()
	return withRunLock(ctx, cfg, pool, output)
}

func withRunLock(
	ctx context.Context,
	cfg config.SeedConfig,
	pool *pgxpool.Pool,
	output io.Writer,
) error {
	repository := store.NewSeedRepository(pool, cfg.Application.DatabaseTimeout)
	release, err := repository.AcquireRunLock(ctx)
	if err != nil {
		return fmt.Errorf("seed run lock unavailable: %w", err)
	}
	if err := apply(ctx, cfg, repository, output); err != nil {
		_ = release()
		return err
	}
	return release()
}

func apply(
	ctx context.Context,
	cfg config.SeedConfig,
	repository *store.SeedRepository,
	output io.Writer,
) error {
	catalog, err := resolveCatalog(cfg, output)
	if err != nil {
		return err
	}
	if err := applyCatalog(ctx, repository, catalog, output); err != nil {
		return err
	}
	accountID, err := applyAdmin(ctx, cfg, repository, output)
	if err != nil {
		return err
	}
	return applyMemberships(ctx, repository, accountID, catalog, output)
}

// resolveCatalog treats an absent path as an explicit choice: the administrator
// is still seeded, and the establishments are expected to arrive through the
// application's own onboarding.
func resolveCatalog(
	cfg config.SeedConfig,
	output io.Writer,
) (store.SeedOrganization, error) {
	if cfg.CatalogPath == "" {
		report(output, "no accommodation catalog configured")
		return store.SeedOrganization{}, nil
	}
	return loadCatalog(cfg.CatalogPath)
}

func applyCatalog(
	ctx context.Context,
	repository *store.SeedRepository,
	catalog store.SeedOrganization,
	output io.Writer,
) error {
	if len(catalog.Accommodations) == 0 {
		return nil
	}
	if err := repository.EnsureCatalog(ctx, catalog); err != nil {
		return fmt.Errorf("accommodation catalog rejected: %w", err)
	}
	report(
		output,
		fmt.Sprintf("catalog applied: %d accommodations", len(catalog.Accommodations)),
	)
	return nil
}

func applyAdmin(
	ctx context.Context,
	cfg config.SeedConfig,
	repository *store.SeedRepository,
	output io.Writer,
) (uuid.UUID, error) {
	hash, err := access.NewPasswordHasher().Hash(cfg.Admin.Password)
	if err != nil {
		return uuid.Nil, errors.New("administrator secret is unusable")
	}
	accountID, err := repository.EnsureAccount(ctx, store.SeedAccount{
		ID:                 uuid.New(),
		Email:              cfg.Admin.Email,
		DisplayName:        cfg.Admin.DisplayName,
		PasswordHash:       hash,
		Scopes:             adminScopes,
		PasswordMustChange: cfg.Admin.MustChange,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("administrator account rejected: %w", err)
	}
	report(output, "administrator account ready: "+cfg.Admin.Email)
	return accountID, nil
}

func applyMemberships(
	ctx context.Context,
	repository *store.SeedRepository,
	accountID uuid.UUID,
	catalog store.SeedOrganization,
	output io.Writer,
) error {
	if len(catalog.Accommodations) == 0 {
		return nil
	}
	err := repository.EnsureMemberships(
		ctx, access.LocalSessionIssuer, accountID, catalog.Accommodations,
	)
	if err != nil {
		return fmt.Errorf("administrator memberships rejected: %w", err)
	}
	report(output, "administrator bound to the catalog accommodations")
	return nil
}

// runDemoFixtures reuses the prototype seeder so a single flag governs both
// tracks. The configuration loader already refused this profile outside local
// and test.
func runDemoFixtures(
	ctx context.Context,
	cfg config.SeedConfig,
	output io.Writer,
) error {
	if cfg.Profile != config.SeedProfileAdminDemo {
		return nil
	}
	demo, err := config.LoadLocalDemo(nil)
	if err != nil {
		return fmt.Errorf("demo fixtures requested but not configured: %w", err)
	}
	return localdemo.Run(ctx, demo, output)
}

func report(output io.Writer, message string) {
	if output == nil {
		return
	}
	_, _ = fmt.Fprintln(output, "seed: "+message)
}
