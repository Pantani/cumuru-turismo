#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-migration-test-${PPID}-$$"
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.migration-test.yaml"
  --project-name "${PROJECT_NAME}"
)
MIGRATION_URL="postgres://cumuru_migration:cumuru-local-migration-only@postgres:5432/cumuru?sslmode=disable"

cleanup() {
  local primary_status=$?
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1
  local cleanup_status=$?
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  exit "${cleanup_status}"
}
trap cleanup EXIT

run_migrate() {
  "${COMPOSE[@]}" run --rm --no-deps migrate \
    -path=/migrations -database="${MIGRATION_URL}" "$@"
}

psql_as() {
  local user="$1"
  local password="$2"
  shift 2
  "${COMPOSE[@]}" exec -T \
    -e "PGPASSWORD=${password}" \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username="${user}" --dbname=cumuru "$@"
}

expect_psql_failure() {
  local user="$1"
  local password="$2"
  local description="$3"
  local statement="$4"
  if psql_as "${user}" "${password}" --command="${statement}" >/dev/null 2>&1; then
    echo "${description}" >&2
    exit 1
  fi
}

migration_files="$(
  find "${ROOT_DIR}/apps/api/migrations" \
    -maxdepth 1 -type f -name '*.sql' -exec basename {} \; |
    LC_ALL=C sort
)"
expected_migration_files=$'000001_initial_schema.down.sql\n000001_initial_schema.up.sql'
test "${migration_files}" = "${expected_migration_files}"

"${COMPOSE[@]}" up --detach --wait postgres

run_migrate up 1
actual_migration_state="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="SELECT version || ':' || dirty FROM public.schema_migrations"
)"
test "${actual_migration_state}" = "1:false"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO core.organizations (id, name)
VALUES
  ('00000000-0000-7000-8000-000000000001', 'Organização Fictícia A'),
  ('00000000-0000-7000-8000-000000000002', 'Organização Fictícia B');

INSERT INTO core.accommodations (id, organization_id, name, category, status)
VALUES
  (
    '00000000-0000-7000-8000-000000000011',
    '00000000-0000-7000-8000-000000000001',
    'Hospedagem Fictícia A',
    'prototype',
    'active'
  ),
  (
    '00000000-0000-7000-8000-000000000012',
    '00000000-0000-7000-8000-000000000002',
    'Hospedagem Fictícia B',
    'prototype',
    'active'
  );

INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES
  (
    '00000000-0000-7000-8000-000000000021',
    '00000000-0000-7000-8000-000000000011',
    'https://oidc.invalid/local',
    'operator-a',
    'operator'
  ),
  (
    '00000000-0000-7000-8000-000000000022',
    '00000000-0000-7000-8000-000000000012',
    'https://oidc.invalid/local',
    'operator-b',
    'operator'
  );

INSERT INTO platform.audit_events (
  id,
  actor_subject_hmac,
  actor_hmac_key_version,
  actor_type,
  action,
  entity_type,
  outcome
)
VALUES (
  '00000000-0000-7000-8000-000000000023',
  decode('aa', 'hex'),
  'fixture-v1',
  'user',
  'phase1.fixture',
  'accommodation',
  'success'
);
SQL

preserved_memberships="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command='SELECT count(*) FROM core.memberships WHERE version = 1'
)"
test "${preserved_memberships}" = "2"

audit_key_version="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT actor_hmac_key_version
      FROM platform.audit_events
      WHERE id = '00000000-0000-7000-8000-000000000023'
    "
)"
test "${audit_key_version}" = "fixture-v1"

outbox_payload_columns="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM information_schema.columns
      WHERE table_schema = 'platform'
        AND table_name = 'outbox_events'
        AND column_name = 'payload'
    "
)"
test "${outbox_payload_columns}" = "0"

preserved_phase2_stays="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command='SELECT count(*) FROM core.stays'
)"
test "${preserved_phase2_stays}" = "0"

survey_catalog_tables="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM information_schema.tables
      WHERE table_schema = 'survey'
        AND table_name IN (
          'questionnaires',
          'questionnaire_versions',
          'questions',
          'question_options',
          'consent_requirements',
          'capabilities',
          'responses',
          'answers',
          'consent_decisions'
        )
    "
)"
test "${survey_catalog_tables}" = "9"

psql_as cumuru_app cumuru-local-app-only <<'SQL'
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  '00000000-0000-7000-8000-000000000024',
  '00000000-0000-7000-8000-000000000011',
  'https://oidc.invalid/local',
  'operator-c',
  'operator'
);

INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count
)
VALUES (
  '00000000-0000-7000-8000-000000000031',
  '00000000-0000-7000-8000-000000000011',
  '00000000-0000-7000-8000-000000000021',
  '00000000-0000-7000-8000-000000000041',
  DATE '2026-12-10',
  DATE '2026-12-12',
  2
);

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
  created_at,
  expires_at
)
VALUES (
  decode('01', 'hex'),
  'actor-v1',
  'POST',
  'createStay',
  '00000000-0000-7000-8000-000000000011',
  decode('02', 'hex'),
  'idempotency-v1',
  decode('03', 'hex'),
  'processing',
  now() - interval '31 days',
  now() - interval '1 day'
);

INSERT INTO platform.outbox_events (
  id,
  aggregate_type,
  aggregate_id,
  aggregate_version,
  event_type
)
VALUES (
  '00000000-0000-7000-8000-000000000051',
  'stay',
  '00000000-0000-7000-8000-000000000031',
  1,
  'stay.created'
);

INSERT INTO platform.rate_limit_buckets (
  scope,
  subject_hmac,
  subject_key_version,
  window_started_at,
  request_count,
  expires_at
)
VALUES (
  'invite_context',
  decode('04', 'hex'),
  'rate-v1',
  now() - interval '2 minutes',
  1,
  now() - interval '1 minute'
);
SQL

producer_outbox_defaults="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT (
        occurred_at IS NOT NULL
        AND available_at IS NOT NULL
        AND lease_owner IS NULL
        AND lease_until IS NULL
        AND attempts = 0
        AND processed_at IS NULL
        AND last_error_code IS NULL
      )::integer
      FROM platform.outbox_events
      WHERE id = '00000000-0000-7000-8000-000000000051'
    "
)"
test "${producer_outbox_defaults}" = "1"

visible_count="$(
  psql_as cumuru_app cumuru-local-app-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM core.memberships AS m
      JOIN core.accommodations AS a ON a.id = m.accommodation_id
      WHERE m.oidc_issuer = 'https://oidc.invalid/local'
        AND m.oidc_subject = 'operator-a'
        AND m.active = true
        AND a.status = 'active'
    "
)"
test "${visible_count}" = "1"

psql_as cumuru_app cumuru-local-app-only \
  --command="
    UPDATE core.accommodations
    SET capacity = 12, updated_at = now(), version = version + 1
    WHERE id = '00000000-0000-7000-8000-000000000011'
  "

psql_as cumuru_app cumuru-local-app-only \
  --command="
    UPDATE core.memberships
    SET
      role = 'manager',
      active = true,
      updated_at = now(),
      version = version + 1
    WHERE id = '00000000-0000-7000-8000-000000000021'
  "

for protected_column in \
  "oidc_subject = oidc_subject" \
  "oidc_issuer = oidc_issuer" \
  "accommodation_id = accommodation_id" \
  "id = id" \
  "created_at = created_at"; do
  expect_psql_failure \
    cumuru_app \
    cumuru-local-app-only \
    "app_runtime unexpectedly updated protected membership column: ${protected_column}" \
    "UPDATE core.memberships SET ${protected_column}
     WHERE id = '00000000-0000-7000-8000-000000000021'"
done

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly selected platform.outbox_events" \
  "SELECT count(*) FROM platform.outbox_events"

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly updated platform.outbox_events" \
  "UPDATE platform.outbox_events SET available_at = available_at"

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly deleted platform.outbox_events" \
  "DELETE FROM platform.outbox_events"

while IFS='|' read -r protected_column protected_value; do
  expect_psql_failure \
    cumuru_app \
    cumuru-local-app-only \
    "app_runtime unexpectedly inserted protected outbox column: ${protected_column}" \
    "INSERT INTO platform.outbox_events (
       id,
       aggregate_type,
       aggregate_id,
       aggregate_version,
       event_type,
       ${protected_column}
     ) VALUES (
       '00000000-0000-7000-8000-000000000052',
       'stay',
       '00000000-0000-7000-8000-000000000031',
       2,
       'stay.updated',
       ${protected_value}
     )"
done <<'OUTBOX_PROTECTED_INSERTS'
lease_owner|'app-fixture'
lease_until|now()
attempts|1
processed_at|now()
last_error_code|'fixture_error'
occurred_at|now()
available_at|now()
OUTBOX_PROTECTED_INSERTS

worker_outbox_count="$(
  psql_as cumuru_worker cumuru-local-worker-only \
    --tuples-only --no-align \
    --command='SELECT count(*) FROM platform.outbox_events'
)"
test "${worker_outbox_count}" = "1"

psql_as cumuru_worker cumuru-local-worker-only \
  --command="
    UPDATE platform.outbox_events
    SET
      available_at = now(),
      lease_owner = 'worker-fixture',
      lease_until = now() + interval '1 minute',
      attempts = attempts + 1,
      processed_at = now(),
      last_error_code = NULL
    WHERE id = '00000000-0000-7000-8000-000000000051'
  "

for protected_column in \
  "id = id" \
  "aggregate_type = aggregate_type" \
  "aggregate_id = aggregate_id" \
  "aggregate_version = aggregate_version" \
  "event_type = event_type" \
  "occurred_at = occurred_at"; do
  expect_psql_failure \
    cumuru_worker \
    cumuru-local-worker-only \
    "worker_runtime unexpectedly updated immutable outbox column: ${protected_column}" \
    "UPDATE platform.outbox_events SET ${protected_column}
     WHERE id = '00000000-0000-7000-8000-000000000051'"
done

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly inserted platform.outbox_events" \
  "INSERT INTO platform.outbox_events (
     id, aggregate_type, aggregate_id, aggregate_version, event_type
   ) VALUES (
     '00000000-0000-7000-8000-000000000052',
     'stay',
     '00000000-0000-7000-8000-000000000031',
     2,
     'stay.updated'
   )"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly selected protected idempotency columns" \
  "SELECT actor_key_hmac FROM platform.idempotency_records"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly updated platform.idempotency_records" \
  "UPDATE platform.idempotency_records SET expires_at = expires_at"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly selected protected rate-limit columns" \
  "SELECT subject_hmac FROM platform.rate_limit_buckets"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly updated platform.rate_limit_buckets" \
  "UPDATE platform.rate_limit_buckets SET expires_at = expires_at"

psql_as cumuru_worker cumuru-local-worker-only \
  --command="DELETE FROM platform.outbox_events WHERE processed_at IS NOT NULL"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="
    DELETE FROM platform.idempotency_records
    WHERE actor_key_hmac = decode('01', 'hex');
    DELETE FROM platform.rate_limit_buckets
    WHERE subject_hmac = decode('04', 'hex');
  "

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly read identity.visitor_identities" \
  "SELECT count(*) FROM identity.visitor_identities"

expect_psql_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime unexpectedly read core.stays" \
  "SELECT count(*) FROM core.stays"

expect_psql_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime unexpectedly read core.organizations" \
  "SELECT count(*) FROM core.organizations"

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly updated platform.audit_events" \
  "UPDATE platform.audit_events SET outcome = outcome"

fallback_columns="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM information_schema.columns
      WHERE table_schema = 'public_data'
        AND table_name = 'current_methodology'
        AND column_name IN (
          'forecast_fallback_lower_percent',
          'forecast_fallback_upper_percent'
        )
    "
)"
test "${fallback_columns}" = "2"

forecast_bounds="$(
  psql_as cumuru_public cumuru-local-public-only \
    --tuples-only --no-align \
    --command="
      SET ROLE public_runtime;
      SELECT
        forecast_lower_percent
        || ':'
        || forecast_upper_percent
        || ':'
        || forecast_fallback_lower_percent
        || ':'
        || forecast_fallback_upper_percent
      FROM public_data.current_methodology
    "
)"
test "${forecast_bounds}" = "SET"

selector_kind_constraint="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM pg_catalog.pg_constraint
      WHERE conrelid = 'public_data.metric_cells'::regclass
        AND conname = 'metric_cells_presence_selector_kind_valid'
    "
)"
test "${selector_kind_constraint}" = "1"

cleanup_function_shape="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT (
        procedure.prosecdef
        AND procedure.proretset
        AND owner.rolname = 'migration_admin'
        AND NOT owner.rolcanlogin
        AND NOT has_schema_privilege(
          'migration_admin',
          'platform',
          'CREATE'
        )
        AND procedure.proconfig @> ARRAY['search_path=pg_catalog']
        AND pg_catalog.pg_get_function_result(procedure.oid)
          = 'TABLE(idempotency_records bigint, rate_limit_buckets bigint)'
      )::integer
      FROM pg_catalog.pg_proc AS procedure
      JOIN pg_catalog.pg_namespace AS namespace
        ON namespace.oid = procedure.pronamespace
      JOIN pg_catalog.pg_roles AS owner
        ON owner.oid = procedure.proowner
      WHERE namespace.nspname = 'platform'
        AND procedure.proname = 'cleanup_expired_operational_records'
        AND pg_catalog.pg_get_function_identity_arguments(procedure.oid)
          = 'cleanup_cutoff timestamp with time zone, cleanup_batch_size integer'
    "
)"
test "${cleanup_function_shape}" = "1"

cleanup_function_acl="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        has_function_privilege(
          'worker_runtime',
          'platform.cleanup_expired_operational_records(timestamptz,integer)',
          'EXECUTE'
        )::integer
        || ':'
        || has_function_privilege(
          'app_runtime',
          'platform.cleanup_expired_operational_records(timestamptz,integer)',
          'EXECUTE'
        )::integer
        || ':'
        || has_function_privilege(
          'public_runtime',
          'platform.cleanup_expired_operational_records(timestamptz,integer)',
          'EXECUTE'
        )::integer
        || ':'
        || has_function_privilege(
          'privacy_officer',
          'platform.cleanup_expired_operational_records(timestamptz,integer)',
          'EXECUTE'
        )::integer
        || ':'
        || (
          NOT EXISTS (
            SELECT 1
            FROM pg_catalog.pg_proc AS procedure,
              LATERAL pg_catalog.aclexplode(procedure.proacl) AS privilege
            WHERE procedure.oid =
              'platform.cleanup_expired_operational_records(timestamptz,integer)'
                ::regprocedure
              AND privilege.grantee = 0
              AND privilege.privilege_type = 'EXECUTE'
          )
        )::integer
    "
)"
test "${cleanup_function_acl}" = "1:0:0:0:1"

worker_cleanup_table_acl="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        has_table_privilege(
          'worker_runtime',
          'platform.idempotency_records',
          'DELETE'
        )::integer
        || ':'
        || has_column_privilege(
          'worker_runtime',
          'platform.idempotency_records',
          'expires_at',
          'SELECT'
        )::integer
        || ':'
        || has_table_privilege(
          'worker_runtime',
          'platform.rate_limit_buckets',
          'DELETE'
        )::integer
        || ':'
        || has_column_privilege(
          'worker_runtime',
          'platform.rate_limit_buckets',
          'expires_at',
          'SELECT'
        )::integer
    "
)"
test "${worker_cleanup_table_acl}" = "0:0:0:0"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly selected idempotency expiry directly" \
  "SELECT expires_at FROM platform.idempotency_records LIMIT 1"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly deleted idempotency directly" \
  "DELETE FROM platform.idempotency_records WHERE false"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly selected rate-limit expiry directly" \
  "SELECT expires_at FROM platform.rate_limit_buckets LIMIT 1"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly deleted rate-limit directly" \
  "DELETE FROM platform.rate_limit_buckets WHERE false"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
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
  response_status,
  response_headers,
  created_at,
  completed_at,
  expires_at
)
VALUES
  (
    decode('11', 'hex'),
    'actor-v1',
    'POST',
    'cleanupExpiredA',
    '00000000-0000-7000-8000-000000000061',
    decode('21', 'hex'),
    'idempotency-v1',
    decode('31', 'hex'),
    'completed',
    200,
    '{}'::jsonb,
    TIMESTAMPTZ '2026-07-01T00:00:00Z',
    TIMESTAMPTZ '2026-07-01T00:01:00Z',
    TIMESTAMPTZ '2026-07-28T11:00:00Z'
  ),
  (
    decode('12', 'hex'),
    'actor-v1',
    'POST',
    'cleanupExpiredB',
    '00000000-0000-7000-8000-000000000062',
    decode('22', 'hex'),
    'idempotency-v1',
    decode('32', 'hex'),
    'completed',
    200,
    '{}'::jsonb,
    TIMESTAMPTZ '2026-07-01T00:00:00Z',
    TIMESTAMPTZ '2026-07-01T00:01:00Z',
    TIMESTAMPTZ '2026-07-28T11:30:00Z'
  ),
  (
    decode('13', 'hex'),
    'actor-v1',
    'POST',
    'cleanupProcessing',
    '00000000-0000-7000-8000-000000000063',
    decode('23', 'hex'),
    'idempotency-v1',
    decode('33', 'hex'),
    'processing',
    NULL,
    NULL,
    TIMESTAMPTZ '2026-07-01T00:00:00Z',
    NULL,
    TIMESTAMPTZ '2026-07-28T10:00:00Z'
  ),
  (
    decode('14', 'hex'),
    'actor-v1',
    'POST',
    'cleanupAtCutoff',
    '00000000-0000-7000-8000-000000000064',
    decode('24', 'hex'),
    'idempotency-v1',
    decode('34', 'hex'),
    'completed',
    200,
    '{}'::jsonb,
    TIMESTAMPTZ '2026-07-01T00:00:00Z',
    TIMESTAMPTZ '2026-07-01T00:01:00Z',
    TIMESTAMPTZ '2026-07-28T12:00:00Z'
  ),
  (
    decode('15', 'hex'),
    'actor-v1',
    'POST',
    'cleanupFuture',
    '00000000-0000-7000-8000-000000000065',
    decode('25', 'hex'),
    'idempotency-v1',
    decode('35', 'hex'),
    'completed',
    200,
    '{}'::jsonb,
    TIMESTAMPTZ '2026-07-01T00:00:00Z',
    TIMESTAMPTZ '2026-07-01T00:01:00Z',
    TIMESTAMPTZ '2026-07-28T13:00:00Z'
  );

INSERT INTO platform.rate_limit_buckets (
  scope,
  subject_hmac,
  subject_key_version,
  window_started_at,
  request_count,
  expires_at
)
VALUES
  (
    'invite_context',
    decode('41', 'hex'),
    'rate-v1',
    TIMESTAMPTZ '2026-07-28T10:00:00Z',
    1,
    TIMESTAMPTZ '2026-07-28T11:00:00Z'
  ),
  (
    'invite_submit',
    decode('42', 'hex'),
    'rate-v1',
    TIMESTAMPTZ '2026-07-28T10:30:00Z',
    1,
    TIMESTAMPTZ '2026-07-28T11:30:00Z'
  ),
  (
    'survey_submit',
    decode('43', 'hex'),
    'rate-v1',
    TIMESTAMPTZ '2026-07-28T11:00:00Z',
    1,
    TIMESTAMPTZ '2026-07-28T12:00:00Z'
  ),
  (
    'invite_context',
    decode('44', 'hex'),
    'rate-v1',
    TIMESTAMPTZ '2026-07-28T12:00:00Z',
    1,
    TIMESTAMPTZ '2026-07-28T13:00:00Z'
  );
SQL

for invalid_cleanup_call in \
  "NULL, 1" \
  "TIMESTAMPTZ '2026-07-28T12:00:00Z', 0" \
  "TIMESTAMPTZ '2026-07-28T12:00:00Z', 1001"; do
  expect_psql_failure \
    cumuru_worker \
    cumuru-local-worker-only \
    "bounded cleanup unexpectedly accepted invalid arguments: ${invalid_cleanup_call}" \
    "SELECT * FROM platform.cleanup_expired_operational_records(
       ${invalid_cleanup_call}
     )"
done

first_cleanup_counts="$(
  psql_as cumuru_worker cumuru-local-worker-only \
    --tuples-only --no-align \
    --command="
      SELECT idempotency_records || ':' || rate_limit_buckets
      FROM platform.cleanup_expired_operational_records(
        TIMESTAMPTZ '2026-07-28T12:00:00Z',
        1
      )
    "
)"
test "${first_cleanup_counts}" = "1:1"

idempotency_after_first_cleanup="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        count(*) FILTER (
          WHERE state = 'completed'
            AND expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE state = 'processing'
            AND expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE state = 'completed'
            AND expires_at >= TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
      FROM platform.idempotency_records
      WHERE operation_key LIKE 'cleanup%'
    "
)"
test "${idempotency_after_first_cleanup}" = "1:1:2"

rate_limit_after_first_cleanup="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        count(*) FILTER (
          WHERE expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE expires_at >= TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
      FROM platform.rate_limit_buckets
    "
)"
test "${rate_limit_after_first_cleanup}" = "1:2"

second_cleanup_counts="$(
  psql_as cumuru_worker cumuru-local-worker-only \
    --tuples-only --no-align \
    --command="
      SELECT idempotency_records || ':' || rate_limit_buckets
      FROM platform.cleanup_expired_operational_records(
        TIMESTAMPTZ '2026-07-28T12:00:00Z',
        100
      )
    "
)"
test "${second_cleanup_counts}" = "1:1"

idempotency_after_second_cleanup="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        count(*) FILTER (
          WHERE state = 'completed'
            AND expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE state = 'processing'
            AND expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE state = 'completed'
            AND expires_at >= TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
      FROM platform.idempotency_records
      WHERE operation_key LIKE 'cleanup%'
    "
)"
test "${idempotency_after_second_cleanup}" = "0:1:2"

rate_limit_after_second_cleanup="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        count(*) FILTER (
          WHERE expires_at < TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
        || ':'
        || count(*) FILTER (
          WHERE expires_at >= TIMESTAMPTZ '2026-07-28T12:00:00Z'
        )
      FROM platform.rate_limit_buckets
    "
)"
test "${rate_limit_after_second_cleanup}" = "0:2"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO analytics.quality_snapshots (
  id,
  window_code,
  updated_at,
  incomplete_stays,
  overdue_planned_departures,
  silent_accommodations,
  aggregation_failures,
  suspected_duplicates_reason,
  fnrh_failures_reason
)
VALUES
  (
    '00000000-0000-7000-8000-000000000701',
    'last_30_days',
    TIMESTAMPTZ '2026-07-29T12:00:00Z',
    1,
    1,
    1,
    1,
    'pseudonym_not_approved',
    'phase_not_implemented'
  ),
  (
    '00000000-0000-7000-8000-000000000702',
    'last_30_days',
    TIMESTAMPTZ '2026-07-29T12:00:00Z',
    2,
    2,
    2,
    2,
    'pseudonym_not_approved',
    'phase_not_implemented'
  );

INSERT INTO analytics.quality_coverage (
  quality_snapshot_id,
  category_code,
  status,
  ratio
)
VALUES
  (
    '00000000-0000-7000-8000-000000000701',
    'older_snapshot',
    'available',
    0.1
  ),
  (
    '00000000-0000-7000-8000-000000000702',
    'newer_snapshot',
    'available',
    0.2
  );
SQL

quality_tie_selection="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        count(*)
        || ':'
        || min(incomplete_stays)
        || ':'
        || min(category_code)
      FROM analytics.current_quality
      WHERE window_code = 'last_30_days'
    "
)"
test "${quality_tie_selection}" = "1:2:newer_snapshot"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="
    INSERT INTO public_data.publications (
      publication_version,
      build_fingerprint,
      as_of_on,
      data_mode,
      privacy_policy_version,
      methodology_version,
      coverage_status,
      published_at
    ) VALUES (
      170,
      repeat('1', 64),
      DATE '2026-07-28',
      'prototype_fixtures',
      'prototype-v1',
      'explainable-baseline-v1',
      'protected',
      TIMESTAMPTZ '2026-07-28T12:00:00Z'
    )
  "

expect_psql_failure \
  cumuru_migration \
  cumuru-local-migration-only \
  "recent presence cell unexpectedly accepted forecast kind" \
  "INSERT INTO public_data.metric_cells (
     publication_version,
     cell_key,
     metric_code,
     period_selector,
     period_start,
     period_end,
     unit,
     dimension_code,
     category_code,
     kind,
     status
   ) VALUES (
     170,
     repeat('2', 64),
     'presence',
     'recent_30_days',
     DATE '2026-07-28',
     DATE '2026-07-29',
     'person_day',
     'none',
     'none',
     'forecast',
     'protected'
   )"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="DELETE FROM public_data.publications WHERE publication_version = 170"

phase4_public_views="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM information_schema.views
      WHERE table_schema = 'public_data'
        AND table_name IN (
          'current_summary',
          'current_presence',
          'current_preferences',
          'current_methodology'
        )
    "
)"
test "${phase4_public_views}" = "4"

phase4_public_role="$(
  psql_as cumuru_public cumuru-local-public-only \
    --tuples-only --no-align \
    --command='SET ROLE public_runtime; SELECT current_user'
)"
test "${phase4_public_role}" = "SET"$'\n'"public_runtime"

for public_view in \
  current_summary \
  current_presence \
  current_preferences \
  current_methodology; do
  psql_as cumuru_public cumuru-local-public-only \
    --command="SET ROLE public_runtime;
      SELECT count(*) FROM public_data.${public_view}"
done

expect_psql_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime unexpectedly read public_data.publications" \
  "SET ROLE public_runtime; SELECT count(*) FROM public_data.publications"

expect_psql_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime unexpectedly read analytics.presence_days" \
  "SET ROLE public_runtime; SELECT count(*) FROM analytics.presence_days"

psql_as cumuru_app cumuru-local-app-only \
  --command="SELECT count(*) FROM analytics.current_quality"

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly read analytics.quality_snapshots" \
  "SELECT count(*) FROM analytics.quality_snapshots"

worker_structured_access="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT
        has_column_privilege(
          'worker_runtime',
          'survey.answers',
          'structured_value',
          'SELECT'
        )::integer
        || ':'
        || has_column_privilege(
          'worker_runtime',
          'survey.answers',
          'encrypted_free_text',
          'SELECT'
        )::integer
    "
)"
test "${worker_structured_access}" = "0:0"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly read structured survey answers directly" \
  "SELECT structured_value FROM survey.answers LIMIT 1"

expect_psql_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker_runtime unexpectedly read encrypted survey text" \
  "SELECT encrypted_free_text FROM survey.answers LIMIT 1"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="CREATE TABLE public_data.phase4_default_privilege_canary (
    canary integer
  )"

expect_psql_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime inherited SELECT on a future public_data table" \
  "SET ROLE public_runtime;
   SELECT count(*) FROM public_data.phase4_default_privilege_canary"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="DROP TABLE public_data.phase4_default_privilege_canary"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="
    DELETE FROM platform.rate_limit_buckets
    WHERE subject_hmac IN (
      decode('41', 'hex'),
      decode('42', 'hex'),
      decode('43', 'hex'),
      decode('44', 'hex')
    );
    DELETE FROM platform.idempotency_records
    WHERE operation_key LIKE 'cleanupExpired%';
  "

run_migrate down 1
schemas_left="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM information_schema.schemata
      WHERE schema_name IN (
        'identity',
        'core',
        'survey',
        'analytics',
        'public_data',
        'platform'
      )
    "
)"
test "${schemas_left}" = "0"

run_migrate up
final_version="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="SELECT version || ':' || dirty FROM public.schema_migrations"
)"
test "${final_version}" = "1:false"

echo "consolidated migration 001 zero-to-one-to-zero-to-one cycle, deterministic quality, grants, bounded cleanup and fictitious tenant isolation passed"
