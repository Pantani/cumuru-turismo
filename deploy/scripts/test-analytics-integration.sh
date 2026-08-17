#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-analytics-integration-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet analytics-integration
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.analytics-integration.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  local inspect_status=0
  local residual=""
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans
  cleanup_status=$?
  residual="$(
    docker container ls --all --quiet \
      --filter "label=com.docker.compose.project=${PROJECT_NAME}"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "analytics integration left containers behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "analytics integration left its network behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker volume ls --quiet --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "analytics integration left its volume behind" >&2
    cleanup_status=1
  fi
  set -e
  # O lock só cai depois do teardown: é isso que impede a execução seguinte de
  # pedir um endereço que ainda está sendo liberado.
  cumuru_release_subnet
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    exit "${cleanup_status}"
  fi
  echo "analytics integration cleanup confirmed"
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

expect_failure() {
  local user="$1"
  local password="$2"
  local message="$3"
  local statement="$4"
  if psql_as "${user}" "${password}" \
    --command="${statement}" >/dev/null 2>&1; then
    echo "${message}" >&2
    exit 1
  fi
}

expect_no_rows() {
  local user="$1"
  local password="$2"
  local message="$3"
  local statement="$4"
  local result=""
  result="$(scalar_as "${user}" "${password}" "${statement}")"
  if test -n "${result}"; then
    echo "${message}" >&2
    exit 1
  fi
}

scalar_as() {
  local user="$1"
  local password="$2"
  local statement="$3"
  psql_as "${user}" "${password}" \
    --quiet --tuples-only --no-align \
    --command="${statement}"
}

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
BEGIN;

INSERT INTO core.organizations (id, name)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000100',
  'private-organization-canary-f4'
);

INSERT INTO core.accommodations (
  id,
  organization_id,
  name,
  category,
  status,
  capacity
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000101',
    '0197f4cf-2a69-7000-8000-000000000100',
    'private-accommodation-canary-f4-a',
    'formal_lodging',
    'active',
    10
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000102',
    '0197f4cf-2a69-7000-8000-000000000100',
    'private-accommodation-canary-f4-b',
    'formal_lodging',
    'active',
    20
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000103',
    '0197f4cf-2a69-7000-8000-000000000100',
    'private-accommodation-canary-f4-c',
    'formal_lodging',
    'active',
    30
  );

INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000104',
  '0197f4cf-2a69-7000-8000-000000000101',
  'https://identity.invalid/analytics',
  'private-operator-canary-f4',
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
  expected_guest_count,
  version
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000105',
  '0197f4cf-2a69-7000-8000-000000000101',
  '0197f4cf-2a69-7000-8000-000000000104',
  'pre_registered',
  '0197f4cf-2a69-7000-8000-000000000106',
  DATE '2026-07-28',
  DATE '2026-07-30',
  1,
  1
);

INSERT INTO core.visitors (
  id,
  stay_id,
  client_id,
  role,
  age_band,
  residence_country
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000107',
  '0197f4cf-2a69-7000-8000-000000000105',
  '0197f4cf-2a69-7000-8000-000000000108',
  'responsible',
  '25_34',
  'BR'
);

INSERT INTO analytics.presence_days (
  stay_id,
  visitor_id,
  presence_on,
  kind,
  weight,
  source_stay_version,
  as_of_on,
  updated_at
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000105',
  '0197f4cf-2a69-7000-8000-000000000107',
  DATE '2026-07-28',
  'forecast',
  0.8,
  1,
  DATE '2026-07-28',
  TIMESTAMPTZ '2026-07-28T10:00:00Z'
);

INSERT INTO survey.questionnaires (id, stable_key, name)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000120',
  'analytics_preference_window',
  'Analytics preference window fixture'
);

INSERT INTO survey.questionnaire_versions (
  id,
  questionnaire_id,
  version_number,
  title,
  privacy_notice_version,
  last_editor_hmac,
  last_editor_key_version
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000121',
  '0197f4cf-2a69-7000-8000-000000000120',
  1,
  'Analytics preference window fixture',
  'prototype-v1',
  decode(repeat('1', 64), 'hex'),
  'local-v1'
);

INSERT INTO survey.questions (
  id,
  questionnaire_version_id,
  stable_key,
  prompt,
  answer_type,
  data_classification,
  purpose_code,
  retention_policy_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell,
  display_order
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000122',
    '0197f4cf-2a69-7000-8000-000000000121',
    'visit_profile',
    'Perfil da visita',
    'single_choice',
    'operational',
    'tourism_analytics',
    'prototype-aggregate-only',
    'first_visit_share',
    true,
    10,
    1
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000151',
    '0197f4cf-2a69-7000-8000-000000000121',
    'returning_profile',
    'Perfil de retorno',
    'single_choice',
    'operational',
    'tourism_analytics',
    'prototype-aggregate-only',
    'first_visit_share',
    true,
    20,
    2
  );

INSERT INTO survey.consent_requirements (
  questionnaire_version_id,
  purpose_code,
  notice_version,
  prompt,
  required_for_answers,
  display_order
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000121',
  'tourism_analytics',
  'prototype-v1',
  'Consentimento para estatística agregada',
  true,
  1
);

INSERT INTO analytics.metric_mappings (
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
)
VALUES
  (
    'prototype-v1',
    'first_visit_share',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000122',
    'first_visit',
    'first_visit'
  ),
  (
    'prototype-v1',
    'first_visit_share',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000151',
    'returning',
    'returning'
  );

INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  version
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000123',
    '0197f4cf-2a69-7000-8000-000000000101',
    '0197f4cf-2a69-7000-8000-000000000104',
    'pre_registered',
    '0197f4cf-2a69-7000-8000-000000000143',
    DATE '2026-06-01',
    DATE '2026-06-02',
    1,
    1
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000124',
    '0197f4cf-2a69-7000-8000-000000000101',
    '0197f4cf-2a69-7000-8000-000000000104',
    'pre_registered',
    '0197f4cf-2a69-7000-8000-000000000144',
    DATE '2026-06-01',
    DATE '2026-06-02',
    1,
    1
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000125',
    '0197f4cf-2a69-7000-8000-000000000101',
    '0197f4cf-2a69-7000-8000-000000000104',
    'pre_registered',
    '0197f4cf-2a69-7000-8000-000000000145',
    DATE '2026-06-30',
    DATE '2026-07-01',
    1,
    1
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000126',
    '0197f4cf-2a69-7000-8000-000000000101',
    '0197f4cf-2a69-7000-8000-000000000104',
    'pre_registered',
    '0197f4cf-2a69-7000-8000-000000000146',
    DATE '2026-07-01',
    DATE '2026-07-02',
    1,
    1
  );

INSERT INTO core.visitors (
  id,
  stay_id,
  client_id,
  role,
  age_band,
  residence_country
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000159',
  '0197f4cf-2a69-7000-8000-000000000123',
  '0197f4cf-2a69-7000-8000-000000000162',
  'responsible',
  '25_34',
  'BR'
);

INSERT INTO analytics.presence_days (
  stay_id,
  visitor_id,
  presence_on,
  kind,
  weight,
  source_stay_version,
  as_of_on,
  updated_at
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000123',
    '0197f4cf-2a69-7000-8000-000000000159',
    DATE '2026-06-01',
    'forecast',
    0.8,
    1,
    DATE '2026-07-28',
    TIMESTAMPTZ '2026-07-28T10:01:00Z'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000105',
    '0197f4cf-2a69-7000-8000-000000000107',
    DATE '2026-07-29',
    'forecast',
    0.8,
    2,
    DATE '2026-07-28',
    TIMESTAMPTZ '2026-07-28T10:02:00Z'
  );

INSERT INTO survey.capabilities (
  id,
  token_hmac,
  token_key_version,
  purpose,
  stay_id,
  questionnaire_version_id,
  expires_at
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000127',
    decode(repeat('2', 64), 'hex'),
    'local-v1',
    'survey_response',
    '0197f4cf-2a69-7000-8000-000000000123',
    '0197f4cf-2a69-7000-8000-000000000121',
    TIMESTAMPTZ '2027-01-01T00:00:00Z'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000128',
    decode(repeat('3', 64), 'hex'),
    'local-v1',
    'survey_response',
    '0197f4cf-2a69-7000-8000-000000000124',
    '0197f4cf-2a69-7000-8000-000000000121',
    TIMESTAMPTZ '2027-01-01T00:00:00Z'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000129',
    decode(repeat('4', 64), 'hex'),
    'local-v1',
    'survey_response',
    '0197f4cf-2a69-7000-8000-000000000125',
    '0197f4cf-2a69-7000-8000-000000000121',
    TIMESTAMPTZ '2027-01-01T00:00:00Z'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000130',
    decode(repeat('5', 64), 'hex'),
    'local-v1',
    'survey_response',
    '0197f4cf-2a69-7000-8000-000000000126',
    '0197f4cf-2a69-7000-8000-000000000121',
    TIMESTAMPTZ '2027-01-01T00:00:00Z'
  );

INSERT INTO survey.responses (
  id,
  stay_id,
  questionnaire_version_id,
  capability_id,
  client_submission_id,
  participation,
  submitted_at
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000131',
    '0197f4cf-2a69-7000-8000-000000000123',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000127',
    '0197f4cf-2a69-7000-8000-000000000147',
    'submitted',
    TIMESTAMPTZ '2026-05-31T23:59:59.999999-03:00'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000132',
    '0197f4cf-2a69-7000-8000-000000000124',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000128',
    '0197f4cf-2a69-7000-8000-000000000148',
    'submitted',
    TIMESTAMPTZ '2026-06-01T00:00:00-03:00'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000133',
    '0197f4cf-2a69-7000-8000-000000000125',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000129',
    '0197f4cf-2a69-7000-8000-000000000149',
    'submitted',
    TIMESTAMPTZ '2026-06-30T23:59:59.999999-03:00'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000134',
    '0197f4cf-2a69-7000-8000-000000000126',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000130',
    '0197f4cf-2a69-7000-8000-000000000150',
    'submitted',
    TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
  );

INSERT INTO survey.answers (
  id,
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000135',
    '0197f4cf-2a69-7000-8000-000000000131',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000122',
    '"first_visit"'::jsonb
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000136',
    '0197f4cf-2a69-7000-8000-000000000132',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000122',
    '"first_visit"'::jsonb
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000137',
    '0197f4cf-2a69-7000-8000-000000000133',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000122',
    '"first_visit"'::jsonb
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000138',
    '0197f4cf-2a69-7000-8000-000000000134',
    '0197f4cf-2a69-7000-8000-000000000121',
    '0197f4cf-2a69-7000-8000-000000000122',
    '"first_visit"'::jsonb
  );

INSERT INTO survey.consent_decisions (
  id,
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted,
  collection_channel
)
VALUES
  (
    '0197f4cf-2a69-7000-8000-000000000139',
    '0197f4cf-2a69-7000-8000-000000000131',
    '0197f4cf-2a69-7000-8000-000000000121',
    'tourism_analytics',
    'prototype-v1',
    true,
    'survey_capability'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000140',
    '0197f4cf-2a69-7000-8000-000000000132',
    '0197f4cf-2a69-7000-8000-000000000121',
    'tourism_analytics',
    'prototype-v1',
    true,
    'survey_capability'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000141',
    '0197f4cf-2a69-7000-8000-000000000133',
    '0197f4cf-2a69-7000-8000-000000000121',
    'tourism_analytics',
    'prototype-v1',
    true,
    'survey_capability'
  ),
  (
    '0197f4cf-2a69-7000-8000-000000000142',
    '0197f4cf-2a69-7000-8000-000000000134',
    '0197f4cf-2a69-7000-8000-000000000121',
    'tourism_analytics',
    'prototype-v1',
    true,
    'survey_capability'
  );

INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  version
)
SELECT
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((300 + fixture)::text, 12, '0')
  )::uuid,
  '0197f4cf-2a69-7000-8000-000000000101'::uuid,
  '0197f4cf-2a69-7000-8000-000000000104'::uuid,
  'pre_registered',
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((400 + fixture)::text, 12, '0')
  )::uuid,
  DATE '2026-06-15',
  DATE '2026-06-16',
  1,
  1
FROM generate_series(1, 17) AS fixture;

INSERT INTO survey.capabilities (
  id,
  token_hmac,
  token_key_version,
  purpose,
  stay_id,
  questionnaire_version_id,
  expires_at
)
SELECT
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((500 + fixture)::text, 12, '0')
  )::uuid,
  decode(lpad(to_hex(500 + fixture), 64, '0'), 'hex'),
  'local-v1',
  'survey_response',
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((300 + fixture)::text, 12, '0')
  )::uuid,
  '0197f4cf-2a69-7000-8000-000000000121'::uuid,
  TIMESTAMPTZ '2027-01-01T00:00:00Z'
FROM generate_series(1, 17) AS fixture;

INSERT INTO survey.responses (
  id,
  stay_id,
  questionnaire_version_id,
  capability_id,
  client_submission_id,
  participation,
  submitted_at
)
SELECT
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((600 + fixture)::text, 12, '0')
  )::uuid,
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((300 + fixture)::text, 12, '0')
  )::uuid,
  '0197f4cf-2a69-7000-8000-000000000121'::uuid,
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((500 + fixture)::text, 12, '0')
  )::uuid,
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((900 + fixture)::text, 12, '0')
  )::uuid,
  'submitted',
  TIMESTAMPTZ '2026-06-15T12:00:00-03:00'
    + fixture * interval '1 second'
FROM generate_series(1, 17) AS fixture;

INSERT INTO survey.answers (
  id,
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
)
SELECT
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((700 + fixture)::text, 12, '0')
  )::uuid,
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((600 + fixture)::text, 12, '0')
  )::uuid,
  '0197f4cf-2a69-7000-8000-000000000121'::uuid,
  '0197f4cf-2a69-7000-8000-000000000122'::uuid,
  '"first_visit"'::jsonb
FROM generate_series(1, 17) AS fixture;

INSERT INTO survey.consent_decisions (
  id,
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted,
  collection_channel
)
SELECT
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((800 + fixture)::text, 12, '0')
  )::uuid,
  (
    '0197f4cf-2a69-7000-8000-'
    || lpad((600 + fixture)::text, 12, '0')
  )::uuid,
  '0197f4cf-2a69-7000-8000-000000000121'::uuid,
  'tourism_analytics',
  'prototype-v1',
  true,
  'survey_capability'
FROM generate_series(1, 17) AS fixture;

INSERT INTO analytics.publication_runs (
  id,
  build_fingerprint,
  as_of_on,
  privacy_policy_version,
  methodology_version,
  status
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000109',
  repeat('f', 64),
  DATE '2026-07-28',
  'prototype-v1',
  'explainable-baseline-v1',
  'building'
);

INSERT INTO analytics.staged_metric_cells (
  publication_run_id,
  cell_key,
  metric_code,
  period_selector,
  period_start,
  period_end,
  dimension_code,
  category_code,
  kind,
  sample_size,
  accommodation_count,
  protection_status
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000109',
  repeat('9', 64),
  'first_visit_share',
  'last_complete_month',
  DATE '2026-06-01',
  DATE '2026-07-01',
  'visit_profile',
  'private-capability-canary-f4',
  'preference',
  99,
  1,
  'protected'
);

INSERT INTO public_data.publications (
  publication_version,
  build_fingerprint,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
)
VALUES (
  1,
  repeat('e', 64),
  DATE '2026-07-28',
  'prototype_fixtures',
  'prototype-v1',
  'explainable-baseline-v1',
  'published',
  65,
  TIMESTAMPTZ '2026-07-28T12:00:00Z'
);

INSERT INTO public_data.metric_cells (
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
  status,
  published_value,
  published_lower,
  published_central,
  published_upper
)
VALUES
  (
    1,
    repeat('a', 64),
    'presence',
    'recent_30_days',
    DATE '2026-07-28',
    DATE '2026-07-29',
    'person_day',
    'none',
    'none',
    'observed',
    'published',
    20,
    NULL,
    NULL,
    NULL
  ),
  (
    1,
    repeat('b', 64),
    'presence',
    'next_30_days',
    DATE '2026-07-29',
    DATE '2026-07-30',
    'person_day',
    'none',
    'none',
    'forecast',
    'published',
    NULL,
    10,
    20,
    30
  ),
  (
    1,
    repeat('c', 64),
    'first_visit_share',
    'last_complete_month',
    DATE '2026-06-01',
    DATE '2026-07-01',
    'survey_response',
    'visit_profile',
    'first_visit',
    'preference',
    'published',
    60,
    NULL,
    NULL,
    NULL
  ),
  (
    1,
    repeat('d', 64),
    'first_visit_share',
    'last_complete_month',
    DATE '2026-06-01',
    DATE '2026-07-01',
    'survey_response',
    'visit_profile',
    'returning',
    'preference',
    'published',
    40,
    NULL,
    NULL,
    NULL
  );

INSERT INTO public_data.current_publication (publication_version)
VALUES (1);

COMMIT;
SQL

validation_sql="$(
  sed -n \
    '/^-- name: ValidatePublicRuntimeSession /,/^-- name: ListCurrentSummaryCells /p' \
    "${ROOT_DIR}/apps/api/internal/platform/store/queries/public.sql" |
    sed '1d;$d'
)"
public_session="$(
  psql_as cumuru_public cumuru-local-public-only \
    --quiet --tuples-only --no-align \
    --command='SET ROLE public_runtime' \
    --command='SET search_path = pg_catalog, public_data' \
    --command="${validation_sql}"
)"
test "${public_session}" = \
  "public_runtime|cumuru_public|pg_catalog, public_data"

expect_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public login created a temporary table" \
  "CREATE TEMPORARY TABLE analytics_public_temp_canary (value integer)"

expect_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public login created a schema" \
  "CREATE SCHEMA analytics_public_schema_canary"

expect_failure \
  cumuru_app \
  cumuru-local-app-only \
  "application login assumed public_runtime" \
  "SET ROLE public_runtime"

for forbidden_role in app_runtime worker_runtime privacy_officer migration_admin; do
  expect_failure \
    cumuru_public \
    cumuru-local-public-only \
    "public login assumed ${forbidden_role}" \
    "SET ROLE ${forbidden_role}"
done

expect_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public runtime read a public_data base table" \
  "SET ROLE public_runtime; SELECT * FROM public_data.metric_cells"

expect_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public runtime read analytics microdata" \
  "SET ROLE public_runtime; SELECT * FROM analytics.presence_days"

expect_failure \
  cumuru_public \
  cumuru-local-public-only \
  "public runtime executed the preference aggregate" \
  "SET ROLE public_runtime;
   SELECT *
   FROM analytics.aggregate_eligible_preferences(
     'prototype-v1',
     TIMESTAMPTZ '2026-06-01T00:00:00-03:00',
     TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
   ) AS aggregate (
     privacy_policy_version text,
     metric_code text,
     category_code text,
     sample_size bigint,
     accommodation_count bigint,
     minimum_public_cell integer
   )"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
GRANT USAGE ON SCHEMA core TO cumuru_public;
GRANT SELECT ON TABLE core.organizations TO cumuru_public;
SQL

expect_no_rows \
  cumuru_public \
  cumuru-local-public-only \
  "public session validation accepted direct login ACL contamination" \
  "SET ROLE public_runtime;
   SET search_path = pg_catalog, public_data;
   ${validation_sql}"

direct_acl_risk="$(
  scalar_as \
    cumuru_public \
    cumuru-local-public-only \
    "RESET ROLE; SELECT count(*) FROM core.organizations"
)"
test "${direct_acl_risk}" = "1"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
REVOKE SELECT ON TABLE core.organizations FROM cumuru_public;
REVOKE USAGE ON SCHEMA core FROM cumuru_public;
SQL

psql_as postgres cumuru-local-admin-only <<'SQL'
CREATE ROLE analytics_public_elevated
  NOLOGIN CREATEDB CREATEROLE BYPASSRLS;
GRANT analytics_public_elevated TO cumuru_public;
SQL

expect_no_rows \
  cumuru_public \
  cumuru-local-public-only \
  "public session validation accepted an extra reachable role" \
  "SET ROLE public_runtime;
   SET search_path = pg_catalog, public_data;
   ${validation_sql}"

elevated_role_risk="$(
  scalar_as \
    cumuru_public \
    cumuru-local-public-only \
    "RESET ROLE;
     SET ROLE analytics_public_elevated;
     SELECT
       current_user,
       rolcreatedb,
       rolcreaterole,
       rolbypassrls
     FROM pg_catalog.pg_roles
     WHERE rolname = current_user"
)"
test "${elevated_role_risk}" = \
  "analytics_public_elevated|t|t|t"

psql_as postgres cumuru-local-admin-only <<'SQL'
REVOKE analytics_public_elevated FROM cumuru_public;
DROP ROLE analytics_public_elevated;
SQL

restored_public_session="$(
  psql_as cumuru_public cumuru-local-public-only \
    --quiet --tuples-only --no-align \
    --command='SET ROLE public_runtime' \
    --command='SET search_path = pg_catalog, public_data' \
    --command="${validation_sql}"
)"
test "${restored_public_session}" = \
  "public_runtime|cumuru_public|pg_catalog, public_data"

expect_failure \
  cumuru_app \
  cumuru-local-app-only \
  "app runtime read a quality base table" \
  "SELECT * FROM analytics.quality_snapshots"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime read encrypted survey text" \
  "SELECT encrypted_free_text FROM survey.answers LIMIT 1"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime enumerated survey response IDs" \
  "SELECT id FROM survey.responses LIMIT 1"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime read structured survey answers directly" \
  "SELECT structured_value FROM survey.answers LIMIT 1"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime read survey consent directly" \
  "SELECT granted FROM survey.consent_decisions LIMIT 1"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime read survey question metadata directly" \
  "SELECT minimum_public_cell FROM survey.questions LIMIT 1"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime mutated the metric catalog" \
  "UPDATE analytics.metric_catalog SET active = false"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "worker runtime mutated metric mappings" \
  "DELETE FROM analytics.metric_mappings"

catalog_access="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT
       (SELECT count(*) FROM analytics.metric_catalog),
       (SELECT count(*) FROM analytics.metric_mappings)"
)"
test "${catalog_access}" = "3|2"

boundary_proof="$(
  scalar_as \
    cumuru_migration \
    cumuru-local-migration-only \
    "SELECT
       count(*) FILTER (
         WHERE submitted_at < TIMESTAMPTZ '2026-06-01T00:00:00-03:00'
       ),
       count(*) FILTER (
         WHERE submitted_at = TIMESTAMPTZ '2026-06-01T00:00:00-03:00'
       ),
       count(*) FILTER (
         WHERE submitted_at > TIMESTAMPTZ '2026-06-01T00:00:00-03:00'
           AND submitted_at
             < TIMESTAMPTZ '2026-06-30T23:59:59.999999-03:00'
       ),
       count(*) FILTER (
         WHERE submitted_at
           = TIMESTAMPTZ '2026-06-30T23:59:59.999999-03:00'
       ),
       count(*) FILTER (
         WHERE submitted_at = TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
       )
     FROM survey.responses
     WHERE questionnaire_version_id
       = '0197f4cf-2a69-7000-8000-000000000121'"
)"
test "${boundary_proof}" = "1|1|17|1|1"

preference_sql="$(
  sed -n \
    '/^-- name: ListEligiblePreferenceCounts /,/^-- name: RecordAggregationFailureQualitySnapshot /p' \
    "${ROOT_DIR}/apps/api/internal/platform/store/queries/analytics.sql" |
    sed \
      -e '1d;$d' \
      -e "s/sqlc.arg(privacy_policy_version)::text/'prototype-v1'::text/g" \
      -e "s/sqlc.arg(period_start)::timestamptz/TIMESTAMPTZ '2026-06-01T00:00:00-03:00'/g" \
      -e "s/sqlc.arg(period_end)::timestamptz/TIMESTAMPTZ '2026-07-01T00:00:00-03:00'/g"
)"
preference_sql="${preference_sql%;}"
preference_nineteen="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT
       privacy_policy_version,
       metric_code,
       category_code,
       sample_size,
       accommodation_count,
       minimum_public_cell
     FROM (${preference_sql}) AS eligible
     ORDER BY category_code"
)"
test "${preference_nineteen}" = \
  $'prototype-v1|first_visit_share|first_visit|19|1|10\nprototype-v1|first_visit_share|returning|0|0|20'

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "preference aggregate accepted an empty policy" \
  "SELECT *
   FROM analytics.aggregate_eligible_preferences(
     '',
     TIMESTAMPTZ '2026-06-01T00:00:00-03:00',
     TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
   ) AS aggregate (
     privacy_policy_version text,
     metric_code text,
     category_code text,
     sample_size bigint,
     accommodation_count bigint,
     minimum_public_cell integer
   )"

expect_failure \
  cumuru_worker \
  cumuru-local-worker-only \
  "preference aggregate accepted a partial civil month" \
  "SELECT *
   FROM analytics.aggregate_eligible_preferences(
     'prototype-v1',
     TIMESTAMPTZ '2026-06-15T00:00:00-03:00',
     TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
   ) AS aggregate (
     privacy_policy_version text,
     metric_code text,
     category_code text,
     sample_size bigint,
     accommodation_count bigint,
     minimum_public_cell integer
   )"

injection_proof="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT count(*)
     FROM analytics.aggregate_eligible_preferences(
       'prototype-v1''; DELETE FROM analytics.metric_catalog; --',
       TIMESTAMPTZ '2026-06-01T00:00:00-03:00',
       TIMESTAMPTZ '2026-07-01T00:00:00-03:00'
     ) AS aggregate (
       privacy_policy_version text,
       metric_code text,
       category_code text,
       sample_size bigint,
       accommodation_count bigint,
       minimum_public_cell integer
     )"
)"
test "${injection_proof}" = "0"
test "$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT count(*) FROM analytics.metric_catalog"
)" = "3"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
BEGIN;

INSERT INTO core.stays (
  id,
  accommodation_id,
  created_by_membership_id,
  status,
  client_submission_id,
  planned_arrival_on,
  planned_departure_on,
  expected_guest_count,
  version
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000152',
  '0197f4cf-2a69-7000-8000-000000000101',
  '0197f4cf-2a69-7000-8000-000000000104',
  'pre_registered',
  '0197f4cf-2a69-7000-8000-000000000157',
  DATE '2026-06-20',
  DATE '2026-06-21',
  1,
  1
);

INSERT INTO survey.capabilities (
  id,
  token_hmac,
  token_key_version,
  purpose,
  stay_id,
  questionnaire_version_id,
  expires_at
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000153',
  decode(repeat('6', 64), 'hex'),
  'local-v1',
  'survey_response',
  '0197f4cf-2a69-7000-8000-000000000152',
  '0197f4cf-2a69-7000-8000-000000000121',
  TIMESTAMPTZ '2027-01-01T00:00:00Z'
);

INSERT INTO survey.responses (
  id,
  stay_id,
  questionnaire_version_id,
  capability_id,
  client_submission_id,
  participation,
  submitted_at
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000154',
  '0197f4cf-2a69-7000-8000-000000000152',
  '0197f4cf-2a69-7000-8000-000000000121',
  '0197f4cf-2a69-7000-8000-000000000153',
  '0197f4cf-2a69-7000-8000-000000000158',
  'submitted',
  TIMESTAMPTZ '2026-06-20T12:00:00-03:00'
);

INSERT INTO survey.answers (
  id,
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000155',
  '0197f4cf-2a69-7000-8000-000000000154',
  '0197f4cf-2a69-7000-8000-000000000121',
  '0197f4cf-2a69-7000-8000-000000000122',
  '"first_visit"'::jsonb
);

INSERT INTO survey.consent_decisions (
  id,
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted,
  collection_channel
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000156',
  '0197f4cf-2a69-7000-8000-000000000154',
  '0197f4cf-2a69-7000-8000-000000000121',
  'tourism_analytics',
  'prototype-v1',
  true,
  'survey_capability'
);

COMMIT;
SQL

preference_twenty="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT
       category_code,
       sample_size,
       accommodation_count,
       minimum_public_cell
     FROM (${preference_sql}) AS eligible
     ORDER BY category_code"
)"
test "${preference_twenty}" = \
  $'first_visit|20|1|10\nreturning|0|0|20'

failure_sql="$(
  sed -n \
    '/^-- name: RecordAggregationFailureQualitySnapshot /,/^-- name: DeleteStagedMetricCellsForRun /p' \
    "${ROOT_DIR}/apps/api/internal/platform/store/queries/analytics.sql" |
    sed '1d;$d'
)"
first_failure_sql="$(
  printf '%s\n' "${failure_sql}" |
    sed \
      -e "s/sqlc.arg(snapshot_id)::uuid/'0197f4cf-2a69-7000-8000-000000000160'::uuid/g" \
      -e "s/sqlc.arg(updated_at)::timestamptz/TIMESTAMPTZ '2026-07-28T13:00:00Z'/g"
)"
first_failure="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "${first_failure_sql}"
)"
test "${first_failure}" = \
  "0197f4cf-2a69-7000-8000-000000000160|1|0"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO analytics.quality_coverage (
  quality_snapshot_id,
  category_code,
  status,
  ratio
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000160',
  'formal_lodging',
  'available',
  0.65
);
SQL

second_failure_sql="$(
  printf '%s\n' "${failure_sql}" |
    sed \
      -e "s/sqlc.arg(snapshot_id)::uuid/'0197f4cf-2a69-7000-8000-000000000161'::uuid/g" \
      -e "s/sqlc.arg(updated_at)::timestamptz/TIMESTAMPTZ '2026-07-28T14:00:00Z'/g"
)"
second_failure="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "${second_failure_sql}"
)"
test "${second_failure}" = \
  "0197f4cf-2a69-7000-8000-000000000161|2|1"

quality_rows="$(
  scalar_as \
    cumuru_app \
    cumuru-local-app-only \
    "SELECT count(*) FROM analytics.current_quality"
)"
test "${quality_rows}" = "1"

quality_failure_proof="$(
  scalar_as \
    cumuru_migration \
    cumuru-local-migration-only \
    "SELECT
       current.publication_version,
       quality.aggregation_failures,
       count(coverage.category_code)
     FROM public_data.current_publication AS current
     CROSS JOIN analytics.quality_snapshots AS quality
     LEFT JOIN analytics.quality_coverage AS coverage
       ON coverage.quality_snapshot_id = quality.id
     WHERE quality.id
       = '0197f4cf-2a69-7000-8000-000000000161'
     GROUP BY current.publication_version, quality.aggregation_failures"
)"
test "${quality_failure_proof}" = "1|2|1"

coverage_sql="$(
  sed -n \
    '/^-- name: ListActiveAccommodationCoverage /,/^-- name: GetReconciliationRunByFingerprint /p' \
    "${ROOT_DIR}/apps/api/internal/platform/store/queries/analytics.sql" |
    sed \
      -e '1d;$d' \
      -e "s/sqlc.arg(window_start)::date/DATE '2026-07-01'/g" \
      -e "s/sqlc.arg(window_end)::date/DATE '2026-08-01'/g"
)"
coverage_sql="${coverage_sql%;}"
coverage_proof="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT
       count(*),
       sum(capacity),
       count(last_reported_at),
       sum(capacity) FILTER (WHERE last_reported_at IS NOT NULL)
     FROM (${coverage_sql}) AS coverage"
)"
test "${coverage_proof}" = "3|60|1|10"

psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
UPDATE core.stays
SET
  status = 'cancelled',
  cancelled_at = TIMESTAMPTZ '2026-07-28T15:00:00Z',
  cancellation_reason_code = 'fixture_cancelled',
  updated_at = TIMESTAMPTZ '2026-07-28T15:00:00Z'
WHERE id = '0197f4cf-2a69-7000-8000-000000000105';

UPDATE core.stays
SET
  status = 'no_show',
  no_show_at = TIMESTAMPTZ '2026-07-28T15:00:00Z',
  no_show_reason_code = 'fixture_no_show',
  updated_at = TIMESTAMPTZ '2026-07-28T15:00:00Z'
WHERE id = '0197f4cf-2a69-7000-8000-000000000123';
SQL

reconciliation_sql="$(
  sed -n \
    '/^-- name: ListPresenceReconciliationStays /,/^-- name: ListStayVisitorsForPresence /p' \
    "${ROOT_DIR}/apps/api/internal/platform/store/queries/analytics.sql" |
    sed '1d;$d'
)"
reconciliation_sql="${reconciliation_sql%;}"
authoritative_cleanup_sources="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT count(*)
     FROM (${reconciliation_sql}) AS source
     WHERE source.id IN (
       '0197f4cf-2a69-7000-8000-000000000105',
       '0197f4cf-2a69-7000-8000-000000000123'
     )
       AND source.status IN ('cancelled', 'no_show')"
)"
test "${authoritative_cleanup_sources}" = "2"

first_cleanup="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "WITH source AS (${reconciliation_sql}),
     deleted AS (
       DELETE FROM analytics.presence_days AS fact
       USING source
       WHERE fact.stay_id = source.id
         AND source.status IN ('cancelled', 'no_show')
         AND fact.source_stay_version <= source.expected_version
       RETURNING fact.stay_id
     )
     SELECT count(*) FROM deleted"
)"
test "${first_cleanup}" = "2"
test "$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT count(*) FROM analytics.presence_days"
)" = "1"

psql_as cumuru_migration cumuru-local-migration-only \
  --command="
    UPDATE core.stays
    SET version = 2, updated_at = TIMESTAMPTZ '2026-07-28T15:01:00Z'
    WHERE id = '0197f4cf-2a69-7000-8000-000000000105'
  "

second_cleanup="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "WITH source AS (${reconciliation_sql}),
     deleted AS (
       DELETE FROM analytics.presence_days AS fact
       USING source
       WHERE fact.stay_id = source.id
         AND source.status IN ('cancelled', 'no_show')
         AND fact.source_stay_version <= source.expected_version
       RETURNING fact.stay_id
     )
     SELECT count(*) FROM deleted"
)"
test "${second_cleanup}" = "1"

repeat_cleanup="$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "WITH source AS (${reconciliation_sql}),
     deleted AS (
       DELETE FROM analytics.presence_days AS fact
       USING source
       WHERE fact.stay_id = source.id
         AND source.status IN ('cancelled', 'no_show')
         AND fact.source_stay_version <= source.expected_version
       RETURNING fact.stay_id
     )
     SELECT count(*) FROM deleted"
)"
test "${repeat_cleanup}" = "0"
test "$(
  scalar_as \
    cumuru_worker \
    cumuru-local-worker-only \
    "SELECT count(*) FROM analytics.presence_days"
)" = "0"

if psql_as cumuru_migration cumuru-local-migration-only \
  >/dev/null 2>&1 <<'SQL'; then
BEGIN;
INSERT INTO public_data.publications (
  publication_version,
  build_fingerprint,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
)
VALUES (
  2,
  repeat('8', 64),
  DATE '2026-07-29',
  'prototype_fixtures',
  'prototype-v1',
  'explainable-baseline-v1',
  'published',
  65,
  TIMESTAMPTZ '2026-07-29T12:00:00Z'
);
INSERT INTO public_data.metric_cells (
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
  status,
  published_value
)
VALUES (
  2,
  repeat('7', 64),
  'presence',
  'recent_30_days',
  DATE '2026-07-29',
  DATE '2026-07-30',
  'person_day',
  'none',
  'none',
  'observed',
  'published',
  15
);
UPDATE public_data.current_publication SET publication_version = 2;
COMMIT;
SQL
  echo "invalid release transaction unexpectedly committed" >&2
  exit 1
fi

last_valid="$(
  scalar_as \
    cumuru_migration \
    cumuru-local-migration-only \
    "SELECT
       current.publication_version,
       count(publication.publication_version) FILTER (
         WHERE publication.publication_version = 2
       ),
       max(presence.published_value)
     FROM public_data.current_publication AS current
     CROSS JOIN public_data.publications AS publication
     CROSS JOIN public_data.current_presence AS presence
     GROUP BY current.publication_version"
)"
test "${last_valid}" = "1|0|20"

public_projection="$(
  psql_as cumuru_public cumuru-local-public-only \
    --quiet --tuples-only --no-align \
    --command="
      SET ROLE public_runtime;
      SELECT row_to_json(projected)::text
      FROM (
        SELECT * FROM public_data.current_summary
        UNION ALL
        SELECT * FROM public_data.current_presence
      ) AS projected
    "
)"
if grep --extended-regexp --quiet \
  '"(id|publication_version|build_fingerprint|cell_key|sample_size|accommodation_count|reason|reason_code|encrypted_free_text)"' \
  <<<"${public_projection}"; then
  echo "public projection exposed a forbidden key" >&2
  exit 1
fi
if grep --fixed-strings --quiet \
  'private-capability-canary-f4' <<<"${public_projection}"; then
  echo "public projection exposed a private canary" >&2
  exit 1
fi

echo "ANALYTICS_POSTGRES_INTEGRATION=PASS"
