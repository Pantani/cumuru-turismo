#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-phase3-integration-${PPID}-$$"
CAPABILITY_CANARY="PHASE3_CAPABILITY_CANARY_DO_NOT_LOG_7F4C"
FREE_TEXT_CANARY="PHASE3_FREE_TEXT_CANARY_DO_NOT_LOG_91E2"
PROMPT_CANARY="PHASE3_PROMPT_CANARY_DO_NOT_LOG_B63A"
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.phase3-integration.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans
  cleanup_status=$?
  residual_containers="$(
    docker container ls --all --quiet \
      --filter "label=com.docker.compose.project=${PROJECT_NAME}"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual_containers}"; then
    echo "phase 3 integration cleanup left containers or could not inspect" >&2
    cleanup_status=1
  fi
  residual_networks="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual_networks}"; then
    echo "phase 3 integration cleanup left its network or could not inspect" >&2
    cleanup_status=1
  fi
  residual_volumes="$(
    docker volume ls --quiet --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual_volumes}"; then
    echo "phase 3 integration cleanup left its volume or could not inspect" >&2
    cleanup_status=1
  fi
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  exit "${cleanup_status}"
}
trap cleanup EXIT

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
  if psql_as "${user}" "${password}" \
    --command="${statement}" >/dev/null 2>&1; then
    echo "${description}" >&2
    exit 1
  fi
}

assert_denied_matrix() {
  local connection_user="$1"
  local password="$2"
  local probe_role="$3"
  psql_as "${connection_user}" "${password}" \
    --set=probe_role="${probe_role}" <<'SQL'
SET ROLE :"probe_role";
DO $phase3_denied_operations$
DECLARE
  operation text;
  statement text;
  target_table text;
BEGIN
  FOREACH target_table IN ARRAY ARRAY[
    'responses',
    'answers',
    'consent_decisions'
  ]
  LOOP
    FOREACH operation IN ARRAY ARRAY[
      'SELECT',
      'INSERT',
      'UPDATE',
      'DELETE'
    ]
    LOOP
      IF current_user = 'cumuru_app' AND operation = 'INSERT' THEN
        CONTINUE;
      END IF;

      statement := CASE operation
        WHEN 'SELECT' THEN
          format('SELECT 1 FROM survey.%I LIMIT 1', target_table)
        WHEN 'INSERT' THEN
          format('INSERT INTO survey.%I DEFAULT VALUES', target_table)
        WHEN 'UPDATE' THEN
          format(
            'UPDATE survey.%I SET id = id WHERE false',
            target_table
          )
        WHEN 'DELETE' THEN
          format('DELETE FROM survey.%I WHERE false', target_table)
      END;

      BEGIN
        EXECUTE statement;
        RAISE EXCEPTION
          'unexpected grant: role=%, operation=%, table=survey.%',
          current_user,
          operation,
          target_table
          USING ERRCODE = '23514';
      EXCEPTION
        WHEN insufficient_privilege THEN
          NULL;
      END;
    END LOOP;
  END LOOP;
END
$phase3_denied_operations$;
SQL
}

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
DO $phase3_grants$
DECLARE
  mismatch record;
BEGIN
  FOR mismatch IN
    WITH roles(role_name) AS (
      VALUES
        ('cumuru_app'),
        ('worker_runtime'),
        ('public_runtime'),
        ('privacy_officer')
    ),
    tables(table_name) AS (
      VALUES
        ('survey.responses'),
        ('survey.answers'),
        ('survey.consent_decisions')
    ),
    privileges(privilege_name) AS (
      VALUES ('SELECT'), ('INSERT'), ('UPDATE'), ('DELETE')
    ),
    expected AS (
      SELECT
        role_name,
        table_name,
        privilege_name,
        role_name = 'cumuru_app' AND privilege_name = 'INSERT'
          AS should_have_privilege
      FROM roles
      CROSS JOIN tables
      CROSS JOIN privileges
    )
    SELECT
      role_name,
      table_name,
      privilege_name,
      should_have_privilege,
      has_table_privilege(
        role_name::name,
        table_name,
        privilege_name
      ) AS has_privilege
    FROM expected
    WHERE has_table_privilege(
      role_name::name,
      table_name,
      privilege_name
    ) IS DISTINCT FROM should_have_privilege
  LOOP
    RAISE EXCEPTION
      'unexpected grant: role=%, table=%, privilege=%, expected=%, actual=%',
      mismatch.role_name,
      mismatch.table_name,
      mismatch.privilege_name,
      mismatch.should_have_privilege,
      mismatch.has_privilege;
  END LOOP;

  IF EXISTS (
    SELECT 1
    FROM pg_catalog.pg_class AS relation
    JOIN pg_catalog.pg_namespace AS namespace
      ON namespace.oid = relation.relnamespace
    CROSS JOIN LATERAL pg_catalog.aclexplode(
      COALESCE(
        relation.relacl,
        pg_catalog.acldefault('r', relation.relowner)
      )
    ) AS privilege
    WHERE namespace.nspname = 'survey'
      AND relation.relname IN (
        'responses',
        'answers',
        'consent_decisions'
      )
      AND privilege.grantee = 0
      AND privilege.privilege_type IN (
        'SELECT',
        'INSERT',
        'UPDATE',
        'DELETE'
      )
  ) THEN
    RAISE EXCEPTION
      'PUBLIC unexpectedly has a survey response table privilege';
  END IF;
END
$phase3_grants$;
SQL

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO core.organizations (id, name)
VALUES ('00000000-0000-7000-8000-000000003001', 'Organização Fictícia F3');

INSERT INTO core.accommodations (id, organization_id, name, category, status)
VALUES (
  '00000000-0000-7000-8000-000000003002',
  '00000000-0000-7000-8000-000000003001',
  'Hospedagem Fictícia F3',
  'prototype',
  'active'
);

INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
) VALUES (
  '00000000-0000-7000-8000-000000003003',
  '00000000-0000-7000-8000-000000003002',
  'https://oidc.invalid/local',
  'phase3-editor',
  'manager'
);

INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count
) VALUES (
  '00000000-0000-7000-8000-000000003004',
  '00000000-0000-7000-8000-000000003002',
  '00000000-0000-7000-8000-000000003003',
  'pre_registered',
  '00000000-0000-7000-8000-000000003005',
  DATE '2026-12-10',
  DATE '2026-12-12',
  2
);
SQL

psql_as cumuru_app cumuru-local-app-only \
  --set=capability_canary="${CAPABILITY_CANARY}" \
  --set=free_text_canary="${FREE_TEXT_CANARY}" \
  --set=prompt_canary="${PROMPT_CANARY}" <<'SQL'
INSERT INTO survey.questionnaires (id, stable_key, name)
VALUES (
  '00000000-0000-7000-8000-000000003010',
  'tourism_profile',
  'Perfil turístico fictício'
);

INSERT INTO survey.questionnaire_versions (
  id,
  questionnaire_id,
  version_number,
  title,
  privacy_notice_version,
  last_editor_hmac,
  last_editor_key_version
) VALUES (
  '00000000-0000-7000-8000-000000003011',
  '00000000-0000-7000-8000-000000003010',
  1,
  'Pesquisa fictícia',
  'notice-v1',
  decode('0101', 'hex'),
  'actor-v1'
);

INSERT INTO survey.questions (
  id,
  questionnaire_version_id,
  stable_key,
  prompt,
  answer_type,
  required,
  data_classification,
  purpose_code,
  retention_policy_code,
  public_aggregation_allowed,
  display_order
) VALUES (
  '00000000-0000-7000-8000-000000003012',
  '00000000-0000-7000-8000-000000003011',
  'expectation',
  :'prompt_canary',
  'short_text',
  false,
  'personal',
  'tourism_research',
  'survey_prototype_v1',
  false,
  1
);

INSERT INTO survey.consent_requirements (
  questionnaire_version_id,
  purpose_code,
  notice_version,
  prompt,
  required_for_answers,
  display_order
) VALUES (
  '00000000-0000-7000-8000-000000003011',
  'tourism_research',
  'notice-v1',
  'Aceite fictício',
  true,
  1
);

UPDATE survey.questionnaire_versions
SET
  status = 'privacy_review',
  submitted_by_hmac = decode('0101', 'hex'),
  submitted_by_key_version = 'actor-v1',
  submitted_for_review_at = now(),
  revision = revision + 1,
  updated_at = now()
WHERE id = '00000000-0000-7000-8000-000000003011';

UPDATE survey.questionnaire_versions
SET
  status = 'approved',
  reviewed_by_hmac = decode('0202', 'hex'),
  reviewed_by_key_version = 'actor-v1',
  privacy_reviewed_at = now(),
  revision = revision + 1,
  updated_at = now()
WHERE id = '00000000-0000-7000-8000-000000003011';

UPDATE survey.questionnaire_versions
SET
  status = 'published',
  published_at = now(),
  revision = revision + 1,
  updated_at = now()
WHERE id = '00000000-0000-7000-8000-000000003011';

INSERT INTO survey.capabilities (
  id,
  token_hmac,
  token_key_version,
  purpose,
  stay_id,
  questionnaire_version_id,
  expires_at
) VALUES (
  '00000000-0000-7000-8000-000000003020',
  decode(md5(:'capability_canary'), 'hex'),
  'survey-v1',
  'survey_response',
  '00000000-0000-7000-8000-000000003004',
  '00000000-0000-7000-8000-000000003011',
  now() + interval '24 hours'
);

BEGIN;
INSERT INTO survey.responses (
  id,
  stay_id,
  questionnaire_version_id,
  capability_id,
  client_submission_id,
  participation
) VALUES (
  '00000000-0000-7000-8000-000000003021',
  '00000000-0000-7000-8000-000000003004',
  '00000000-0000-7000-8000-000000003011',
  '00000000-0000-7000-8000-000000003020',
  '00000000-0000-7000-8000-000000003022',
  'submitted'
);

INSERT INTO survey.consent_decisions (
  id,
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted,
  collection_channel
) VALUES (
  '00000000-0000-7000-8000-000000003023',
  '00000000-0000-7000-8000-000000003021',
  '00000000-0000-7000-8000-000000003011',
  'tourism_research',
  'notice-v1',
  true,
  'survey_capability'
);

INSERT INTO survey.answers (
  id,
  response_id,
  questionnaire_version_id,
  question_id,
  encrypted_free_text,
  free_text_nonce,
  encryption_key_version,
  erase_after,
  created_at
) VALUES (
  '00000000-0000-7000-8000-000000003024',
  '00000000-0000-7000-8000-000000003021',
  '00000000-0000-7000-8000-000000003011',
  '00000000-0000-7000-8000-000000003012',
  decode(
    md5(:'free_text_canary') || md5(:'free_text_canary' || '-2'),
    'hex'
  ),
  decode(repeat('05', 12), 'hex'),
  'free-text-v1',
  now() - interval '1 hour',
  now() - interval '2 hours'
);
COMMIT;
SQL

expect_psql_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app_runtime unexpectedly changed published survey content" \
  "UPDATE survey.questions SET prompt = 'mutated'
   WHERE id = '00000000-0000-7000-8000-000000003012'"

assert_denied_matrix \
  cumuru_app \
  cumuru-local-app-only \
  cumuru_app
assert_denied_matrix \
  cumuru_worker \
  cumuru-local-worker-only \
  worker_runtime
assert_denied_matrix \
  cumuru_public \
  cumuru-local-public-only \
  public_runtime

assert_denied_matrix \
  postgres \
  cumuru-local-admin-only \
  privacy_officer

psql_as postgres cumuru-local-admin-only \
  --command='CREATE ROLE phase3_public_probe NOLOGIN'
assert_denied_matrix \
  postgres \
  cumuru-local-admin-only \
  phase3_public_probe
psql_as postgres cumuru-local-admin-only \
  --command='DROP ROLE phase3_public_probe'

erased_count="$(
  psql_as cumuru_worker cumuru-local-worker-only \
    --tuples-only --no-align \
    --command='SELECT survey.erase_expired_free_text(now())'
)"
test "${erased_count}" = "1"

remaining_ciphertext="$(
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align \
    --command="
      SELECT count(*)
      FROM survey.answers
      WHERE encrypted_free_text IS NOT NULL
    "
)"
test "${remaining_ciphertext}" = "0"

if ! postgres_sink_logs="$(
  "${COMPOSE[@]}" logs --no-color postgres 2>&1
)"; then
  echo "could not inspect the phase 3 PostgreSQL log sink" >&2
  exit 1
fi
for canary in \
  "${CAPABILITY_CANARY}" \
  "${FREE_TEXT_CANARY}" \
  "${PROMPT_CANARY}"; do
  if grep --fixed-strings --quiet "${canary}" <<<"${postgres_sink_logs}"; then
    echo "phase 3 canary reached the PostgreSQL log sink" >&2
    exit 1
  fi
done

published_address="$("${COMPOSE[@]}" port postgres 5432)"
database_port="${published_address##*:}"
case "${database_port}" in
  ""|*[!0-9]*)
    echo "could not resolve the ephemeral PostgreSQL port" >&2
    exit 1
    ;;
esac

admin_dsn="postgres://cumuru_migration:cumuru-local-migration-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"
runtime_dsn="postgres://cumuru_app:cumuru-local-app-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"

CUMURU_TEST_ADMIN_DATABASE_URL="${admin_dsn}" \
CUMURU_TEST_DATABASE_URL="${runtime_dsn}" \
  go -C "${ROOT_DIR}/apps/api" test \
    -tags=integration -race -count=1 ./internal/platform/store

echo "phase 3 PostgreSQL integration passed with consolidated migration baseline"
