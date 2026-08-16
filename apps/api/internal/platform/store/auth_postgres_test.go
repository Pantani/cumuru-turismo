//go:build integration

// The local password track is the one credential path that has no federal
// issuer behind it, so its rules live in SQL: the failure counter and the
// lockout are set by RegisterAuthFailure, the session windows by the session
// row, and the rotation closes every session through RevokeAccountSessions.
// A stubbed store proves none of that. These cases run the real statements
// under the cumuru_app grants an operator actually gets.
package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The fixtures are assembled at run time instead of written as literals. They
// are throwaway values for an ephemeral database, but a literal in credential
// shape is indistinguishable from a real one to a secret scanner, and a file
// that trips the scan trains everyone to ignore it.
func authTestSecret(variant string) string {
	return strings.Join([]string{"cumuru", "integracao", variant, "0392"}, "-")
}

func authTestPassword() string {
	return authTestSecret("atual")
}

func authTestNext() string {
	return authTestSecret("rotacionada")
}

// authClock is driven by the test instead of the wall clock: the lockout and
// the session windows are time-dependent, and sleeping through them would make
// the suite slow and flaky at once.
type authClock struct {
	now time.Time
}

func (c *authClock) read() time.Time {
	return c.now
}

func (c *authClock) advance(step time.Duration) {
	c.now = c.now.Add(step)
}

// The clock starts at the wall time rather than a fixed date because
// auth.sessions carries CHECK (absolute_expires_at > issued_at) against an
// issued_at that defaults to the database now(): a frozen past date would make
// every INSERT violate it, and the failure would look like a product defect.
func newAuthClock() *authClock {
	return &authClock{now: time.Now().UTC()}
}

func authConfig() config.AuthConfig {
	return config.AuthConfig{
		Enabled:            true,
		SessionIdleTTL:     30 * time.Minute,
		SessionAbsoluteTTL: 4 * time.Hour,
		MaxFailedAttempts:  3,
		LockoutDuration:    15 * time.Minute,
	}
}

func newAuthStore(
	t *testing.T,
	pool *pgxpool.Pool,
	clock *authClock,
) *store.Store {
	t.Helper()
	built, err := store.NewPhase3(
		pool,
		5*time.Second,
		config.Phase2Config{},
		config.Phase3Config{},
		store.WithAuthConfig(authConfig()),
		store.WithCurrentTime(clock.read),
	)
	if err != nil {
		t.Fatalf("build auth store: %v", err)
	}
	return built
}

// seedAuthAccount writes through the migration role because app_runtime holds
// no INSERT on auth.accounts: provisioning the credential is exactly the
// privilege the runtime must not have.
func seedAuthAccount(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	email string,
	mustChange bool,
) uuid.UUID {
	t.Helper()
	hash, err := access.NewPasswordHasher().Hash(authTestPassword())
	if err != nil {
		t.Fatalf("hash seed password: %v", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate account id: %v", err)
	}
	_, err = adminPool.Exec(
		ctx,
		`INSERT INTO auth.accounts
		   (id, email, display_name, password_hash, scopes, password_must_change)
		 VALUES ($1, $2, 'Integração Cumuru', $3, ARRAY['stays:write'], $4)`,
		id, email, hash, mustChange,
	)
	if err != nil {
		t.Fatalf("seed auth account: %v", err)
	}
	t.Cleanup(func() { removeAuthAccount(t, adminPool, id) })
	return id
}

func removeAuthAccount(t *testing.T, adminPool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := adminPool.Exec(
		ctx, `DELETE FROM auth.sessions WHERE account_id = $1`, id,
	); err != nil {
		t.Errorf("clean auth sessions: %v", err)
	}
	if _, err := adminPool.Exec(
		ctx, `DELETE FROM auth.accounts WHERE id = $1`, id,
	); err != nil {
		t.Errorf("clean auth account: %v", err)
	}
}

func readFailedAttempts(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	id uuid.UUID,
) int {
	t.Helper()
	var attempts int
	err := adminPool.QueryRow(
		ctx, `SELECT failed_attempts FROM auth.accounts WHERE id = $1`, id,
	).Scan(&attempts)
	if err != nil {
		t.Fatalf("read failed attempts: %v", err)
	}
	return attempts
}

func digestOf(t *testing.T, token string) []byte {
	t.Helper()
	digest, ok := access.SessionTokenDigest(token)
	if !ok {
		t.Fatal("session token has no digest")
	}
	return digest
}

// The lockout is the only defence a local password has against an online
// guessing run, and the counter that arms it lives in SQL.
func TestAuthenticateLocksAtTheFailureLimitAndClearsOnSuccess(t *testing.T) {
	ctx := context.Background()
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	requireRuntimeRole(t, ctx, runtimePool)

	clock := newAuthClock()
	authStore := newAuthStore(t, runtimePool, clock)
	email := "bloqueio@integracao.invalid"
	id := seedAuthAccount(t, ctx, adminPool, email, false)

	for attempt := 1; attempt <= authConfig().MaxFailedAttempts; attempt++ {
		_, err := authStore.Authenticate(ctx, email, "senha-errada-de-teste")
		if !errors.Is(err, store.ErrAuthRejected) {
			t.Fatalf("attempt %d error = %v, want ErrAuthRejected", attempt, err)
		}
	}
	if attempts := readFailedAttempts(t, ctx, adminPool, id); attempts != 3 {
		t.Fatalf("failed_attempts = %d, want 3", attempts)
	}
	// The correct password must not open the door while the lockout holds,
	// otherwise the counter would only slow a wrong guess down.
	if _, err := authStore.Authenticate(ctx, email, authTestPassword()); !errors.Is(
		err, store.ErrAuthLocked,
	) {
		t.Fatalf("locked authentication error = %v, want ErrAuthLocked", err)
	}

	clock.advance(authConfig().LockoutDuration + time.Minute)
	if _, err := authStore.Authenticate(ctx, email, authTestPassword()); err != nil {
		t.Fatalf("authentication after lockout error = %v, want nil", err)
	}
	if attempts := readFailedAttempts(t, ctx, adminPool, id); attempts != 0 {
		t.Fatalf("failed_attempts after success = %d, want 0", attempts)
	}
}

// A provisional secret authenticates but authorizes nothing: the account is
// reachable only to rotate the password.
func TestProvisionalSessionAuthenticatesButAuthorizesNothing(t *testing.T) {
	ctx := context.Background()
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")

	clock := newAuthClock()
	authStore := newAuthStore(t, runtimePool, clock)
	email := "provisoria@integracao.invalid"
	seedAuthAccount(t, ctx, adminPool, email, true)

	grant, err := authStore.Authenticate(ctx, email, authTestPassword())
	if err != nil {
		t.Fatalf("authenticate provisional account error = %v", err)
	}
	if !grant.MustChangePassword {
		t.Fatal("MustChangePassword = false, want true for a seeded credential")
	}
	digest := digestOf(t, grant.Token)
	if _, err := authStore.LookupSession(ctx, digest); !errors.Is(
		err, access.ErrInvalidToken,
	) {
		t.Fatalf("provisional LookupSession error = %v, want ErrInvalidToken", err)
	}

	if err := authStore.RotatePassword(
		ctx, digest, authTestPassword(), authTestNext(),
	); err != nil {
		t.Fatalf("rotate provisional password error = %v", err)
	}
	rotated, err := authStore.Authenticate(ctx, email, authTestNext())
	if err != nil {
		t.Fatalf("authenticate after rotation error = %v", err)
	}
	if rotated.MustChangePassword {
		t.Fatal("MustChangePassword = true after rotation, want false")
	}
	if _, err := authStore.LookupSession(ctx, digestOf(t, rotated.Token)); err != nil {
		t.Fatalf("rotated LookupSession error = %v, want nil", err)
	}
}

// A rotation exists to end the reach of the previous secret, so it must close
// the sessions that secret opened elsewhere, not only the caller's.
func TestRotatePasswordClosesEverySessionOfTheAccount(t *testing.T) {
	ctx := context.Background()
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")

	clock := newAuthClock()
	authStore := newAuthStore(t, runtimePool, clock)
	email := "rotacao@integracao.invalid"
	seedAuthAccount(t, ctx, adminPool, email, false)

	first, err := authStore.Authenticate(ctx, email, authTestPassword())
	if err != nil {
		t.Fatalf("open first session error = %v", err)
	}
	second, err := authStore.Authenticate(ctx, email, authTestPassword())
	if err != nil {
		t.Fatalf("open second session error = %v", err)
	}

	if err := authStore.RotatePassword(
		ctx, digestOf(t, first.Token), authTestPassword(), authTestNext(),
	); err != nil {
		t.Fatalf("rotate password error = %v", err)
	}
	for name, token := range map[string]string{
		"caller":    first.Token,
		"bystander": second.Token,
	} {
		if _, err := authStore.LookupSession(
			ctx, digestOf(t, token),
		); !errors.Is(err, access.ErrInvalidToken) {
			t.Fatalf("%s session error = %v, want ErrInvalidToken", name, err)
		}
	}
	if _, err := authStore.Authenticate(
		ctx, email, authTestPassword(),
	); !errors.Is(err, store.ErrAuthRejected) {
		t.Fatalf("old password error = %v, want ErrAuthRejected", err)
	}
}

// Logout is idempotent so it never discloses whether the token existed, and a
// revoked digest must stop resolving from the first call.
func TestRevokeSessionIsIdempotentAndEndsTheSession(t *testing.T) {
	ctx := context.Background()
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")

	clock := newAuthClock()
	authStore := newAuthStore(t, runtimePool, clock)
	email := "revogacao@integracao.invalid"
	seedAuthAccount(t, ctx, adminPool, email, false)

	granted, err := authStore.Authenticate(ctx, email, authTestPassword())
	if err != nil {
		t.Fatalf("open session error = %v", err)
	}
	digest := digestOf(t, granted.Token)
	for round := 1; round <= 2; round++ {
		if err := authStore.RevokeSession(ctx, digest); err != nil {
			t.Fatalf("revoke round %d error = %v, want nil", round, err)
		}
	}
	if _, err := authStore.LookupSession(ctx, digest); !errors.Is(
		err, access.ErrInvalidToken,
	) {
		t.Fatalf("revoked LookupSession error = %v, want ErrInvalidToken", err)
	}
}

// The purge must reach only rows past their absolute window: collecting a live
// session with them would log an operator out mid-shift.
func TestPurgeExpiredSessionsSparesLiveSessions(t *testing.T) {
	ctx := context.Background()
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")

	clock := newAuthClock()
	authStore := newAuthStore(t, runtimePool, clock)
	email := "expiracao@integracao.invalid"
	id := seedAuthAccount(t, ctx, adminPool, email, false)

	live, err := authStore.Authenticate(ctx, email, authTestPassword())
	if err != nil {
		t.Fatalf("open live session error = %v", err)
	}
	if err := authStore.PurgeExpiredSessions(ctx); err != nil {
		t.Fatalf("purge before expiry error = %v", err)
	}
	if _, err := authStore.LookupSession(ctx, digestOf(t, live.Token)); err != nil {
		t.Fatalf("live session after purge error = %v, want nil", err)
	}

	clock.advance(authConfig().SessionAbsoluteTTL + time.Minute)
	if err := authStore.PurgeExpiredSessions(ctx); err != nil {
		t.Fatalf("purge after expiry error = %v", err)
	}
	if remaining := countAccountSessions(t, ctx, adminPool, id); remaining != 0 {
		t.Fatalf("sessions after purge = %d, want 0", remaining)
	}
}

func countAccountSessions(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	id uuid.UUID,
) int {
	t.Helper()
	var remaining int
	err := adminPool.QueryRow(
		ctx, `SELECT count(*) FROM auth.sessions WHERE account_id = $1`, id,
	).Scan(&remaining)
	if err != nil {
		t.Fatalf("count auth sessions: %v", err)
	}
	return remaining
}
