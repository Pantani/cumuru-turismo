package config_test

import (
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

type seedProfileCase struct {
	name        string
	values      map[string]string
	wantProfile config.SeedProfile
	wantChange  bool
}

func assertSeedProfile(t *testing.T, test seedProfileCase) {
	t.Helper()
	cfg, err := config.LoadSeed(lookup(test.values))
	if err != nil {
		t.Fatalf("LoadSeed() error = %v", err)
	}
	if cfg.Profile != test.wantProfile {
		t.Fatalf("Profile = %q, want %q", cfg.Profile, test.wantProfile)
	}
	if cfg.Admin.MustChange != test.wantChange {
		t.Fatalf(
			"Admin.MustChange = %t, want %t",
			cfg.Admin.MustChange, test.wantChange,
		)
	}
}

func TestLoadSeedDerivesProfileFromEnvironment(t *testing.T) {
	t.Parallel()

	tests := []seedProfileCase{
		{
			name:        "local defaults to fixtures",
			values:      localSeedValues(),
			wantProfile: config.SeedProfileAdminDemo,
		},
		{
			name: "local may opt out of fixtures",
			values: merge(localSeedValues(), map[string]string{
				"SEED_PROFILE": "admin",
			}),
			wantProfile: config.SeedProfileAdmin,
		},
		{
			name:        "production stays inert by default",
			values:      productionSeedValues(),
			wantProfile: config.SeedProfileNone,
		},
		{
			name: "production seeds the administrator when enabled",
			values: merge(productionSeedValues(), map[string]string{
				"SEED_ENABLED": "true",
			}),
			wantProfile: config.SeedProfileAdmin,
			wantChange:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertSeedProfile(t, test)
		})
	}
}

// An inert profile must not require the bootstrap secret; otherwise every
// deployed environment would have to carry a credential it never uses.
func TestLoadSeedIgnoresAdminWhenDisabled(t *testing.T) {
	t.Parallel()

	values := merge(productionSeedValues(), map[string]string{
		"SEED_ADMIN_EMAIL":    "",
		"SEED_ADMIN_PASSWORD": "",
	})
	cfg, err := config.LoadSeed(lookup(values))
	if err != nil {
		t.Fatalf("LoadSeed() error = %v", err)
	}
	if cfg.Profile != config.SeedProfileNone {
		t.Fatalf("Profile = %q, want %q", cfg.Profile, config.SeedProfileNone)
	}
}

func TestLoadSeedFailsClosed(t *testing.T) {
	t.Parallel()

	local := localSeedValues()
	production := merge(productionSeedValues(), map[string]string{
		"SEED_ENABLED": "true",
	})
	tests := []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{
			name: "fixtures rejected in production",
			values: merge(production, map[string]string{
				"SEED_PROFILE": "admin+demo",
			}),
			wantErr: "SEED_PROFILE",
		},
		{
			name: "unknown profile",
			values: merge(local, map[string]string{
				"SEED_PROFILE": "everything",
			}),
			wantErr: "SEED_PROFILE",
		},
		{
			name: "rotation cannot be relaxed in production",
			values: merge(production, map[string]string{
				"SEED_ADMIN_MUST_CHANGE_PASSWORD": "false",
			}),
			wantErr: "SEED_ADMIN_MUST_CHANGE_PASSWORD",
		},
		{
			name: "missing administrator secret",
			values: merge(local, map[string]string{
				"SEED_ADMIN_PASSWORD": "",
			}),
			wantErr: "SEED_ADMIN_PASSWORD",
		},
		{
			name: "administrator secret below the minimum",
			values: merge(local, map[string]string{
				"SEED_ADMIN_PASSWORD": "curta",
			}),
			wantErr: "SEED_ADMIN_PASSWORD",
		},
		{
			name: "malformed administrator e-mail",
			values: merge(local, map[string]string{
				"SEED_ADMIN_EMAIL": "administracao",
			}),
			wantErr: "SEED_ADMIN_EMAIL",
		},
		{
			name: "provisioning role equals the application role",
			values: merge(local, map[string]string{
				"PROVISIONING_DATABASE_URL": local["DATABASE_URL"],
			}),
			wantErr: "roles must be distinct",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.LoadSeed(lookup(test.values))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("LoadSeed() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

// The e-mail reaches a column constrained to its normalized form, so the loader
// must normalize instead of letting the INSERT fail.
func TestLoadSeedNormalizesAdministratorEmail(t *testing.T) {
	t.Parallel()

	values := merge(localSeedValues(), map[string]string{
		"SEED_ADMIN_EMAIL": "  Administracao@Cumuru.Local  ",
	})
	cfg, err := config.LoadSeed(lookup(values))
	if err != nil {
		t.Fatalf("LoadSeed() error = %v", err)
	}
	if cfg.Admin.Email != "administracao@cumuru.local" {
		t.Fatalf("Admin.Email = %q, want normalized form", cfg.Admin.Email)
	}
}

func localSeedValues() map[string]string {
	return merge(localDemoValues(), seedAdminValues())
}

func productionSeedValues() map[string]string {
	return merge(validProduction(), merge(seedAdminValues(), map[string]string{
		"PROVISIONING_DATABASE_URL": "postgres://migration:placeholder@database.invalid/cumuru?sslmode=verify-full",
	}))
}

func seedAdminValues() map[string]string {
	return map[string]string{
		"SEED_ADMIN_EMAIL":    "administracao@cumuru.local",
		"SEED_ADMIN_PASSWORD": "senha-fictilocal-2026",
	}
}
