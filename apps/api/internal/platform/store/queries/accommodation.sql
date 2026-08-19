-- name: ListAccessibleAccommodations :many
SELECT
  a.id,
  a.organization_id,
  a.name,
  a.category,
  a.status,
  a.cadastur_id,
  a.capacity,
  a.public_area_code,
  a.public_listing_enabled,
  a.public_contact_phone,
  a.public_contact_whatsapp,
  a.public_website_url,
  a.public_listing_consented_at,
  a.version,
  a.created_at,
  a.updated_at
FROM core.accommodations AS a
JOIN core.memberships AS m
  ON m.accommodation_id = a.id
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (a.created_at, a.id) < (
      sqlc.narg(cursor_created_at)::timestamptz,
      sqlc.narg(cursor_id)::uuid
    )
  )
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetAccessibleAccommodation :one
SELECT
  a.id,
  a.organization_id,
  a.name,
  a.category,
  a.status,
  a.cadastur_id,
  a.capacity,
  a.public_area_code,
  a.public_listing_enabled,
  a.public_contact_phone,
  a.public_contact_whatsapp,
  a.public_website_url,
  a.public_listing_consented_at,
  a.version,
  a.created_at,
  a.updated_at,
  m.id AS actor_membership_id,
  m.role AS actor_role
FROM core.accommodations AS a
JOIN core.memberships AS m
  ON m.accommodation_id = a.id
WHERE a.id = sqlc.arg(accommodation_id)
  AND m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true;

-- name: AcquireAccommodationOnboardingLock :exec
SELECT pg_catalog.pg_advisory_xact_lock(
  pg_catalog.hashtextextended(sqlc.arg(actor_lock_key)::text, 485202)
);

-- name: ListAccommodationOnboardingOrganizations :many
SELECT
  accommodation.organization_id,
  pg_catalog.bool_or(membership.role = 'manager') AS has_manager_membership
FROM core.memberships AS membership
JOIN core.accommodations AS accommodation
  ON accommodation.id = membership.accommodation_id
WHERE membership.oidc_issuer = sqlc.arg(oidc_issuer)
  AND membership.oidc_subject = sqlc.arg(oidc_subject)
  AND membership.active = true
GROUP BY accommodation.organization_id
ORDER BY accommodation.organization_id;

-- name: FindOnboardedAccommodation :one
SELECT
  accommodation.id,
  accommodation.organization_id,
  accommodation.name,
  accommodation.category,
  accommodation.status,
  accommodation.cadastur_id,
  accommodation.capacity,
  accommodation.public_area_code,
  accommodation.public_listing_enabled,
  accommodation.public_contact_phone,
  accommodation.public_contact_whatsapp,
  accommodation.public_website_url,
  accommodation.public_listing_consented_at,
  accommodation.version,
  accommodation.created_at,
  accommodation.updated_at
FROM core.accommodations AS accommodation
JOIN core.memberships AS membership
  ON membership.accommodation_id = accommodation.id
WHERE accommodation.organization_id = sqlc.arg(organization_id)
  AND accommodation.onboarding_submission_id = sqlc.arg(onboarding_submission_id)
  AND membership.oidc_issuer = sqlc.arg(oidc_issuer)
  AND membership.oidc_subject = sqlc.arg(oidc_subject)
  AND membership.active = true;

-- name: InsertOnboardingOrganization :one
INSERT INTO core.organizations (
  id,
  name
)
VALUES (
  sqlc.arg(organization_id),
  sqlc.arg(name)
)
RETURNING id;

-- name: InsertOnboardingAccommodation :one
INSERT INTO core.accommodations (
  id,
  organization_id,
  name,
  category,
  status,
  capacity,
  onboarding_submission_id
)
VALUES (
  sqlc.arg(accommodation_id),
  sqlc.arg(organization_id),
  sqlc.arg(name),
  sqlc.arg(category),
  'active',
  sqlc.arg(capacity),
  sqlc.arg(onboarding_submission_id)
)
RETURNING
  id,
  organization_id,
  name,
  category,
  status,
  cadastur_id,
  capacity,
  public_area_code,
  public_listing_enabled,
  public_contact_phone,
  public_contact_whatsapp,
  public_website_url,
  public_listing_consented_at,
  version,
  created_at,
  updated_at;

-- name: InsertOnboardingManagerMembership :one
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  sqlc.arg(membership_id),
  sqlc.arg(accommodation_id),
  sqlc.arg(oidc_issuer),
  sqlc.arg(oidc_subject),
  'manager'
)
RETURNING
  id,
  accommodation_id,
  role,
  active,
  version,
  created_at,
  updated_at;

-- name: UpdateAccommodation :one
UPDATE core.accommodations AS a
SET
  name = CASE
    WHEN sqlc.arg(set_name)::boolean THEN sqlc.arg(name)::text
    ELSE a.name
  END,
  category = CASE
    WHEN sqlc.arg(set_category)::boolean THEN sqlc.arg(category)::text
    ELSE a.category
  END,
  capacity = CASE
    WHEN sqlc.arg(set_capacity)::boolean THEN sqlc.narg(capacity)::integer
    ELSE a.capacity
  END,
  public_area_code = CASE
    WHEN sqlc.arg(set_public_area_code)::boolean THEN sqlc.narg(public_area_code)::text
    ELSE a.public_area_code
  END,
  public_listing_enabled = CASE
    WHEN sqlc.arg(set_public_listing_enabled)::boolean
      THEN sqlc.arg(public_listing_enabled)::boolean
    ELSE a.public_listing_enabled
  END,
  public_contact_phone = CASE
    WHEN sqlc.arg(set_public_contact_phone)::boolean
      THEN sqlc.narg(public_contact_phone)::text
    ELSE a.public_contact_phone
  END,
  public_contact_whatsapp = CASE
    WHEN sqlc.arg(set_public_contact_whatsapp)::boolean
      THEN sqlc.arg(public_contact_whatsapp)::boolean
    ELSE a.public_contact_whatsapp
  END,
  public_website_url = CASE
    WHEN sqlc.arg(set_public_website_url)::boolean
      THEN sqlc.narg(public_website_url)::text
    ELSE a.public_website_url
  END,
  -- O carimbo é do consentimento, não da edição: publicar de novo o que já
  -- estava publicado não reescreve a data em que a hospedagem consentiu, e
  -- despublicar a elimina em vez de guardar consentimento que foi retirado.
  public_listing_consented_at = CASE
    WHEN NOT sqlc.arg(set_public_listing_enabled)::boolean
      THEN a.public_listing_consented_at
    WHEN sqlc.arg(public_listing_enabled)::boolean
      THEN pg_catalog.coalesce(
        a.public_listing_consented_at,
        sqlc.arg(updated_at)::timestamptz
      )
    ELSE NULL
  END,
  updated_at = sqlc.arg(updated_at),
  version = a.version + 1
WHERE a.id = sqlc.arg(accommodation_id)
  AND a.version = sqlc.arg(expected_version)
  AND a.status <> 'closed'
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = a.id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role = 'manager'
  )
RETURNING
  a.id,
  a.organization_id,
  a.name,
  a.category,
  a.status,
  a.cadastur_id,
  a.capacity,
  a.public_area_code,
  a.public_listing_enabled,
  a.public_contact_phone,
  a.public_contact_whatsapp,
  a.public_website_url,
  a.public_listing_consented_at,
  a.version,
  a.created_at,
  a.updated_at;

-- name: ListAccommodationMemberships :many
SELECT
  target.id,
  target.accommodation_id,
  target.oidc_issuer,
  target.oidc_subject,
  target.role,
  target.active,
  target.version,
  target.created_at,
  target.updated_at
FROM core.memberships AS target
WHERE target.accommodation_id = sqlc.arg(accommodation_id)
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS actor
    WHERE actor.accommodation_id = target.accommodation_id
      AND actor.oidc_issuer = sqlc.arg(oidc_issuer)
      AND actor.oidc_subject = sqlc.arg(oidc_subject)
      AND actor.active = true
      AND actor.role = 'manager'
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (target.created_at, target.id) < (
      sqlc.narg(cursor_created_at)::timestamptz,
      sqlc.narg(cursor_id)::uuid
    )
  )
ORDER BY target.created_at DESC, target.id DESC
LIMIT sqlc.arg(page_limit);

-- name: CreateAccommodationMembership :one
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
SELECT
  sqlc.arg(membership_id),
  a.id,
  sqlc.arg(target_oidc_issuer),
  sqlc.arg(target_oidc_subject),
  sqlc.arg(target_role)
FROM core.accommodations AS a
WHERE a.id = sqlc.arg(accommodation_id)
  AND a.status <> 'closed'
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS actor
    WHERE actor.accommodation_id = a.id
      AND actor.oidc_issuer = sqlc.arg(oidc_issuer)
      AND actor.oidc_subject = sqlc.arg(oidc_subject)
      AND actor.active = true
      AND actor.role = 'manager'
  )
RETURNING
  id,
  accommodation_id,
  role,
  active,
  version,
  created_at,
  updated_at;

-- name: LockMembershipSetForManager :many
SELECT
  target.id,
  target.role,
  target.active,
  target.version
FROM core.memberships AS target
JOIN core.accommodations AS accommodation
  ON accommodation.id = target.accommodation_id
WHERE target.accommodation_id = sqlc.arg(accommodation_id)
  AND accommodation.status <> 'closed'
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS actor
    WHERE actor.accommodation_id = target.accommodation_id
      AND actor.oidc_issuer = sqlc.arg(oidc_issuer)
      AND actor.oidc_subject = sqlc.arg(oidc_subject)
      AND actor.active = true
      AND actor.role = 'manager'
  )
ORDER BY target.id
FOR UPDATE OF target;

-- name: UpdateAccommodationMembership :one
UPDATE core.memberships AS target
SET
  role = sqlc.arg(next_role),
  active = sqlc.arg(next_active),
  updated_at = sqlc.arg(updated_at),
  version = target.version + 1
WHERE target.id = sqlc.arg(membership_id)
  AND target.accommodation_id = sqlc.arg(accommodation_id)
  AND target.version = sqlc.arg(expected_version)
  AND EXISTS (
    SELECT 1
    FROM core.accommodations AS accommodation
    WHERE accommodation.id = target.accommodation_id
      AND accommodation.status <> 'closed'
  )
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS actor
    WHERE actor.accommodation_id = target.accommodation_id
      AND actor.oidc_issuer = sqlc.arg(oidc_issuer)
      AND actor.oidc_subject = sqlc.arg(oidc_subject)
      AND actor.active = true
      AND actor.role = 'manager'
  )
RETURNING
  target.id,
  target.accommodation_id,
  target.oidc_issuer,
  target.oidc_subject,
  target.role,
  target.active,
  target.version,
  target.created_at,
  target.updated_at;

-- name: CreateActivationManagerMembership :one
-- Vínculo da conta pendente com a acomodação (ADR-041). Gated em
-- a.status = 'active' porque issue_activation só existe na acomodação ativa;
-- CreateAccommodationMembership é mais permissiva (<> 'closed') e não serve.
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
SELECT
  sqlc.arg(membership_id),
  a.id,
  sqlc.arg(target_oidc_issuer),
  sqlc.arg(target_oidc_subject),
  'manager'
FROM core.accommodations AS a
WHERE a.id = sqlc.arg(accommodation_id)
  AND a.status = 'active'
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS actor
    WHERE actor.accommodation_id = a.id
      AND actor.oidc_issuer = sqlc.arg(oidc_issuer)
      AND actor.oidc_subject = sqlc.arg(oidc_subject)
      AND actor.active = true
      AND actor.role = 'manager'
  )
RETURNING
  id,
  accommodation_id,
  role,
  active,
  version,
  created_at,
  updated_at;

-- name: ListPublicAccommodationDirectory :many
-- Lista pública de hospedagens. O filtro está aqui, e não no chamador: a rota
-- é aberta, e uma consulta que dependesse do handler para recortar publicaria
-- o cadastro inteiro no primeiro erro de quem a chamasse. Sem paginação por
-- cursor de propósito — a lista é municipal e cabe num documento só, cacheável
-- por inteiro; o limite acima do teto existe para a leitura recusar em vez de
-- truncar em silêncio.
SELECT
  a.id,
  a.name,
  a.category,
  a.capacity,
  a.public_area_code,
  a.public_contact_phone,
  a.public_contact_whatsapp,
  a.public_website_url,
  a.updated_at
FROM core.accommodations AS a
WHERE a.public_listing_enabled = true
  AND a.status = 'active'
ORDER BY a.name, a.id
LIMIT sqlc.arg(page_limit);
