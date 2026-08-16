BEGIN;

REVOKE UPDATE (
  password_hash,
  password_changed_at,
  password_must_change
) ON TABLE auth.accounts FROM app_runtime;

ALTER TABLE auth.accounts
  DROP COLUMN password_must_change;

COMMIT;
