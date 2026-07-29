-- name: CreateStay :one
INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count
)
SELECT
  sqlc.arg(stay_id),
  a.id,
  m.id,
  'draft',
  sqlc.arg(client_submission_id),
  sqlc.arg(planned_arrival_on),
  sqlc.arg(planned_departure_on),
  sqlc.arg(expected_guest_count)
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
  AND i.use_count < i.max_uses
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
  AND i.use_count = i.max_uses
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
