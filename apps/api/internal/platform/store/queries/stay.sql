-- name: CreateStay :one
INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  provenance
)
SELECT
  sqlc.arg(stay_id),
  a.id,
  m.id,
  'draft',
  sqlc.arg(client_submission_id),
  sqlc.arg(planned_arrival_on),
  sqlc.arg(planned_departure_on),
  sqlc.arg(expected_guest_count),
  -- Explícito em vez de depender do DEFAULT: a proveniência do fluxo nominal
  -- passa a ser legível na própria query.
  'assisted'
FROM core.accommodations AS a
JOIN core.memberships AS m
  ON m.accommodation_id = a.id
WHERE a.id = sqlc.arg(accommodation_id)
  AND a.status = 'active'
  AND m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND m.role IN ('operator', 'manager')
RETURNING
  id,
  accommodation_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  checked_in_at,
  checked_out_at,
  cancelled_at,
  no_show_at,
  cancellation_reason_code,
  no_show_reason_code,
  version,
  created_at,
  updated_at;

-- name: ListAccessibleStays :many
SELECT
  s.id,
  s.accommodation_id,
  s.status,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  (
    SELECT count(*)::integer
    FROM core.visitors AS v
    WHERE v.stay_id = s.id
  ) AS visitor_count,
  s.checked_in_at,
  s.checked_out_at,
  s.cancelled_at,
  s.no_show_at,
  s.cancellation_reason_code,
  s.no_show_reason_code,
  s.provenance,
  s.approval_state,
  s.approval_expires_at,
  s.version,
  s.created_at,
  s.updated_at
FROM core.stays AS s
JOIN core.memberships AS m
  ON m.accommodation_id = s.accommodation_id
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND (
    sqlc.narg(accommodation_id)::uuid IS NULL
    OR s.accommodation_id = sqlc.narg(accommodation_id)::uuid
  )
  AND (
    sqlc.narg(stay_status)::core.stay_status IS NULL
    OR s.status = sqlc.narg(stay_status)::core.stay_status
  )
  -- A fila de aprovação é GET /stays?accommodation_id=…&approval_state=pending.
  -- Nenhum endpoint novo de listagem: cursor, limite, ordenação e isolamento
  -- por membership continuam sendo os já provados no núcleo.
  AND (
    sqlc.narg(approval_state)::text IS NULL
    OR s.approval_state = sqlc.narg(approval_state)::text
  )
  AND (
    sqlc.narg(provenance)::text IS NULL
    OR s.provenance = sqlc.narg(provenance)::text
  )
  AND (
    sqlc.narg(arrival_from)::date IS NULL
    OR s.planned_arrival_on >= sqlc.narg(arrival_from)::date
  )
  AND (
    sqlc.narg(arrival_to)::date IS NULL
    OR s.planned_arrival_on <= sqlc.narg(arrival_to)::date
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (s.created_at, s.id) < (
      sqlc.narg(cursor_created_at)::timestamptz,
      sqlc.narg(cursor_id)::uuid
    )
  )
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetAccessibleStay :one
SELECT
  s.id,
  s.accommodation_id,
  s.status,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  (
    SELECT count(*)::integer
    FROM core.visitors AS v
    WHERE v.stay_id = s.id
  ) AS visitor_count,
  s.checked_in_at,
  s.checked_out_at,
  s.cancelled_at,
  s.no_show_at,
  s.cancellation_reason_code,
  s.no_show_reason_code,
  s.provenance,
  s.approval_state,
  s.approval_expires_at,
  s.version,
  s.created_at,
  s.updated_at,
  a.organization_id,
  a.status AS accommodation_status,
  m.id AS actor_membership_id,
  m.role AS actor_role
FROM core.stays AS s
JOIN core.accommodations AS a
  ON a.id = s.accommodation_id
JOIN core.memberships AS m
  ON m.accommodation_id = s.accommodation_id
WHERE s.id = sqlc.arg(stay_id)
  AND m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true;

-- name: UpdateStay :one
UPDATE core.stays AS s
SET
  planned_arrival_on = CASE
    WHEN sqlc.arg(set_planned_arrival)::boolean THEN sqlc.arg(planned_arrival_on)::date
    ELSE s.planned_arrival_on
  END,
  planned_departure_on = CASE
    WHEN sqlc.arg(set_planned_departure)::boolean THEN sqlc.arg(planned_departure_on)::date
    ELSE s.planned_departure_on
  END,
  expected_guest_count = CASE
    WHEN sqlc.arg(set_expected_guest_count)::boolean THEN sqlc.arg(expected_guest_count)::integer
    ELSE s.expected_guest_count
  END,
  updated_at = sqlc.arg(updated_at),
  version = s.version + 1
WHERE s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND s.status IN ('draft', 'invited', 'pre_registered', 'checked_in')
  AND EXISTS (
    SELECT 1
    FROM core.accommodations AS accommodation
    WHERE accommodation.id = s.accommodation_id
      AND accommodation.status = 'active'
  )
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role IN ('operator', 'manager')
  )
RETURNING
  s.id,
  s.accommodation_id,
  s.status,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  s.checked_in_at,
  s.checked_out_at,
  s.cancelled_at,
  s.no_show_at,
  s.cancellation_reason_code,
  s.no_show_reason_code,
  s.version,
  s.created_at,
  s.updated_at;

-- name: LockStayForCommand :one
SELECT
  s.id,
  s.accommodation_id,
  s.status,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  s.checked_in_at,
  s.checked_out_at,
  s.cancelled_at,
  s.no_show_at,
  s.cancellation_reason_code,
  s.no_show_reason_code,
  s.provenance,
  s.approval_state,
  s.approval_expires_at,
  s.version,
  s.created_at,
  s.updated_at,
  a.organization_id,
  a.status AS accommodation_status,
  m.id AS actor_membership_id,
  m.role AS actor_role,
  (
    SELECT count(*)::integer
    FROM core.visitors AS v
    WHERE v.stay_id = s.id
  ) AS visitor_count
FROM core.stays AS s
JOIN core.accommodations AS a
  ON a.id = s.accommodation_id
JOIN core.memberships AS m
  ON m.accommodation_id = s.accommodation_id
WHERE s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND m.role IN ('operator', 'manager')
FOR UPDATE OF s;

-- name: ListVisitorsForStay :many
SELECT
  v.id,
  v.role,
  v.age_band,
  v.residence_country,
  v.residence_state,
  v.residence_city_code,
  v.version
FROM core.visitors AS v
JOIN core.stays AS s
  ON s.id = v.stay_id
WHERE s.id = sqlc.arg(stay_id)
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
  )
ORDER BY v.created_at, v.id;

-- name: GetStayGroupSubmission :one
SELECT
  gs.id,
  gs.stay_id,
  gs.privacy_notice_version,
  gs.collection_channel,
  gs.submitted_at
FROM core.group_submissions AS gs
JOIN core.stays AS s
  ON s.id = gs.stay_id
WHERE gs.stay_id = sqlc.arg(stay_id)
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
  );

-- name: CreateAssistedGroupSubmission :one
INSERT INTO core.group_submissions (
  id,
  stay_id,
  client_submission_id,
  request_hash,
  privacy_notice_version,
  collection_channel,
  submitted_by_membership_id,
  submitted_at
)
SELECT
  sqlc.arg(submission_id),
  s.id,
  sqlc.arg(client_submission_id),
  sqlc.arg(request_hash),
  sqlc.arg(privacy_notice_version),
  'assisted',
  m.id,
  sqlc.arg(submitted_at)
FROM core.stays AS s
JOIN core.memberships AS m
  ON m.accommodation_id = s.accommodation_id
WHERE s.id = sqlc.arg(stay_id)
  AND s.status IN ('draft', 'invited')
  AND m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND m.role IN ('operator', 'manager')
RETURNING id, stay_id, client_submission_id, privacy_notice_version, submitted_at;

-- name: CreateInviteGroupSubmission :one
INSERT INTO core.group_submissions (
  id,
  stay_id,
  client_submission_id,
  request_hash,
  privacy_notice_version,
  collection_channel,
  submitted_at
)
SELECT
  sqlc.arg(submission_id),
  s.id,
  sqlc.arg(client_submission_id),
  sqlc.arg(request_hash),
  i.privacy_notice_version,
  'invite',
  sqlc.arg(submitted_at)
FROM core.invites AS i
JOIN core.stays AS s
  ON s.id = i.stay_id
WHERE i.id = sqlc.arg(invite_id)
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(submitted_at)
  AND s.status IN ('draft', 'invited')
RETURNING id, stay_id, client_submission_id, privacy_notice_version, submitted_at;

-- name: InsertAssistedVisitor :one
INSERT INTO core.visitors (
  id,
  stay_id,
  client_id,
  role,
  age_band,
  residence_country,
  residence_state,
  residence_city_code
)
SELECT
  sqlc.arg(visitor_id),
  s.id,
  sqlc.arg(client_id),
  sqlc.arg(visitor_role),
  sqlc.arg(age_band),
  sqlc.arg(residence_country),
  sqlc.narg(residence_state),
  sqlc.narg(residence_city_code)
FROM core.stays AS s
WHERE s.id = sqlc.arg(stay_id)
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role IN ('operator', 'manager')
  )
RETURNING id, stay_id, client_id, role, age_band, residence_country,
  residence_state, residence_city_code, version;

-- name: InsertInviteVisitor :one
INSERT INTO core.visitors (
  id,
  stay_id,
  client_id,
  role,
  age_band,
  residence_country,
  residence_state,
  residence_city_code
)
SELECT
  sqlc.arg(visitor_id),
  i.stay_id,
  sqlc.arg(client_id),
  sqlc.arg(visitor_role),
  sqlc.arg(age_band),
  sqlc.arg(residence_country),
  sqlc.narg(residence_state),
  sqlc.narg(residence_city_code)
FROM core.invites AS i
WHERE i.id = sqlc.arg(invite_id)
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(now)
RETURNING id, stay_id, client_id, role, age_band, residence_country,
  residence_state, residence_city_code, version;

-- name: RevokeActiveInvites :execrows
UPDATE core.invites AS i
SET revoked_at = sqlc.arg(revoked_at), updated_at = sqlc.arg(revoked_at)
WHERE i.stay_id = sqlc.arg(stay_id)
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(revoked_at)
  AND EXISTS (
    SELECT 1
    FROM core.stays AS s
    JOIN core.memberships AS m
      ON m.accommodation_id = s.accommodation_id
    WHERE s.id = i.stay_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role IN ('operator', 'manager')
  );

-- name: CreateInvite :one
INSERT INTO core.invites (
  id,
  stay_id,
  token_hmac,
  token_key_version,
  purpose,
  privacy_notice_version,
  expires_at
)
SELECT
  sqlc.arg(invite_id),
  s.id,
  sqlc.arg(token_hmac),
  sqlc.arg(token_key_version),
  'stay_group_submission',
  sqlc.arg(privacy_notice_version),
  sqlc.arg(expires_at)
FROM core.stays AS s
WHERE s.id = sqlc.arg(stay_id)
  AND s.status IN ('draft', 'invited')
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role IN ('operator', 'manager')
  )
RETURNING
  id,
  stay_id,
  token_hmac,
  token_key_version,
  privacy_notice_version,
  expires_at,
  max_uses,
  use_count,
  revoked_at;

-- name: GetInviteForCapability :one
SELECT
  i.id,
  i.stay_id,
  i.token_hmac,
  i.token_key_version,
  i.privacy_notice_version,
  i.expires_at,
  i.max_uses,
  i.use_count,
  i.revoked_at,
  s.status AS stay_status,
  s.version AS stay_version,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  a.name AS accommodation_name,
  a.status AS accommodation_status,
  a.organization_id
FROM core.invites AS i
JOIN core.stays AS s
  ON s.id = i.stay_id
JOIN core.accommodations AS a
  ON a.id = s.accommodation_id
WHERE i.id = sqlc.arg(invite_id);

-- name: ConsumeInvite :one
UPDATE core.invites AS i
SET
  use_count = i.use_count + 1,
  updated_at = sqlc.arg(consumed_at)
WHERE i.id = sqlc.arg(invite_id)
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(consumed_at)
  -- O teste de nulidade vem antes da comparação. Escrito como
  -- `i.use_count < i.max_uses`, o predicado avalia UNKNOWN quando max_uses é
  -- nulo, o UPDATE afeta zero linhas e o convite ilimitado passa a se comportar
  -- como convite já consumido — falha silenciosa que nenhuma suíte de convite
  -- limitado detecta.
  AND (i.max_uses IS NULL OR i.use_count < i.max_uses)
RETURNING id, stay_id, use_count, max_uses, updated_at;

-- name: FinalizeInviteSubmission :one
UPDATE core.stays AS s
SET
  expected_guest_count = sqlc.arg(expected_guest_count)::integer,
  status = 'pre_registered',
  updated_at = sqlc.arg(finalized_at),
  version = s.version + 1
FROM core.invites AS i
WHERE i.id = sqlc.arg(invite_id)
  AND i.stay_id = s.id
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(finalized_at)
  -- Mesma falha silenciosa do ConsumeInvite, pelo mesmo motivo: com max_uses
  -- nulo a igualdade avalia UNKNOWN e a estadia nunca sai de 'invited'.
  AND (i.max_uses IS NULL OR i.use_count = i.max_uses)
  AND s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND s.status IN ('draft', 'invited')
RETURNING
  s.id,
  s.status,
  s.expected_guest_count,
  s.version,
  s.updated_at;

-- name: ApplyStayTransition :one
UPDATE core.stays AS s
SET
  status = sqlc.arg(next_status),
  checked_in_at = sqlc.narg(checked_in_at),
  checked_out_at = sqlc.narg(checked_out_at),
  cancelled_at = sqlc.narg(cancelled_at),
  no_show_at = sqlc.narg(no_show_at),
  cancellation_reason_code = sqlc.narg(cancellation_reason_code),
  no_show_reason_code = sqlc.narg(no_show_reason_code),
  updated_at = sqlc.arg(updated_at),
  version = s.version + 1
WHERE s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role IN ('operator', 'manager')
  )
RETURNING
  s.id,
  s.accommodation_id,
  s.status,
  s.planned_arrival_on,
  s.planned_departure_on,
  s.expected_guest_count,
  s.checked_in_at,
  s.checked_out_at,
  s.cancelled_at,
  s.no_show_at,
  s.cancellation_reason_code,
  s.no_show_reason_code,
  s.version,
  s.created_at,
  s.updated_at;

-- name: CreateAccommodationInvite :one
-- Cartaz reutilizável da acomodação (ADR-039). max_uses nulo significa uso
-- ilimitado; stay_id fica nulo e invites_target_valid garante a exclusividade.
INSERT INTO core.invites (
  id,
  accommodation_id,
  token_hmac,
  token_key_version,
  purpose,
  privacy_notice_version,
  expires_at,
  max_uses
)
SELECT
  sqlc.arg(invite_id),
  a.id,
  sqlc.arg(token_hmac),
  sqlc.arg(token_key_version),
  'accommodation_self_registration',
  sqlc.arg(privacy_notice_version),
  sqlc.arg(expires_at),
  sqlc.narg(max_uses)::integer
FROM core.accommodations AS a
WHERE a.id = sqlc.arg(accommodation_id)
  AND a.status = 'active'
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
  id,
  accommodation_id,
  token_hmac,
  token_key_version,
  privacy_notice_version,
  expires_at,
  max_uses,
  use_count,
  revoked_at;

-- name: RevokeActiveAccommodationInvites :execrows
-- A rotação revoga o cartaz anterior na mesma transação: o índice parcial
-- invites_accommodation_single_active_idx torna dois ativos impossíveis.
UPDATE core.invites AS i
SET revoked_at = sqlc.arg(revoked_at), updated_at = sqlc.arg(revoked_at)
WHERE i.accommodation_id = sqlc.arg(accommodation_id)
  AND i.purpose = 'accommodation_self_registration'
  AND i.revoked_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = i.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role = 'manager'
  );

-- name: GetActiveAccommodationInvite :one
-- Sem token e sem HMAC na projeção: a URL só existe na criação e no replay
-- idempotente exato (ADR-019).
SELECT
  i.id,
  i.accommodation_id,
  i.privacy_notice_version,
  i.expires_at,
  i.max_uses,
  i.use_count,
  i.revoked_at
FROM core.invites AS i
WHERE i.accommodation_id = sqlc.arg(accommodation_id)
  AND i.purpose = 'accommodation_self_registration'
  AND i.revoked_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = i.accommodation_id
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role = 'manager'
  );

-- name: GetAccommodationInviteForCapability :one
SELECT
  i.id,
  i.accommodation_id,
  i.token_hmac,
  i.token_key_version,
  i.privacy_notice_version,
  i.expires_at,
  i.max_uses,
  i.use_count,
  i.revoked_at,
  a.name AS accommodation_name,
  a.status AS accommodation_status,
  a.organization_id
FROM core.invites AS i
JOIN core.accommodations AS a
  ON a.id = i.accommodation_id
WHERE i.id = sqlc.arg(invite_id)
  AND i.purpose = 'accommodation_self_registration';

-- name: CreateSelfServiceStay :one
-- created_by_membership_id é omitido de propósito: não existe autora, e
-- stays_provenance_author_valid exige exatamente isso quando a proveniência é
-- self_service. Nenhuma membership sintética de sistema é fabricada.
INSERT INTO core.stays (
  id,
  accommodation_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  provenance,
  approval_state,
  approval_expires_at
)
SELECT
  sqlc.arg(stay_id),
  i.accommodation_id,
  'pre_registered',
  sqlc.arg(client_submission_id),
  sqlc.arg(planned_arrival_on),
  sqlc.arg(planned_departure_on),
  sqlc.arg(expected_guest_count),
  'self_service',
  'pending',
  sqlc.arg(approval_expires_at)
FROM core.invites AS i
JOIN core.accommodations AS a
  ON a.id = i.accommodation_id
WHERE i.id = sqlc.arg(invite_id)
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.purpose = 'accommodation_self_registration'
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(now)
  AND a.status = 'active'
RETURNING
  id,
  accommodation_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  provenance,
  approval_state,
  approval_expires_at,
  version,
  created_at,
  updated_at;

-- name: CreateSelfServiceGroupSubmission :one
INSERT INTO core.group_submissions (
  id,
  stay_id,
  client_submission_id,
  request_hash,
  privacy_notice_version,
  collection_channel,
  submitted_at
)
SELECT
  sqlc.arg(submission_id),
  sqlc.arg(stay_id),
  sqlc.arg(client_submission_id),
  sqlc.arg(request_hash),
  i.privacy_notice_version,
  'self_service',
  sqlc.arg(submitted_at)
FROM core.invites AS i
WHERE i.id = sqlc.arg(invite_id)
  AND i.token_hmac = sqlc.arg(token_hmac)
  AND i.purpose = 'accommodation_self_registration'
  AND i.revoked_at IS NULL
  AND i.expires_at > sqlc.arg(submitted_at)
RETURNING id, stay_id, client_submission_id, privacy_notice_version, submitted_at;

-- name: InsertSelfServiceVisitor :one
-- Somente dados generalizados. O canal aberto não aceita nome, documento,
-- e-mail nem telefone (ADR-040), e role='minor' é recusado na aplicação antes
-- de chegar aqui.
INSERT INTO core.visitors (
  id,
  stay_id,
  client_id,
  role,
  age_band,
  residence_country,
  residence_state,
  residence_city_code
)
SELECT
  sqlc.arg(visitor_id),
  s.id,
  sqlc.arg(client_id),
  sqlc.arg(visitor_role),
  sqlc.arg(age_band),
  sqlc.arg(residence_country),
  sqlc.narg(residence_state),
  sqlc.narg(residence_city_code)
FROM core.stays AS s
WHERE s.id = sqlc.arg(stay_id)
  AND s.provenance = 'self_service'
  AND s.approval_state = 'pending'
RETURNING id, stay_id, client_id, role, age_band, residence_country,
  residence_state, residence_city_code, version;

-- name: DeleteSelfServiceStayVisitors :execrows
-- Rejeição e expiração eliminam os visitantes generalizados e preservam a
-- casca auditável em core.stays. Não há restrição de "pelo menos um visitante",
-- então a estadia sobrevive sem violar invariante.
DELETE FROM core.visitors AS v
USING core.stays AS s
WHERE v.stay_id = s.id
  AND s.id = sqlc.arg(stay_id)
  AND s.provenance = 'self_service';

-- name: ApproveSelfServiceStay :one
-- Exige manager ativa e acomodação ativa. O status da estadia NÃO muda: a
-- espera de aprovação é proveniência mais carimbo, nunca um estado novo da
-- máquina de estadia.
UPDATE core.stays AS s
SET
  approval_state = 'approved',
  approved_at = sqlc.arg(approved_at),
  approval_decided_by_membership_id = sqlc.arg(decided_by_membership_id),
  approval_expires_at = NULL,
  updated_at = sqlc.arg(approved_at),
  version = s.version + 1
WHERE s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND s.provenance = 'self_service'
  AND s.approval_state = 'pending'
  AND EXISTS (
    SELECT 1
    FROM core.accommodations AS a
    WHERE a.id = s.accommodation_id
      AND a.status = 'active'
  )
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.id = sqlc.arg(decided_by_membership_id)
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role = 'manager'
  )
RETURNING
  s.id,
  s.accommodation_id,
  s.status,
  s.provenance,
  s.approval_state,
  s.approved_at,
  s.version,
  s.updated_at;

-- name: RejectSelfServiceStay :one
-- Aprovação e cancelamento numa única sentença: a estadia rejeitada sai da
-- presença por dois caminhos independentes (approval_state e status).
UPDATE core.stays AS s
SET
  approval_state = 'rejected',
  approval_reason_code = sqlc.arg(reason_code),
  approval_decided_by_membership_id = sqlc.arg(decided_by_membership_id),
  approval_expires_at = NULL,
  status = 'cancelled',
  cancelled_at = sqlc.arg(rejected_at),
  cancellation_reason_code = 'accommodation_request',
  updated_at = sqlc.arg(rejected_at),
  version = s.version + 1
WHERE s.id = sqlc.arg(stay_id)
  AND s.version = sqlc.arg(expected_version)
  AND s.provenance = 'self_service'
  AND s.approval_state = 'pending'
  AND EXISTS (
    SELECT 1
    FROM core.accommodations AS a
    WHERE a.id = s.accommodation_id
      AND a.status = 'active'
  )
  AND EXISTS (
    SELECT 1
    FROM core.memberships AS m
    WHERE m.accommodation_id = s.accommodation_id
      AND m.id = sqlc.arg(decided_by_membership_id)
      AND m.oidc_issuer = sqlc.arg(oidc_issuer)
      AND m.oidc_subject = sqlc.arg(oidc_subject)
      AND m.active = true
      AND m.role = 'manager'
  )
RETURNING
  s.id,
  s.accommodation_id,
  s.status,
  s.provenance,
  s.approval_state,
  s.approval_reason_code,
  s.version,
  s.updated_at;

-- name: ExpirePendingSelfServiceStays :many
-- Varredura do worker. Eliminar somente na rejeição permitiria retenção
-- indefinida por inação, então a expiração carimba e o chamador executa a mesma
-- purga de visitantes (ADR-040). Não há membership decisora: o ramo 'expired'
-- de stays_approval_fields_valid exige exatamente isso.
-- A organização é projetada porque a varredura precisa gravar auditoria e
-- audit.Event.Validate exige organization_id não nulo para EntityStay. Como a
-- varredura não tem ator por definição, nenhuma query a resolve por membership:
-- o vínculo tem de vir da própria acomodação. O bloqueio é FOR UPDATE OF s para
-- não travar linhas de core.accommodations.
WITH candidates AS (
  SELECT s.id, a.organization_id
  FROM core.stays AS s
  JOIN core.accommodations AS a
    ON a.id = s.accommodation_id
  WHERE s.approval_state = 'pending'
    AND s.approval_expires_at <= sqlc.arg(cutoff)
  ORDER BY s.approval_expires_at, s.id
  LIMIT sqlc.arg(batch_size)
  FOR UPDATE OF s SKIP LOCKED
)
UPDATE core.stays AS s
SET
  approval_state = 'expired',
  approval_expires_at = NULL,
  status = 'cancelled',
  cancelled_at = sqlc.arg(cutoff),
  cancellation_reason_code = 'correction',
  updated_at = sqlc.arg(cutoff),
  version = s.version + 1
FROM candidates
WHERE s.id = candidates.id
RETURNING s.id, s.accommodation_id, candidates.organization_id, s.version;
