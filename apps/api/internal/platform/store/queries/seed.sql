-- name: AcquireSeedRunLock :exec
SELECT pg_catalog.pg_advisory_lock(4852042520260816);

-- name: ReleaseSeedRunLock :one
SELECT pg_catalog.pg_advisory_unlock(4852042520260816);

-- name: UpsertSeedOrganization :exec
INSERT INTO core.organizations (id, name)
VALUES (sqlc.arg(id), sqlc.arg(name))
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  updated_at = now(),
  version = core.organizations.version + 1
WHERE core.organizations.name IS DISTINCT FROM EXCLUDED.name;

-- name: UpsertSeedAccommodation :exec
INSERT INTO core.accommodations (
  id,
  organization_id,
  name,
  category,
  status,
  cadastur_id,
  capacity,
  public_area_code
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(organization_id),
  sqlc.arg(name),
  sqlc.arg(category),
  'active',
  sqlc.narg(cadastur_id),
  sqlc.arg(capacity),
  sqlc.arg(public_area_code)
)
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  cadastur_id = EXCLUDED.cadastur_id,
  capacity = EXCLUDED.capacity,
  public_area_code = EXCLUDED.public_area_code,
  updated_at = now(),
  version = core.accommodations.version + 1
WHERE core.accommodations.organization_id = EXCLUDED.organization_id;

-- InsertSeedAccount never updates password_hash on conflict: re-running the
-- seeder must not reset a credential the administrator already rotated.
-- name: InsertSeedAccount :exec
INSERT INTO auth.accounts (
  id,
  email,
  display_name,
  password_hash,
  scopes,
  password_must_change
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(email),
  sqlc.arg(display_name),
  sqlc.arg(password_hash),
  sqlc.arg(scopes),
  sqlc.arg(password_must_change)
)
ON CONFLICT (email) DO UPDATE
SET
  display_name = EXCLUDED.display_name,
  scopes = EXCLUDED.scopes,
  updated_at = now();

-- name: FindSeedAccount :one
SELECT
  id,
  email,
  display_name,
  scopes,
  status,
  password_must_change
FROM auth.accounts
WHERE email = sqlc.arg(email);

-- name: UpsertSeedMembership :exec
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(accommodation_id),
  sqlc.arg(oidc_issuer),
  sqlc.arg(oidc_subject),
  'manager'
)
ON CONFLICT (accommodation_id, oidc_issuer, oidc_subject) DO UPDATE
SET
  active = true,
  updated_at = now(),
  version = core.memberships.version + 1
WHERE core.memberships.active IS DISTINCT FROM true;
