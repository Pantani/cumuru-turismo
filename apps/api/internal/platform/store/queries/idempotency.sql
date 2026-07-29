-- name: ClaimIdempotencyKey :one
INSERT INTO platform.idempotency_records (
  actor_key_hmac,
  actor_key_version,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  idempotency_key_version,
  request_hash,
  state,
  expires_at
)
VALUES (
  sqlc.arg(actor_key_hmac),
  sqlc.arg(actor_key_version),
  'POST',
  sqlc.arg(operation_key),
  sqlc.arg(resource_id),
  sqlc.arg(idempotency_key_hmac),
  sqlc.arg(idempotency_key_version),
  sqlc.arg(request_hash),
  'processing',
  sqlc.arg(expires_at)
)
ON CONFLICT DO NOTHING
RETURNING
  actor_key_hmac,
  actor_key_version,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  idempotency_key_version,
  request_hash,
  state,
  response_status,
  response_headers,
  response_body,
  response_resource_id,
  created_at,
  completed_at,
  expires_at;

-- name: LockIdempotencyKey :one
SELECT
  actor_key_hmac,
  actor_key_version,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  idempotency_key_version,
  request_hash,
  state,
  response_status,
  response_headers,
  response_body,
  response_resource_id,
  created_at,
  completed_at,
  expires_at
FROM platform.idempotency_records
WHERE actor_key_hmac = sqlc.arg(actor_key_hmac)
  AND method = 'POST'
  AND operation_key = sqlc.arg(operation_key)
  AND resource_id = sqlc.arg(resource_id)
  AND idempotency_key_hmac = sqlc.arg(idempotency_key_hmac)
FOR UPDATE;

-- name: CompleteIdempotencyKey :one
UPDATE platform.idempotency_records
SET
  state = 'completed',
  response_status = sqlc.arg(response_status),
  response_headers = sqlc.arg(response_headers),
  response_body = sqlc.narg(response_body),
  response_resource_id = sqlc.narg(response_resource_id),
  completed_at = sqlc.arg(completed_at)
WHERE actor_key_hmac = sqlc.arg(actor_key_hmac)
  AND method = 'POST'
  AND operation_key = sqlc.arg(operation_key)
  AND resource_id = sqlc.arg(resource_id)
  AND idempotency_key_hmac = sqlc.arg(idempotency_key_hmac)
  AND request_hash = sqlc.arg(request_hash)
  AND state = 'processing'
RETURNING
  state,
  response_status,
  response_headers,
  response_body,
  response_resource_id,
  completed_at,
  expires_at;

-- name: CleanupExpiredOperationalRecords :one
SELECT
  (
    pg_catalog.to_jsonb(cleanup_result) ->> 'idempotency_records'
  )::bigint AS idempotency_records,
  (
    pg_catalog.to_jsonb(cleanup_result) ->> 'rate_limit_buckets'
  )::bigint AS rate_limit_buckets
FROM platform.cleanup_expired_operational_records(
  sqlc.arg(expired_before),
  sqlc.arg(batch_size)::integer
) AS cleanup_result;

-- name: IncrementRateLimit :one
INSERT INTO platform.rate_limit_buckets (
  scope,
  subject_hmac,
  subject_key_version,
  window_started_at,
  request_count,
  expires_at
)
VALUES (
  sqlc.arg(scope),
  sqlc.arg(subject_hmac),
  sqlc.arg(subject_key_version),
  sqlc.arg(window_started_at),
  1,
  sqlc.arg(expires_at)
)
ON CONFLICT (scope, subject_hmac, window_started_at)
DO UPDATE SET request_count = platform.rate_limit_buckets.request_count + 1
RETURNING request_count, expires_at;
