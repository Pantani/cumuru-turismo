#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-restore-drill-${PPID}-$$"
SOURCE_DATABASE="cumuru"
RESTORE_DATABASE="cumuru_restore_drill"
DUMP_PATH="/tmp/cumuru-restore-drill.dump"
ADMIN_PASSWORD="cumuru-local-admin-only"
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.migration-test.yaml"
  --project-name "${PROJECT_NAME}"
)
MIGRATION_URL="postgres://cumuru_migration:cumuru-local-migration-only@postgres:5432/${SOURCE_DATABASE}?sslmode=disable"

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  trap - EXIT
  set +e
  psql_as "${SOURCE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
    --command="DROP DATABASE IF EXISTS ${RESTORE_DATABASE} WITH (FORCE)" \
    >/dev/null 2>&1
  "${COMPOSE[@]}" exec -T postgres rm -f -- "${DUMP_PATH}" \
    >/dev/null 2>&1
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1
  cleanup_status=$?
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  exit "${cleanup_status}"
}
trap cleanup EXIT

psql_as() {
  local database="$1"
  local user="$2"
  local password="$3"
  shift 3
  "${COMPOSE[@]}" exec -T \
    -e "PGPASSWORD=${password}" \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username="${user}" --dbname="${database}" "$@"
}

expect_psql_failure() {
  local database="$1"
  local user="$2"
  local password="$3"
  local description="$4"
  local statement="$5"
  if psql_as "${database}" "${user}" "${password}" \
    --command="${statement}" >/dev/null 2>&1; then
    echo "${description}" >&2
    exit 1
  fi
}

database_fingerprint() {
  local database="$1"
  psql_as "${database}" postgres "${ADMIN_PASSWORD}" \
    --tuples-only --no-align \
    --command="
      SELECT concat_ws(
        ':',
        (SELECT version || ':' || dirty FROM public.schema_migrations),
        (
          SELECT count(*)
          FROM core.organizations
          WHERE id = '00000000-0000-7000-8000-000000000601'
            AND name = 'Organização Sintética de Restore'
        ),
        (
          SELECT count(*)
          FROM core.accommodations
          WHERE id = '00000000-0000-7000-8000-000000000602'
            AND organization_id = '00000000-0000-7000-8000-000000000601'
            AND name = 'Hospedagem Sintética de Restore'
            AND category = 'other'
        ),
        (
          SELECT count(*)
          FROM core.memberships
          WHERE id = '00000000-0000-7000-8000-000000000603'
            AND accommodation_id = '00000000-0000-7000-8000-000000000602'
            AND oidc_issuer = 'https://oidc.invalid/restore-drill'
            AND oidc_subject = 'synthetic-operator'
        ),
        (
          SELECT count(*)
          FROM platform.audit_events
          WHERE id = '00000000-0000-7000-8000-000000000604'
            AND action = 'restore_drill.synthetic_fixture'
            AND metadata = '{}'::jsonb
        ),
        (
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
        ),
        (
          SELECT count(*)
          FROM information_schema.views
          WHERE table_schema = 'public_data'
            AND table_name IN (
              'current_summary',
              'current_presence',
              'current_preferences',
              'current_methodology'
            )
        )
      )
    "
}

"${COMPOSE[@]}" up --detach --wait postgres

"${COMPOSE[@]}" run --rm --no-deps migrate \
  -path=/migrations -database="${MIGRATION_URL}" up

psql_as "${SOURCE_DATABASE}" cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO core.organizations (id, name)
VALUES (
  '00000000-0000-7000-8000-000000000601',
  'Organização Sintética de Restore'
);

INSERT INTO core.accommodations (
  id,
  organization_id,
  name,
  category,
  status
)
VALUES (
  '00000000-0000-7000-8000-000000000602',
  '00000000-0000-7000-8000-000000000601',
  'Hospedagem Sintética de Restore',
  'other',
  'active'
);

INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  '00000000-0000-7000-8000-000000000603',
  '00000000-0000-7000-8000-000000000602',
  'https://oidc.invalid/restore-drill',
  'synthetic-operator',
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
  '00000000-0000-7000-8000-000000000604',
  decode('06', 'hex'),
  'restore-drill-v1',
  'system',
  'restore_drill.synthetic_fixture',
  'database',
  'success'
);
SQL

source_fingerprint="$(database_fingerprint "${SOURCE_DATABASE}")"
test "${source_fingerprint}" = "1:false:1:1:1:1:6:4"

"${COMPOSE[@]}" exec -T \
  -e "PGPASSWORD=${ADMIN_PASSWORD}" \
  postgres pg_dump \
  --username=postgres \
  --dbname="${SOURCE_DATABASE}" \
  --format=custom \
  --compress=9 \
  --file="${DUMP_PATH}"

"${COMPOSE[@]}" exec -T postgres \
  test -s "${DUMP_PATH}"

psql_as "${SOURCE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
  --command="CREATE DATABASE ${RESTORE_DATABASE} TEMPLATE template0"

psql_as "${RESTORE_DATABASE}" postgres "${ADMIN_PASSWORD}" <<'SQL'
GRANT CONNECT, CREATE, TEMPORARY
ON DATABASE cumuru_restore_drill
TO migration_admin;

GRANT CONNECT
ON DATABASE cumuru_restore_drill
TO app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE CREATE, TEMPORARY
ON DATABASE cumuru_restore_drill
FROM PUBLIC, public_runtime, cumuru_public;
SQL

"${COMPOSE[@]}" exec -T \
  -e "PGPASSWORD=${ADMIN_PASSWORD}" \
  postgres pg_restore \
  --username=postgres \
  --dbname="${RESTORE_DATABASE}" \
  --exit-on-error \
  "${DUMP_PATH}"

restore_fingerprint="$(database_fingerprint "${RESTORE_DATABASE}")"
test "${restore_fingerprint}" = "${source_fingerprint}"

schema_owners="$(
  psql_as "${RESTORE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM pg_catalog.pg_namespace AS namespace
      JOIN pg_catalog.pg_roles AS owner
        ON owner.oid = namespace.nspowner
      WHERE namespace.nspname IN (
          'identity',
          'core',
          'survey',
          'analytics',
          'public_data',
          'platform'
        )
        AND owner.rolname = 'cumuru_migration'
    "
)"
test "${schema_owners}" = "6"

function_owners="$(
  psql_as "${RESTORE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM pg_catalog.pg_proc AS function
      JOIN pg_catalog.pg_namespace AS namespace
        ON namespace.oid = function.pronamespace
      JOIN pg_catalog.pg_roles AS owner
        ON owner.oid = function.proowner
      WHERE (
          (
            namespace.nspname = 'analytics'
            AND function.proname = 'aggregate_eligible_preferences'
          )
          OR (
            namespace.nspname = 'platform'
            AND function.proname = 'cleanup_expired_operational_records'
          )
        )
        AND owner.rolname = 'migration_admin'
    "
)"
test "${function_owners}" = "2"

public_view_privileges="$(
  psql_as "${RESTORE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
    --tuples-only --no-align \
    --command="
      SELECT concat_ws(
        ':',
        has_table_privilege(
          'public_runtime',
          'public_data.current_summary',
          'SELECT'
        )::integer,
        has_table_privilege(
          'public_runtime',
          'public_data.current_presence',
          'SELECT'
        )::integer,
        has_table_privilege(
          'public_runtime',
          'public_data.current_preferences',
          'SELECT'
        )::integer,
        has_table_privilege(
          'public_runtime',
          'public_data.current_methodology',
          'SELECT'
        )::integer,
        has_database_privilege(
          'public_runtime',
          current_database(),
          'CREATE'
        )::integer,
        has_database_privilege(
          'public_runtime',
          current_database(),
          'TEMPORARY'
        )::integer
      )
    "
)"
test "${public_view_privileges}" = "1:1:1:1:0:0"

psql_as "${RESTORE_DATABASE}" cumuru_public cumuru-local-public-only \
  --command="SELECT count(*) FROM public_data.current_summary" \
  >/dev/null

expect_psql_failure \
  "${RESTORE_DATABASE}" \
  cumuru_public \
  cumuru-local-public-only \
  "public_runtime unexpectedly read a transactional table after restore" \
  "SELECT count(*) FROM core.organizations"

expect_psql_failure \
  "${RESTORE_DATABASE}" \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly read platform audit history after restore" \
  "SELECT count(*) FROM platform.audit_events"

future_credentials_absent="$(
  psql_as "${RESTORE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
    --tuples-only --no-align \
    --command="
      SELECT (to_regclass('identity.external_credentials') IS NULL)::integer
    "
)"
test "${future_credentials_absent}" = "1"

psql_as "${SOURCE_DATABASE}" postgres "${ADMIN_PASSWORD}" \
  --command="DROP DATABASE ${RESTORE_DATABASE} WITH (FORCE)"

"${COMPOSE[@]}" exec -T postgres rm -f -- "${DUMP_PATH}"

echo "LOCAL_RESTORE_DRILL=PASS"
