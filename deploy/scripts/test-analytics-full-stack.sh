#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-analytics-full-stack-${PPID}-$$"
CONFIG_FILE="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-config.XXXXXX")"
BEFORE_HEADERS="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-before-headers.XXXXXX")"
BEFORE_BODY="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-before-body.XXXXXX")"
AFTER_HEADERS="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-after-headers.XXXXXX")"
AFTER_BODY="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-after-body.XXXXXX")"
METHODOLOGY_BODY="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-methodology.XXXXXX")"
INVALID_DSN_LOG="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-invalid-dsn.XXXXXX")"
WORKER_LOG="$(mktemp "${TMPDIR:-/tmp}/cumuru-analytics-worker.XXXXXX")"

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

# Compose interpola `compose.local.yaml` inteiro, inclusive o serviço de seed
# sob profile, então as credenciais fictícias do demo são exigidas mesmo aqui.
# Em clone limpo e na CI não há `.env`; o exemplo versionado entra como origem.
LOCAL_ENV_FILE="${ROOT_DIR}/.env"
test -f "${LOCAL_ENV_FILE}" || LOCAL_ENV_FILE="${ROOT_DIR}/.env.example"

COMPOSE=(
  docker compose
  --env-file "${LOCAL_ENV_FILE}"
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/compose.local.yaml"
  --file "${ROOT_DIR}/deploy/compose.analytics-full-stack.yaml"
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
    echo "analytics full-stack left containers behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "analytics full-stack left its network behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker volume ls --quiet \
      --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "analytics full-stack left its volume behind" >&2
    cleanup_status=1
  fi
  rm -f -- \
    "${CONFIG_FILE}" \
    "${BEFORE_HEADERS}" \
    "${BEFORE_BODY}" \
    "${AFTER_HEADERS}" \
    "${AFTER_BODY}" \
    "${METHODOLOGY_BODY}" \
    "${INVALID_DSN_LOG}" \
    "${WORKER_LOG}"
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    exit "${cleanup_status}"
  fi
  echo "analytics full-stack cleanup confirmed"
}
trap cleanup EXIT

require_running() {
  local service="$1"
  if ! "${COMPOSE[@]}" ps --status running --services |
    grep --fixed-strings --line-regexp --quiet "${service}"; then
    echo "analytics full-stack service is not running: ${service}" >&2
    return 1
  fi
}

psql_migration() {
  "${COMPOSE[@]}" exec -T \
    -e PGPASSWORD=cumuru-local-migration-only \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username=cumuru_migration --dbname=cumuru "$@"
}

request_public() {
  local path="$1"
  local headers_path="$2"
  local body_path="$3"
  local status=""
  status="$(
    "${COMPOSE[@]}" exec -T \
      -e "REQUEST_PATH=${path}" \
      web sh -eu -c '
        curl \
          --silent \
          --show-error \
          --dump-header /tmp/analytics-full-stack.headers \
          --output /tmp/analytics-full-stack.body \
          --write-out "%{http_code}" \
          "http://127.0.0.1:8080${REQUEST_PATH}"
      '
  )"
  test "${status}" = "200"
  "${COMPOSE[@]}" exec -T web \
    sh -eu -c 'cat /tmp/analytics-full-stack.headers' >"${headers_path}"
  "${COMPOSE[@]}" exec -T web \
    sh -eu -c 'cat /tmp/analytics-full-stack.body' >"${body_path}"
}

header_value() {
  local name="$1"
  local path="$2"
  tr -d '\r' <"${path}" |
    awk -v expected="${name}" '
      tolower($1) == tolower(expected ":") {
        sub(/^[^:]+:[[:space:]]*/, "")
        print
        exit
      }
    '
}

"${COMPOSE[@]}" config --format json >"${CONFIG_FILE}"

node - "${CONFIG_FILE}" <<'NODE'
const fs = require("node:fs");
const config = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
const api = config.services.api;
const postgres = config.services.postgres;
const worker = config.services.worker;
const web = config.services.web;

if (api.environment.ANALYTICS_ENABLED !== "true") {
  throw new Error("analytics is disabled for api");
}
if (worker.environment.ANALYTICS_ENABLED !== "true") {
  throw new Error("analytics is disabled for worker");
}
if (worker.environment.ANALYTICS_INCREMENTAL_INTERVAL !== "500ms") {
  throw new Error("analytics worker cadence is not test-bounded");
}
if (!api.environment.PUBLIC_DATABASE_URL.includes("cumuru_public")) {
  throw new Error("public database login is not isolated");
}
if (api.environment.PUBLIC_DATABASE_URL === api.environment.DATABASE_URL) {
  throw new Error("public and application DSNs are identical");
}
for (const [name, service] of Object.entries({ api, postgres, web })) {
  if ((service.ports ?? []).length !== 0) {
    throw new Error(`${name} unexpectedly publishes a host port`);
  }
}
NODE

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate
"${COMPOSE[@]}" up --build --detach --wait local-demo api worker web
for service in postgres api worker web; do
  require_running "${service}"
done

"${COMPOSE[@]}" exec -T api \
  sh -c 'test -f /usr/share/zoneinfo/America/Bahia && TZ=America/Bahia date +%z' |
  grep --fixed-strings --line-regexp --quiet -- "-0300"

publication_ready=false
for _ in {1..60}; do
  current_publication="$(
    psql_migration \
      --tuples-only --no-align \
      --command="
        SELECT COALESCE(
          (
            SELECT publication_version::text
            FROM public_data.current_publication
            WHERE singleton
          ),
          ''
        )
      "
  )"
  if test -n "${current_publication}"; then
    publication_ready=true
    break
  fi
  sleep 1
done
if test "${publication_ready}" != "true"; then
  echo "worker did not create the initial analytics publication" >&2
  exit 1
fi

request_public \
  "/api/v1/public/summary" \
  "${BEFORE_HEADERS}" \
  "${BEFORE_BODY}"
before_etag="$(header_value ETag "${BEFORE_HEADERS}")"
test -n "${before_etag}"
test "$(header_value Cache-Control "${BEFORE_HEADERS}")" = \
  "public, max-age=300, stale-if-error=86400"

request_public \
  "/api/v1/public/methodology" \
  "${AFTER_HEADERS}" \
  "${METHODOLOGY_BODY}"
grep --fixed-strings --quiet \
  '"forecast_bounds_percent":[85,115]' "${METHODOLOGY_BODY}"
grep --fixed-strings --quiet \
  '"forecast_fallback_bounds_percent":[70,130]' "${METHODOLOGY_BODY}"

shared_dsn="postgres://cumuru_app:cumuru-local-app-only@postgres:5432/cumuru?sslmode=disable"
if "${COMPOSE[@]}" run --rm --no-deps \
  -e "PUBLIC_DATABASE_URL=${shared_dsn}" \
  api >"${INVALID_DSN_LOG}" 2>&1; then
  echo "API accepted a shared public/application DSN" >&2
  exit 1
fi
grep --fixed-strings --quiet "PUBLIC_DATABASE_URL" "${INVALID_DSN_LOG}"
if grep --fixed-strings --quiet "cumuru-local-app-only" "${INVALID_DSN_LOG}"; then
  echo "invalid public DSN failure leaked credentials" >&2
  exit 1
fi

failure_before="$(
  psql_migration \
    --tuples-only --no-align \
    --command="
      SELECT COALESCE(max(aggregation_failures), 0)
      FROM analytics.quality_snapshots
      WHERE window_code = 'last_30_days'
    "
)"

psql_migration \
  --command="
    REVOKE EXECUTE ON FUNCTION
      analytics.aggregate_eligible_preferences(text,timestamptz,timestamptz)
    FROM worker_runtime
  "

failure_observed=false
for _ in {1..60}; do
  failure_after="$(
    psql_migration \
      --tuples-only --no-align \
      --command="
        SELECT COALESCE(max(aggregation_failures), 0)
        FROM analytics.quality_snapshots
        WHERE window_code = 'last_30_days'
      "
  )"
  if test "${failure_after}" -gt "${failure_before}"; then
    failure_observed=true
    break
  fi
  sleep 1
done
if test "${failure_observed}" != "true"; then
  echo "worker publication failure was not recorded" >&2
  exit 1
fi

publication_after_failure="$(
  psql_migration \
    --tuples-only --no-align \
    --command="
      SELECT publication_version
      FROM public_data.current_publication
      WHERE singleton
    "
)"
test "${publication_after_failure}" = "${current_publication}"

request_public \
  "/api/v1/public/summary" \
  "${AFTER_HEADERS}" \
  "${AFTER_BODY}"
test "$(header_value ETag "${AFTER_HEADERS}")" = "${before_etag}"
cmp "${BEFORE_BODY}" "${AFTER_BODY}"

"${COMPOSE[@]}" logs --no-color worker >"${WORKER_LOG}" 2>&1
grep --fixed-strings --quiet "analytics publication failed" "${WORKER_LOG}"
for forbidden in \
  "aggregate_eligible_preferences" \
  "permission denied" \
  "cumuru-local-worker-only"; do
  if grep --fixed-strings --quiet "${forbidden}" "${WORKER_LOG}"; then
    echo "worker failure log leaked internal database detail" >&2
    exit 1
  fi
done

psql_migration \
  --command="
    GRANT EXECUTE ON FUNCTION
      analytics.aggregate_eligible_preferences(text,timestamptz,timestamptz)
    TO worker_runtime
  "

echo "ANALYTICS_LAST_VALID_SNAPSHOT=PASS"
echo "ANALYTICS_FULL_STACK_RUNTIME=PASS"
