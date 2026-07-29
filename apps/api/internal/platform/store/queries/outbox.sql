-- name: InsertOutboxEvent :exec
INSERT INTO platform.outbox_events (
  id,
  aggregate_type,
  aggregate_id,
  aggregate_version,
  event_type
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(aggregate_type),
  sqlc.arg(aggregate_id),
  sqlc.arg(aggregate_version),
  sqlc.arg(event_type)
);

-- name: GetOutboxBacklog :one
SELECT
  count(*)::bigint AS pending_events,
  min(occurred_at)::timestamptz AS oldest_pending_at
FROM platform.outbox_events
WHERE processed_at IS NULL;
