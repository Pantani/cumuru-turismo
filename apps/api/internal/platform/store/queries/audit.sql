-- name: InsertAuditEvent :exec
INSERT INTO platform.audit_events (
  id,
  occurred_at,
  actor_subject_hmac,
  actor_hmac_key_version,
  actor_type,
  organization_id,
  action,
  entity_type,
  entity_id,
  purpose_code,
  request_id,
  changed_fields,
  outcome,
  metadata
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(occurred_at),
  sqlc.arg(actor_subject_hmac),
  sqlc.arg(actor_hmac_key_version),
  sqlc.arg(actor_type),
  sqlc.arg(organization_id),
  sqlc.arg(action),
  sqlc.arg(entity_type),
  sqlc.arg(entity_id),
  sqlc.arg(purpose_code),
  sqlc.arg(request_id),
  sqlc.arg(changed_fields),
  sqlc.arg(outcome),
  '{}'::jsonb
);
