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

-- name: CreateActivationAccount :one
-- Conta pendente: nasce sem hash, com password_must_change false, sem
-- tentativas e sem bloqueio, exatamente o que accounts_credential_state_valid
-- exige do estado pending_activation (ADR-041).
INSERT INTO auth.accounts (
  id,
  email,
  display_name,
  scopes,
  status
)
VALUES (
  sqlc.arg(account_id),
  sqlc.arg(email),
  sqlc.arg(display_name),
  sqlc.arg(scopes),
  'pending_activation'
)
RETURNING id, email, display_name, scopes, status, created_at;

-- name: RevokeOpenActivationCapabilities :execrows
-- A reemissão revoga a capability anterior na mesma transação: o índice parcial
-- activation_capabilities_open_idx garante no máximo uma aberta por conta.
UPDATE auth.activation_capabilities AS c
SET revoked_at = sqlc.arg(revoked_at)
WHERE c.account_id = sqlc.arg(account_id)
  AND c.consumed_at IS NULL
  AND c.revoked_at IS NULL;

-- name: CreateActivationCapability :one
INSERT INTO auth.activation_capabilities (
  id,
  account_id,
  accommodation_id,
  token_hmac,
  token_key_version,
  purpose,
  expires_at
)
VALUES (
  sqlc.arg(capability_id),
  sqlc.arg(account_id),
  sqlc.arg(accommodation_id),
  sqlc.arg(token_hmac),
  sqlc.arg(token_key_version),
  'accommodation_activation',
  sqlc.arg(expires_at)
)
RETURNING id, account_id, accommodation_id, token_key_version, expires_at, created_at;

-- name: GetActivationCapability :one
-- O e-mail NÃO é projetado: a capability não é prova de titularidade do
-- endereço, então o contexto público não pode devolvê-lo.
SELECT
  c.id,
  c.account_id,
  c.accommodation_id,
  c.token_hmac,
  c.token_key_version,
  c.expires_at,
  c.consumed_at,
  c.revoked_at,
  a.name AS accommodation_name,
  a.status AS accommodation_status,
  account.display_name,
  account.status AS account_status
FROM auth.activation_capabilities AS c
JOIN core.accommodations AS a
  ON a.id = c.accommodation_id
JOIN auth.accounts AS account
  ON account.id = c.account_id
WHERE c.id = sqlc.arg(capability_id);

-- name: ConsumeActivationCapability :one
-- Uso único, atômico com a escrita do hash pelo chamador. Zero linhas é o mesmo
-- 404 uniforme de token ausente, errado, expirado, consumido ou revogado.
UPDATE auth.activation_capabilities AS c
SET consumed_at = sqlc.arg(consumed_at)
WHERE c.token_hmac = sqlc.arg(token_hmac)
  AND c.consumed_at IS NULL
  AND c.revoked_at IS NULL
  AND c.expires_at > sqlc.arg(consumed_at)
RETURNING id, account_id, accommodation_id;

-- name: ActivateAccountPassword :execrows
-- Condicionado ao estado pendente: repetir a ativação não reescreve a senha de
-- uma conta já ativa.
UPDATE auth.accounts
SET
  password_hash = sqlc.arg(password_hash),
  password_changed_at = sqlc.arg(changed_at),
  status = 'active',
  updated_at = sqlc.arg(changed_at)
WHERE id = sqlc.arg(account_id)
  AND status = 'pending_activation'
  AND password_hash IS NULL;
