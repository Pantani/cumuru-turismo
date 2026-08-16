BEGIN;

CREATE SCHEMA auth;

REVOKE ALL ON SCHEMA auth FROM PUBLIC;

COMMENT ON SCHEMA auth IS
  'Credencial local de operador e sessão opaca. Não guarda documento fiscal, '
  'dado de visitante ou identificador FNRH.';

CREATE TABLE auth.accounts (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  scopes text[] NOT NULL,
  status text NOT NULL DEFAULT 'active',
  failed_attempts integer NOT NULL DEFAULT 0,
  locked_until timestamptz,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT accounts_email_normalized CHECK (email = btrim(lower(email))),
  CONSTRAINT accounts_email_shaped CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
  CONSTRAINT accounts_display_name_not_blank CHECK (btrim(display_name) <> ''),
  CONSTRAINT accounts_password_hash_algorithm CHECK (password_hash LIKE '$argon2id$%'),
  CONSTRAINT accounts_scopes_bounded CHECK (
    cardinality(scopes) BETWEEN 1 AND 32
  ),
  CONSTRAINT accounts_status_valid CHECK (status IN ('active', 'disabled')),
  CONSTRAINT accounts_failed_attempts_bounded CHECK (
    failed_attempts BETWEEN 0 AND 1000000
  )
);

CREATE UNIQUE INDEX accounts_email_idx ON auth.accounts (email);

COMMENT ON TABLE auth.accounts IS
  'Conta local de operador autenticada por e-mail e senha; substitui a exigência '
  'de CNPJ ou chave federal para participar do observatório local.';
COMMENT ON COLUMN auth.accounts.password_hash IS
  'Codificação Argon2id completa, com sal por conta. Nunca é lida por '
  'worker_runtime, public_runtime ou privacy_officer.';
COMMENT ON COLUMN auth.accounts.scopes IS
  'Escopos concedidos ao principal derivado desta conta; espelham os escopos OIDC.';

CREATE TABLE auth.sessions (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES auth.accounts(id),
  token_hash bytea NOT NULL,
  issued_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  idle_expires_at timestamptz NOT NULL,
  absolute_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CONSTRAINT sessions_token_hash_sha256 CHECK (octet_length(token_hash) = 32),
  CONSTRAINT sessions_expiry_ordered CHECK (idle_expires_at <= absolute_expires_at),
  CONSTRAINT sessions_absolute_after_issue CHECK (absolute_expires_at > issued_at)
);

CREATE UNIQUE INDEX sessions_token_hash_idx ON auth.sessions (token_hash);
CREATE INDEX sessions_account_idx ON auth.sessions (account_id)
  WHERE revoked_at IS NULL;
CREATE INDEX sessions_absolute_expires_idx ON auth.sessions (absolute_expires_at);

COMMENT ON TABLE auth.sessions IS
  'Sessão opaca por SHA-256 do token; o token cru nunca é persistido.';
COMMENT ON COLUMN auth.sessions.token_hash IS
  'SHA-256 do token de sessão emitido ao cliente. Comparação por igualdade '
  'sobre o digest, nunca sobre o segredo.';

GRANT USAGE ON SCHEMA auth TO app_runtime;

GRANT SELECT (
  id,
  email,
  display_name,
  password_hash,
  scopes,
  status,
  failed_attempts,
  locked_until,
  password_changed_at
) ON TABLE auth.accounts TO app_runtime;

GRANT UPDATE (
  failed_attempts,
  locked_until,
  updated_at
) ON TABLE auth.accounts TO app_runtime;

GRANT SELECT (
  id,
  account_id,
  token_hash,
  issued_at,
  last_seen_at,
  idle_expires_at,
  absolute_expires_at,
  revoked_at
) ON TABLE auth.sessions TO app_runtime;

GRANT INSERT (
  id,
  account_id,
  token_hash,
  idle_expires_at,
  absolute_expires_at
) ON TABLE auth.sessions TO app_runtime;

GRANT UPDATE (
  last_seen_at,
  idle_expires_at,
  revoked_at
) ON TABLE auth.sessions TO app_runtime;

GRANT DELETE ON TABLE auth.sessions TO app_runtime;

REVOKE ALL ON SCHEMA auth FROM worker_runtime, public_runtime, privacy_officer;
REVOKE ALL ON ALL TABLES IN SCHEMA auth
  FROM worker_runtime, public_runtime, privacy_officer;

ALTER DEFAULT PRIVILEGES IN SCHEMA auth REVOKE ALL ON TABLES FROM PUBLIC;

COMMIT;
