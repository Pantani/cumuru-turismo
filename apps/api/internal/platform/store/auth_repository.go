package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAuthRejected is the single answer for an unknown e-mail, a wrong password
// and a disabled account, so the response never discloses which one happened.
var ErrAuthRejected = errors.New("authentication rejected")

// ErrAuthLocked reports a temporary lockout. It is distinct from ErrAuthRejected
// because the caller answers 429 with Retry-After instead of 401.
var ErrAuthLocked = errors.New("authentication temporarily locked")

const accountStatusActive = "active"

// ErrPasswordRotationRequired reports a session that may only reach the
// rotation endpoint. It is distinct from ErrAuthRejected because the credential
// is valid; it is the provisional secret that must not survive.
var ErrPasswordRotationRequired = errors.New("password rotation required")

// ErrPasswordReused rejects a rotation that keeps the provisional secret in
// place under the appearance of a change.
var ErrPasswordReused = errors.New("password must differ from the current one")

// SessionGrant is what the transport returns to the client after a successful
// login. Token is the only place the raw secret ever exists outside the client.
type SessionGrant struct {
	Token              string
	ExpiresAt          time.Time
	AccountID          string
	Email              string
	DisplayName        string
	Scopes             []string
	MustChangePassword bool
}

type authAccount struct {
	id           uuid.UUID
	passwordHash string
	scopes       []string
	status       string
	lockedUntil  time.Time
}

// Authenticate verifies an e-mail and password pair and opens a session. The
// password never reaches a log, a metric, a trace or an audit event.
func (s *Store) Authenticate(
	ctx context.Context,
	email string,
	password string,
) (SessionGrant, error) {
	if !s.auth.Enabled {
		return SessionGrant{}, ErrAuthRejected
	}
	normalized := NormalizeEmail(email)
	if normalized == "" || access.ValidatePasswordLength(password) != nil {
		return SessionGrant{}, ErrAuthRejected
	}
	var grant SessionGrant
	err := s.inTransaction(ctx, func(queries generated.Querier) error {
		var workErr error
		grant, workErr = s.openSession(ctx, queries, normalized, password)
		return workErr
	})
	if err != nil {
		return SessionGrant{}, err
	}
	return grant, nil
}

func (s *Store) openSession(
	ctx context.Context,
	queries generated.Querier,
	email string,
	password string,
) (SessionGrant, error) {
	row, err := queries.FindAuthAccountByEmail(ctx, email)
	if err != nil {
		return SessionGrant{}, missingAccountError(err)
	}
	account := newAuthAccount(row)
	if err := s.checkAccountUsable(account); err != nil {
		return SessionGrant{}, err
	}
	if err := s.applyPasswordAttempt(ctx, queries, account, password); err != nil {
		return SessionGrant{}, err
	}
	return s.issueSession(ctx, queries, row)
}

// checkAccountUsable folds the disabled status into the generic rejection and
// keeps the lockout as its own signal.
func (s *Store) checkAccountUsable(account authAccount) error {
	if account.status != accountStatusActive {
		return ErrAuthRejected
	}
	if account.lockedUntil.After(s.now()) {
		return ErrAuthLocked
	}
	return nil
}

func (s *Store) applyPasswordAttempt(
	ctx context.Context,
	queries generated.Querier,
	account authAccount,
	password string,
) error {
	if err := access.VerifyPassword(account.passwordHash, password); err != nil {
		return s.registerFailure(ctx, queries, account.id)
	}
	if err := queries.ClearAuthFailures(ctx, pgUUID(account.id)); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *Store) registerFailure(
	ctx context.Context,
	queries generated.Querier,
	accountID uuid.UUID,
) error {
	lockedUntil := s.now().Add(s.auth.LockoutDuration)
	err := queries.RegisterAuthFailure(ctx, generated.RegisterAuthFailureParams{
		AttemptLimit: int32(s.auth.MaxFailedAttempts),
		LockedUntil:  pgTime(lockedUntil),
		ID:           pgUUID(accountID),
	})
	if err != nil {
		return ErrUnavailable
	}
	return ErrAuthRejected
}

func (s *Store) issueSession(
	ctx context.Context,
	queries generated.Querier,
	row generated.FindAuthAccountByEmailRow,
) (SessionGrant, error) {
	token, digest, err := access.NewSessionToken()
	if err != nil {
		return SessionGrant{}, err
	}
	issued := s.now()
	idleExpiry := issued.Add(s.auth.SessionIdleTTL)
	sessionID, err := uuid.NewV7()
	if err != nil {
		return SessionGrant{}, ErrUnavailable
	}
	insert := generated.InsertAuthSessionParams{
		ID:                pgUUID(sessionID),
		AccountID:         row.ID,
		TokenHash:         digest,
		IdleExpiresAt:     pgTime(idleExpiry),
		AbsoluteExpiresAt: pgTime(issued.Add(s.auth.SessionAbsoluteTTL)),
	}
	if err := queries.InsertAuthSession(ctx, insert); err != nil {
		return SessionGrant{}, ErrUnavailable
	}
	return SessionGrant{
		Token:              token,
		ExpiresAt:          idleExpiry,
		AccountID:          uuid.UUID(row.ID.Bytes).String(),
		Email:              row.Email,
		DisplayName:        row.DisplayName,
		Scopes:             row.Scopes,
		MustChangePassword: row.PasswordMustChange,
	}, nil
}

// LookupSession implements access.SessionLookup. It rejects an expired, revoked
// or disabled session and slides the idle window on every successful use.
func (s *Store) LookupSession(ctx context.Context, digest []byte) (access.Principal, error) {
	if len(digest) == 0 || s.pool == nil {
		return access.Principal{}, access.ErrInvalidToken
	}
	var principal access.Principal
	err := s.inTransaction(ctx, func(queries generated.Querier) error {
		var workErr error
		principal, workErr = s.resolveSession(ctx, queries, digest)
		return workErr
	})
	if err != nil {
		return access.Principal{}, err
	}
	return principal, nil
}

func (s *Store) resolveSession(
	ctx context.Context,
	queries generated.Querier,
	digest []byte,
) (access.Principal, error) {
	row, err := queries.FindAuthSession(ctx, digest)
	if err != nil {
		return access.Principal{}, missingSessionError(err)
	}
	now := s.now()
	if !usableSession(row, now) {
		return access.Principal{}, access.ErrInvalidToken
	}
	// A provisional secret authenticates, but authorizes nothing: every scoped
	// route is closed until the rotation lands.
	if row.PasswordMustChange {
		return access.Principal{}, access.ErrInvalidToken
	}
	touch := generated.TouchAuthSessionParams{
		SeenAt:        pgTime(now),
		IdleExpiresAt: pgTime(now.Add(s.auth.SessionIdleTTL)),
		ID:            row.ID,
	}
	if err := queries.TouchAuthSession(ctx, touch); err != nil {
		return access.Principal{}, ErrUnavailable
	}
	subject := uuid.UUID(row.AccountID.Bytes).String()
	return access.NewPrincipal(access.LocalSessionIssuer, subject, row.Scopes), nil
}

// DescribeSession returns the account behind a live session so a client can
// rehydrate after a reload without persisting anything locally. It never issues
// a new token and never slides the idle window.
func (s *Store) DescribeSession(ctx context.Context, digest []byte) (SessionGrant, error) {
	if len(digest) == 0 || s.pool == nil {
		return SessionGrant{}, access.ErrInvalidToken
	}
	var grant SessionGrant
	err := s.inReadOnlyTransaction(ctx, func(queries generated.Querier) error {
		var readErr error
		grant, readErr = s.readSession(ctx, queries, digest)
		return readErr
	})
	if err != nil {
		return SessionGrant{}, err
	}
	return grant, nil
}

func (s *Store) readSession(
	ctx context.Context,
	queries generated.Querier,
	digest []byte,
) (SessionGrant, error) {
	row, err := queries.FindAuthSession(ctx, digest)
	if err != nil {
		return SessionGrant{}, missingSessionError(err)
	}
	if !usableSession(row, s.now()) {
		return SessionGrant{}, access.ErrInvalidToken
	}
	return describeGrant(row), nil
}

func describeGrant(row generated.FindAuthSessionRow) SessionGrant {
	return SessionGrant{
		ExpiresAt:          row.IdleExpiresAt.Time,
		AccountID:          uuid.UUID(row.AccountID.Bytes).String(),
		Email:              row.Email,
		DisplayName:        row.DisplayName,
		Scopes:             row.Scopes,
		MustChangePassword: row.PasswordMustChange,
	}
}

func usableSession(row generated.FindAuthSessionRow, now time.Time) bool {
	if row.Status != accountStatusActive {
		return false
	}
	if !row.IdleExpiresAt.Valid || !row.AbsoluteExpiresAt.Valid {
		return false
	}
	return now.Before(row.IdleExpiresAt.Time) && now.Before(row.AbsoluteExpiresAt.Time)
}

// RevokeSession is idempotent: an unknown or already revoked digest succeeds so
// logout never leaks whether the token existed.
func (s *Store) RevokeSession(ctx context.Context, digest []byte) error {
	if len(digest) == 0 {
		return nil
	}
	return s.inTransaction(ctx, func(queries generated.Querier) error {
		err := queries.RevokeAuthSession(ctx, generated.RevokeAuthSessionParams{
			RevokedAt: pgTime(s.now()),
			TokenHash: digest,
		})
		if err != nil {
			return ErrUnavailable
		}
		return nil
	})
}

// RotatePassword replaces the secret of the account behind a live session and
// closes every session that secret had opened, the caller's included. The
// client authenticates again with the new password.
func (s *Store) RotatePassword(
	ctx context.Context,
	digest []byte,
	current string,
	next string,
) error {
	if len(digest) == 0 || s.pool == nil {
		return access.ErrInvalidToken
	}
	if access.ValidatePasswordLength(next) != nil {
		return access.ErrInvalidPassword
	}
	if current == next {
		return ErrPasswordReused
	}
	return s.inTransaction(ctx, func(queries generated.Querier) error {
		return s.applyRotation(ctx, queries, digest, current, next)
	})
}

func (s *Store) applyRotation(
	ctx context.Context,
	queries generated.Querier,
	digest []byte,
	current string,
	next string,
) error {
	accountID, err := s.rotatingAccount(ctx, queries, digest)
	if err != nil {
		return err
	}
	account, err := queries.FindAuthAccountByID(ctx, pgUUID(accountID))
	if err != nil {
		return missingAccountError(err)
	}
	if access.VerifyPassword(account.PasswordHash, current) != nil {
		return ErrAuthRejected
	}
	return s.writeRotation(ctx, queries, accountID, next)
}

// rotatingAccount accepts a session whose account still carries the provisional
// flag, which LookupSession refuses; the rotation endpoint is the one path that
// such a session may reach.
func (s *Store) rotatingAccount(
	ctx context.Context,
	queries generated.Querier,
	digest []byte,
) (uuid.UUID, error) {
	row, err := queries.FindAuthSession(ctx, digest)
	if err != nil {
		return uuid.Nil, missingSessionError(err)
	}
	if !usableSession(row, s.now()) {
		return uuid.Nil, access.ErrInvalidToken
	}
	return uuid.UUID(row.AccountID.Bytes), nil
}

func (s *Store) writeRotation(
	ctx context.Context,
	queries generated.Querier,
	accountID uuid.UUID,
	next string,
) error {
	hash, err := access.NewPasswordHasher().Hash(next)
	if err != nil {
		return access.ErrInvalidPassword
	}
	rotate := generated.RotateAccountPasswordParams{
		PasswordHash: hash,
		ChangedAt:    pgTime(s.now()),
		ID:           pgUUID(accountID),
	}
	if err := queries.RotateAccountPassword(ctx, rotate); err != nil {
		return ErrUnavailable
	}
	revoke := generated.RevokeAccountSessionsParams{
		RevokedAt: pgTime(s.now()),
		AccountID: pgUUID(accountID),
	}
	if err := queries.RevokeAccountSessions(ctx, revoke); err != nil {
		return ErrUnavailable
	}
	return nil
}

// PurgeExpiredSessions drops sessions past their absolute expiry.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	return s.inTransaction(ctx, func(queries generated.Querier) error {
		if err := queries.DeleteExpiredAuthSessions(ctx, pgTime(s.now())); err != nil {
			return ErrUnavailable
		}
		return nil
	})
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newAuthAccount(row generated.FindAuthAccountByEmailRow) authAccount {
	account := authAccount{
		id:           uuid.UUID(row.ID.Bytes),
		passwordHash: row.PasswordHash,
		scopes:       row.Scopes,
		status:       row.Status,
	}
	if row.LockedUntil.Valid {
		account.lockedUntil = row.LockedUntil.Time
	}
	return account
}

func missingAccountError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAuthRejected
	}
	return ErrUnavailable
}

func missingSessionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return access.ErrInvalidToken
	}
	return ErrUnavailable
}
