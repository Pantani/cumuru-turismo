-- name: FindAuthAccountByEmail :one
SELECT
  id,
  email,
  display_name,
  password_hash,
  scopes,
  status,
  failed_attempts,
  locked_until,
  password_must_change
FROM auth.accounts
WHERE email = sqlc.arg(email);

-- name: FindAuthAccountByID :one
SELECT
  id,
  email,
  display_name,
  password_hash,
  scopes,
  status,
  password_must_change
FROM auth.accounts
WHERE id = sqlc.arg(id);

-- RotateAccountPassword clears the provisional flag in the same statement that
-- writes the new secret, so a rotation cannot land while leaving the session
-- restricted.
-- name: RotateAccountPassword :exec
UPDATE auth.accounts
SET
  password_hash = sqlc.arg(password_hash),
  password_changed_at = sqlc.arg(changed_at),
  password_must_change = false,
  updated_at = now()
WHERE id = sqlc.arg(id);

-- name: RegisterAuthFailure :exec
UPDATE auth.accounts
SET
  failed_attempts = failed_attempts + 1,
  locked_until = CASE
    WHEN failed_attempts + 1 >= sqlc.arg(attempt_limit)::integer
      THEN sqlc.arg(locked_until)::timestamptz
    ELSE locked_until
  END,
  updated_at = now()
WHERE id = sqlc.arg(id);

-- name: ClearAuthFailures :exec
UPDATE auth.accounts
SET
  failed_attempts = 0,
  locked_until = NULL,
  updated_at = now()
WHERE id = sqlc.arg(id)
  AND (failed_attempts <> 0 OR locked_until IS NOT NULL);

-- name: InsertAuthSession :exec
INSERT INTO auth.sessions (
  id,
  account_id,
  token_hash,
  idle_expires_at,
  absolute_expires_at
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(account_id),
  sqlc.arg(token_hash),
  sqlc.arg(idle_expires_at),
  sqlc.arg(absolute_expires_at)
);

-- name: FindAuthSession :one
SELECT
  session.id,
  session.account_id,
  session.idle_expires_at,
  session.absolute_expires_at,
  account.email,
  account.display_name,
  account.scopes,
  account.status,
  account.password_must_change
FROM auth.sessions AS session
JOIN auth.accounts AS account ON account.id = session.account_id
WHERE session.token_hash = sqlc.arg(token_hash)
  AND session.revoked_at IS NULL;

-- name: TouchAuthSession :exec
UPDATE auth.sessions
SET
  last_seen_at = sqlc.arg(seen_at),
  idle_expires_at = LEAST(
    sqlc.arg(idle_expires_at)::timestamptz,
    absolute_expires_at
  )
WHERE id = sqlc.arg(id)
  AND revoked_at IS NULL;

-- name: RevokeAuthSession :exec
UPDATE auth.sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- RevokeAccountSessions closes every open session of an account. A rotation
-- ends the sessions the previous secret opened, including the one that
-- requested it.
-- name: RevokeAccountSessions :exec
UPDATE auth.sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE account_id = sqlc.arg(account_id)
  AND revoked_at IS NULL;

-- name: DeleteExpiredAuthSessions :exec
DELETE FROM auth.sessions
WHERE absolute_expires_at < sqlc.arg(cutoff);
