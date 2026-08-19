#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-local-demo-test-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet local-demo-test
CONCURRENT_FIRST="$(mktemp "${TMPDIR:-/tmp}/cumuru-local-demo-first.XXXXXX")"
CONCURRENT_SECOND="$(mktemp "${TMPDIR:-/tmp}/cumuru-local-demo-second.XXXXXX")"

build_metadata=()
while IFS= read -r value; do
  build_metadata+=("${value}")
done < <(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh" \
    sh -c 'printf "%s\n%s\n%s\n" \
      "${CUMURU_BUILD_VERSION}" \
      "${CUMURU_BUILD_REVISION}" \
      "${CUMURU_BUILD_TIME}"'
)
test "${#build_metadata[@]}" = "3"
export CUMURU_BUILD_VERSION="${build_metadata[0]}"
export CUMURU_BUILD_REVISION="${build_metadata[1]}"
export CUMURU_BUILD_TIME="${build_metadata[2]}"

# Compose lê `.env` do diretório do projeto, e o serviço de seed exige as
# credenciais fictícias do demo local. Em clone limpo — e na CI — esse arquivo
# não existe, então o exemplo versionado entra como origem explícita. Um `.env`
# do desenvolvedor continua tendo precedência.
LOCAL_ENV_FILE="${ROOT_DIR}/.env"
test -f "${LOCAL_ENV_FILE}" || LOCAL_ENV_FILE="${ROOT_DIR}/.env.example"

COMPOSE=(
  docker compose
  --env-file "${LOCAL_ENV_FILE}"
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/compose.local.yaml"
  --file "${ROOT_DIR}/deploy/compose.local-test.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local status=$?
  trap - EXIT
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -f -- "${CONCURRENT_FIRST}" "${CONCURRENT_SECOND}"
  # O lock só cai depois do teardown: é isso que impede a execução seguinte de
  # pedir um endereço que ainda está sendo liberado.
  cumuru_release_subnet
  exit "${status}"
}
trap cleanup EXIT

psql_migration() {
  "${COMPOSE[@]}" exec -T \
    -e PGPASSWORD=cumuru-local-migration-only \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username=cumuru_migration --dbname=cumuru "$@"
}

fixture_counts() {
  psql_migration --tuples-only --no-align --command="
    SELECT concat_ws(
      ',',
      (SELECT count(*) FROM core.organizations
        WHERE id = '019fae10-0000-7000-8000-000000000001'),
      (SELECT count(*) FROM core.accommodations
        WHERE id::text LIKE '019fae11-%'),
      (SELECT count(*) FROM core.memberships
        WHERE oidc_issuer = 'https://oidc.invalid/local'
          AND oidc_subject = 'fixture-platform-probe'),
      (SELECT count(*) FROM survey.questionnaire_versions
        WHERE id = '019fae13-0000-7000-8000-000000000002'
          AND status = 'published'),
      (SELECT count(*) FROM analytics.metric_mappings
        WHERE questionnaire_version_id =
          '019fae13-0000-7000-8000-000000000002'),
      (SELECT count(*) FROM survey.responses),
      (SELECT count(*) FROM survey.responses
        WHERE substring(id::text FROM 15 FOR 1) = '7'),
      (SELECT count(*) FROM core.stays
        WHERE accommodation_id::text LIKE '019fae11-%'),
      (SELECT count(*) FROM core.visitors v
        JOIN core.stays s ON s.id = v.stay_id
        WHERE s.accommodation_id::text LIKE '019fae11-%')
    )
  "
}

# O catálogo e o questionário são fixos; o volume de estadias acompanha a
# janela publicada de dois anos, então ele é conferido por piso e não por
# igualdade — amanhã a janela é outra e a contagem exata muda sozinha.
require_minimum() {
  local actual="$1"
  local minimum="$2"
  local label="$3"
  if test "${actual}" -ge "${minimum}"; then
    return
  fi
  printf '%s: got %q, want at least %q\n' \
    "${label}" "${actual}" "${minimum}" >&2
  return 1
}

run_local_demo() {
  local output=""
  local status=0
  set +e
  output="$("${COMPOSE[@]}" run --rm --no-deps local-demo 2>&1)"
  status=$?
  set -e
  if test "${status}" -ne 0; then
    printf '%s\n' "${output}" >&2
    return "${status}"
  fi
  printf '%s\n' "${output}"
}

require_log_marker() {
  local output="$1"
  local marker="$2"
  if grep --fixed-strings --quiet "${marker}" <<<"${output}"; then
    return
  fi
  printf '%s\n' "${output}" >&2
  printf 'local demo output missing marker: %s\n' "${marker}" >&2
  return 1
}

require_equal() {
  local actual="$1"
  local expected="$2"
  local label="$3"
  if test "${actual}" = "${expected}"; then
    return
  fi
  printf '%s: got %q, want %q\n' \
    "${label}" "${actual}" "${expected}" >&2
  return 1
}

"${COMPOSE[@]}" build local-demo
"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate

first_log="$(run_local_demo)"
require_log_marker "${first_log}" "LOCAL_DEMO_SEED=PASS"
require_log_marker "${first_log}" "source=go"
if grep -Eiq 'bearer|authorization|capability|cumuru-local-platform-read' \
  <<<"${first_log}"; then
  echo "local demo bootstrap leaked authority material" >&2
  exit 1
fi

before="$(fixture_counts)"
IFS=, read -r \
  fresh_organizations fresh_accommodations fresh_memberships \
  fresh_versions fresh_mappings fresh_responses fresh_v7_responses \
  fresh_stays fresh_visitors <<<"${before}"
require_equal \
  "${fresh_organizations},${fresh_accommodations},${fresh_memberships}" \
  "1,16,16" \
  "fresh catalogue counts"
require_equal \
  "${fresh_versions},${fresh_mappings}" \
  "1,2" \
  "fresh questionnaire counts"
# A lista pública mostra quem consentiu, não quem existe: o catálogo tem 16
# hospedagens e 13 publicadas, e a diferença é o que prova que estar cadastrada
# não publica contato.
published_listings="$(
  psql_migration --tuples-only --no-align --command="
    SELECT count(*)
    FROM core.accommodations
    WHERE id::text LIKE '019fae11-%'
      AND public_listing_enabled = true
      AND public_contact_phone IS NOT NULL
      AND public_listing_consented_at IS NOT NULL
  "
)"
require_equal "${published_listings}" "13" "published directory entries"
require_equal \
  "${fresh_responses}" \
  "${fresh_v7_responses}" \
  "survey responses carry a v7 identifier"
require_minimum "${fresh_responses}" 500 "fresh survey responses"
require_minimum "${fresh_stays}" 900 "fresh stays"
require_minimum "${fresh_visitors}" 4000 "fresh visitors"

psql_migration --command="
  INSERT INTO core.organizations (id, name)
  VALUES ('019fae99-0000-7000-8000-000000000001', 'Canário preservado')
"

second_log="$(run_local_demo)"
require_log_marker "${second_log}" "LOCAL_DEMO_SEED=PASS"
require_log_marker "${second_log}" "source=go"
after="$(fixture_counts)"
require_equal "${after}" "${before}" "idempotent fixture counts"

run_local_demo >"${CONCURRENT_FIRST}" 2>&1 &
first_pid=$!
run_local_demo >"${CONCURRENT_SECOND}" 2>&1 &
second_pid=$!
first_status=0
second_status=0
wait "${first_pid}" || first_status=$?
wait "${second_pid}" || second_status=$?
require_equal "${first_status}" "0" "first concurrent run status"
require_equal "${second_status}" "0" "second concurrent run status"
require_log_marker \
  "$(cat "${CONCURRENT_FIRST}")" \
  "LOCAL_DEMO_SEED=PASS"
require_log_marker \
  "$(cat "${CONCURRENT_SECOND}")" \
  "LOCAL_DEMO_SEED=PASS"
require_equal "$(fixture_counts)" "${before}" "concurrent fixture counts"

psql_migration --command="
  UPDATE core.stays
  SET
    planned_arrival_on =
      (timezone('America/Bahia', now()))::date - 10,
    planned_departure_on =
      (timezone('America/Bahia', now()))::date - 5
  WHERE accommodation_id::text LIKE '019fae11-%'
    AND status = 'checked_in'
"

rollover_log="$(run_local_demo)"
require_log_marker "${rollover_log}" "LOCAL_DEMO_SEED=PASS"
reconciled="$(
  psql_migration --tuples-only --no-align --command="
    SELECT count(*)
    FROM core.stays
    WHERE accommodation_id::text LIKE '019fae11-%'
      AND status = 'checked_in'
      AND planned_arrival_on =
        (timezone('America/Bahia', now()))::date - 2
      AND planned_departure_on =
        (timezone('America/Bahia', now()))::date + 35
  "
)"
require_equal "${reconciled}" "16" "reconciled current stays"
require_equal "$(fixture_counts)" "${before}" "rollover fixture counts"

canary="$(
  psql_migration --tuples-only --no-align --command="
    SELECT name
    FROM core.organizations
    WHERE id = '019fae99-0000-7000-8000-000000000001'
  "
)"
require_equal "${canary}" "Canário preservado" "preserved canary"

psql_migration --command="
  UPDATE core.accommodations
  SET name = 'Colisão reservada preservada'
  WHERE id = '019fae11-0000-7000-8000-000000000001'
"

if "${COMPOSE[@]}" run --rm --no-deps local-demo >/dev/null 2>&1; then
  echo "local demo accepted a divergent reserved fixture" >&2
  exit 1
fi
collision="$(
  psql_migration --tuples-only --no-align --command="
    SELECT name
    FROM core.accommodations
    WHERE id = '019fae11-0000-7000-8000-000000000001'
  "
)"
require_equal \
  "${collision}" \
  "Colisão reservada preservada" \
  "reserved collision"

echo "LOCAL_DEMO_FRESH_DATABASE=PASS"
echo "LOCAL_DEMO_IDEMPOTENCY=PASS"
echo "LOCAL_DEMO_CONCURRENCY=PASS"
echo "LOCAL_DEMO_NON_DESTRUCTIVE=PASS"
echo "LOCAL_DEMO_ROLLOVER=PASS"
echo "LOCAL_DEMO_COLLISION_FAIL_CLOSED=PASS"
