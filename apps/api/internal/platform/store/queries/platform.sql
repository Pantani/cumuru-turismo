-- name: CheckReadiness :one
SELECT 1::integer AS ready;

-- name: ListActiveTenantMemberships :many
SELECT
  m.id AS membership_id,
  m.role,
  a.id AS accommodation_id,
  a.organization_id
FROM core.memberships AS m
JOIN core.accommodations AS a ON a.id = m.accommodation_id
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND a.status = 'active'
ORDER BY a.organization_id, a.id;
