#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cd "${ROOT_DIR}"
npm exec -- redocly lint contracts/openapi.yaml >/dev/null

node <<'NODE'
const fs = require("node:fs");
const yaml = require("js-yaml");

const document = yaml.load(fs.readFileSync("contracts/openapi.yaml", "utf8"));
const publicOperations = new Map([
  ["/public/summary", ["#/components/parameters/IfNoneMatch"]],
  ["/public/presence", [
    "#/components/parameters/PresenceWindow",
    "#/components/parameters/IfNoneMatch",
  ]],
  ["/public/preferences", [
    "#/components/parameters/PreferencePeriod",
    "#/components/parameters/IfNoneMatch",
  ]],
  ["/public/methodology", ["#/components/parameters/IfNoneMatch"]],
]);
const forbiddenPublicKeys = new Set([
  "id",
  "publication_version",
  "build_fingerprint",
  "cell_key",
  "sample_size",
  "accommodation_count",
  "reason",
  "reason_code",
  "suppression_reason",
  "free_text",
  "encrypted_free_text",
]);

function resolveRef(ref) {
  if (typeof ref !== "string" || !ref.startsWith("#/")) {
    throw new Error(`unsupported reference: ${ref}`);
  }
  return ref.slice(2).split("/").reduce((value, token) => {
    const decoded = token.replaceAll("~1", "/").replaceAll("~0", "~");
    return value?.[decoded];
  }, document);
}

function requireRef(value, expected, label) {
  if (value?.$ref !== expected || !resolveRef(value.$ref)) {
    throw new Error(`invalid ${label}`);
  }
}

function isForbiddenPublicKey(key) {
  return forbiddenPublicKeys.has(key) || key.endsWith("_id");
}

function inspectProperties(schema, seenRefs, trail) {
  const properties = schema.properties ?? {};
  for (const [key, child] of Object.entries(properties)) {
    if (isForbiddenPublicKey(key)) {
      throw new Error(`forbidden public key ${trail}.${key}`);
    }
    inspectSchema(child, seenRefs, `${trail}.${key}`);
  }
}

function inspectVariants(schema, seenRefs, trail) {
  const variants = [
    ...(schema.oneOf ?? []),
    ...(schema.anyOf ?? []),
    ...(schema.allOf ?? []),
    ...(schema.prefixItems ?? []),
  ];
  variants.forEach((child, index) => {
    inspectSchema(child, seenRefs, `${trail}[${index}]`);
  });
}

function inspectSchema(schema, seenRefs, trail) {
  if (schema?.$ref) {
    if (seenRefs.has(schema.$ref)) {
      return;
    }
    seenRefs.add(schema.$ref);
    inspectSchema(resolveRef(schema.$ref), seenRefs, schema.$ref);
    return;
  }
  if (!schema || typeof schema !== "object") {
    throw new Error(`invalid public schema at ${trail}`);
  }
  if (schema.type === "object" && schema.additionalProperties !== false) {
    throw new Error(`open public object at ${trail}`);
  }
  inspectProperties(schema, seenRefs, trail);
  inspectVariants(schema, seenRefs, trail);
  if (schema.items) {
    inspectSchema(schema.items, seenRefs, `${trail}.items`);
  }
}

function verifyPublicHeaders(headers, label) {
  requireRef(
    headers?.["X-Request-ID"],
    "#/components/headers/RequestId",
    `${label} request ID`,
  );
  requireRef(
    headers?.["Cache-Control"],
    "#/components/headers/PublicCache",
    `${label} cache`,
  );
  requireRef(
    headers?.ETag,
    "#/components/headers/PublicEntityTag",
    `${label} ETag`,
  );
}

function verifyPublicOperation(path, allowedParameters) {
  const operation = document.paths[path]?.get;
  if (!operation || operation.security?.length !== 0 || operation.requestBody) {
    throw new Error(`unsafe public operation: ${path}`);
  }
  const parameterRefs = operation.parameters?.map((item) => item.$ref) ?? [];
  if (JSON.stringify(parameterRefs) !== JSON.stringify(allowedParameters)) {
    throw new Error(`open or missing selector contract: ${path}`);
  }
  const success = operation.responses?.["200"];
  const notModified = operation.responses?.["304"];
  verifyPublicHeaders(success?.headers, `${path} 200`);
  requireRef(
    notModified,
    "#/components/responses/PublicNotModified",
    `${path} 304`,
  );
  verifyPublicHeaders(resolveRef(notModified.$ref).headers, `${path} 304`);
  const successSchema = success?.content?.["application/json"]?.schema;
  inspectSchema(successSchema, new Set(), `${path} 200`);
  for (const code of ["400", "503"]) {
    const problem = operation.responses?.[code];
    requireRef(problem, "#/components/responses/Problem", `${path} ${code}`);
    requireRef(
      resolveRef(problem.$ref).headers?.["Cache-Control"],
      "#/components/headers/NoStore",
      `${path} ${code} no-store`,
    );
  }
}

const publicCache = document.components.headers.PublicCache;
if (
  publicCache.required !== true
  || publicCache.schema?.const !== "public, max-age=300, stale-if-error=86400"
) {
  throw new Error("public cache policy drifted");
}
const publicETag = document.components.headers.PublicEntityTag;
const strongETagPattern = '^"sha256-[0-9a-f]{64}"$';
if (
  publicETag.required !== true
  || publicETag.schema?.pattern !== strongETagPattern
  || document.components.parameters.IfNoneMatch.schema?.pattern
    !== strongETagPattern
) {
  throw new Error("strong ETag contract drifted");
}
const methodology = document.components.schemas.PublicMethodology;
const nominalBounds = methodology.properties?.forecast_bounds_percent;
const fallbackBounds =
  methodology.properties?.forecast_fallback_bounds_percent;
if (
  !methodology.required?.includes("forecast_bounds_percent")
  || !methodology.required?.includes("forecast_fallback_bounds_percent")
  || nominalBounds?.prefixItems?.[0]?.const !== 85
  || nominalBounds?.prefixItems?.[1]?.const !== 115
  || fallbackBounds?.prefixItems?.[0]?.const !== 70
  || fallbackBounds?.prefixItems?.[1]?.const !== 130
) {
  throw new Error("forecast methodology bounds drifted");
}
if (document.info.version !== "0.6.0") {
  throw new Error("phase 4 contract version drifted");
}
requireRef(
  document.components.schemas.PublicSummary.properties?.presence_today,
  "#/components/schemas/ObservedPresencePoint",
  "summary observed presence",
);
const presenceVariants =
  document.components.schemas.PublicPresence.oneOf?.map((item) => item.$ref);
if (JSON.stringify(presenceVariants) !== JSON.stringify([
  "#/components/schemas/ObservedPublicPresence",
  "#/components/schemas/ForecastPublicPresence",
])) {
  throw new Error("presence window discriminators drifted");
}
const coverageVariants =
  document.components.schemas.QualityCoverage.oneOf?.map((item) => item.$ref);
if (JSON.stringify(coverageVariants) !== JSON.stringify([
  "#/components/schemas/AvailableQualityCoverage",
  "#/components/schemas/UnavailableQualityCoverage",
])) {
  throw new Error("quality coverage invariant drifted");
}
for (const [path, allowedParameters] of publicOperations) {
  verifyPublicOperation(path, allowedParameters);
}

const quality = document.paths["/analytics/quality"].get;
const scope = quality.security?.[0]?.oidc ?? [];
if (!scope.includes("analytics:read:internal")) {
  throw new Error("quality scope is missing");
}
NODE

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

PROJECT_NAME="cumuru-phase4-proxy-${PPID}-$$"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-phase4-proxy.XXXXXX")"
COMPOSE=(
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.phase4-full-stack.yaml"
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
    echo "phase 4 proxy left containers behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "phase 4 proxy left its network behind" >&2
    cleanup_status=1
  fi
  residual="$(
    docker volume ls --quiet --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    echo "phase 4 proxy left its volume behind" >&2
    cleanup_status=1
  fi
  if test -d "${WORK_DIR}" && ! rm -rf -- "${WORK_DIR}"; then
    cleanup_status=1
  fi
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    exit "${cleanup_status}"
  fi
  echo "phase 4 proxy cleanup confirmed"
}
trap cleanup EXIT

require_running() {
  local service="$1"
  if ! "${COMPOSE[@]}" ps --status running --services |
    grep --fixed-strings --line-regexp --quiet "${service}"; then
    echo "phase 4 proxy service is not running: ${service}" >&2
    return 1
  fi
}

assert_absent() {
  local path="$1"
  local value="$2"
  local message="$3"
  if grep --fixed-strings --quiet -- "${value}" "${path}"; then
    echo "${message}" >&2
    return 1
  fi
}

LAST_STATUS=""
LAST_HEADERS=""
LAST_BODY=""

request() {
  local path="$1"
  local token="${2:-}"
  local if_none_match="${3:-}"
  local capability="${4:-}"
  LAST_STATUS="$(
    "${COMPOSE[@]}" exec -T \
      -e "REQUEST_PATH=${path}" \
      -e "AUTH_TOKEN=${token}" \
      -e "IF_NONE_MATCH=${if_none_match}" \
      -e "SURVEY_CAPABILITY=${capability}" \
      web sh -eu -c '
        : > /tmp/phase4-response.body
        set -- \
          --silent \
          --show-error \
          --dump-header /tmp/phase4-response.headers \
          --output /tmp/phase4-response.body \
          --write-out "%{http_code}"
        if test -n "${AUTH_TOKEN}"; then
          set -- "$@" --header "Authorization: Bearer ${AUTH_TOKEN}"
        fi
        if test -n "${IF_NONE_MATCH}"; then
          set -- "$@" --header "If-None-Match: ${IF_NONE_MATCH}"
        fi
        if test -n "${SURVEY_CAPABILITY}"; then
          set -- "$@" --header "Survey-Capability: ${SURVEY_CAPABILITY}"
        fi
        curl "$@" "http://127.0.0.1:8080${REQUEST_PATH}"
      '
  )"
  LAST_HEADERS="$(
    "${COMPOSE[@]}" exec -T web \
      sh -eu -c 'cat /tmp/phase4-response.headers'
  )"
  LAST_BODY="$(
    "${COMPOSE[@]}" exec -T web \
      sh -eu -c 'cat /tmp/phase4-response.body'
  )"
}

header_value() {
  local name="$1"
  printf '%s\n' "${LAST_HEADERS}" |
    tr -d '\r' |
    awk -v expected="${name}" '
      tolower($1) == tolower(expected ":") {
        sub(/^[^:]+:[[:space:]]*/, "")
        print
        exit
      }
    '
}

assert_status() {
  local expected="$1"
  if test "${LAST_STATUS}" != "${expected}"; then
    echo "unexpected HTTP status: got ${LAST_STATUS}, want ${expected}" >&2
    return 1
  fi
}

assert_header() {
  local name="$1"
  local expected="$2"
  local actual=""
  actual="$(header_value "${name}")"
  if test "${actual}" != "${expected}"; then
    echo "unexpected ${name}: got ${actual}, want ${expected}" >&2
    return 1
  fi
}

assert_public_success() {
  assert_status 200
  assert_header Cache-Control "public, max-age=300, stale-if-error=86400"
  if ! header_value Content-Type | grep --fixed-strings --quiet "application/json"; then
    echo "public analytics response is not JSON" >&2
    return 1
  fi
  if test -z "$(header_value X-Request-ID)"; then
    echo "public analytics response lacks X-Request-ID" >&2
    return 1
  fi
  if ! header_value ETag |
    grep --extended-regexp --line-regexp --quiet '"sha256-[0-9a-f]{64}"'; then
    echo "public analytics response lacks a strong ETag" >&2
    return 1
  fi
}

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate

"${COMPOSE[@]}" exec -T \
  -e PGPASSWORD=cumuru-local-migration-only \
  postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
  --username=cumuru_migration --dbname=cumuru <<'SQL'
INSERT INTO core.organizations (id, name)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000201',
  'private-runtime-canary-f4-72f51c'
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

INSERT INTO analytics.quality_snapshots (
  id,
  window_code,
  updated_at,
  incomplete_stays,
  overdue_planned_departures,
  silent_accommodations,
  aggregation_failures,
  suspected_duplicates,
  suspected_duplicates_reason,
  fnrh_failures,
  fnrh_failures_reason
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000202',
  'last_30_days',
  TIMESTAMPTZ '2026-07-28T12:00:00Z',
  0,
  0,
  2,
  0,
  NULL,
  'pseudonym_not_approved',
  NULL,
  'phase_not_implemented'
);

INSERT INTO analytics.quality_coverage (
  quality_snapshot_id,
  category_code,
  status,
  ratio
)
VALUES (
  '0197f4cf-2a69-7000-8000-000000000202',
  'formal_lodging',
  'available',
  0.65
);
SQL

"${COMPOSE[@]}" up --build --detach --wait api web
for service in postgres api web; do
  require_running "${service}"
done

ready=false
for ignored in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if request "/api/v1/public/summary" && test "${LAST_STATUS}" = "200"; then
    ready=true
    break
  fi
  sleep 1
done
if test "${ready}" != "true"; then
  echo "timed out waiting for phase 4 public analytics" >&2
  exit 1
fi

PUBLIC_BODIES="${WORK_DIR}/public-bodies.jsonl"
assert_public_success
summary_etag="$(header_value ETag)"
printf '%s\n' "${LAST_BODY}" >>"${PUBLIC_BODIES}"
grep --fixed-strings --quiet '"presence_today"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"value":20' <<<"${LAST_BODY}"

request "/api/v1/public/presence?window=recent_30_days"
assert_public_success
printf '%s\n' "${LAST_BODY}" >>"${PUBLIC_BODIES}"
grep --fixed-strings --quiet '"window":"recent_30_days"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"value":20' <<<"${LAST_BODY}"

request "/api/v1/public/presence?window=next_30_days"
assert_public_success
printf '%s\n' "${LAST_BODY}" >>"${PUBLIC_BODIES}"
grep --fixed-strings --quiet '"window":"next_30_days"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"central":20' <<<"${LAST_BODY}"

request "/api/v1/public/preferences?period=last_complete_month"
assert_public_success
printf '%s\n' "${LAST_BODY}" >>"${PUBLIC_BODIES}"
grep --fixed-strings --quiet '"category_code":"first_visit"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"share_percent":60' <<<"${LAST_BODY}"

request "/api/v1/public/methodology"
assert_public_success
printf '%s\n' "${LAST_BODY}" >>"${PUBLIC_BODIES}"
grep --fixed-strings --quiet '"time_zone":"America/Bahia"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"rounding_mode":"stable-half-up"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet \
  '"forecast_bounds_percent":[85,115]' <<<"${LAST_BODY}"
grep --fixed-strings --quiet \
  '"forecast_fallback_bounds_percent":[70,130]' <<<"${LAST_BODY}"

request "/api/v1/public/summary" "" "${summary_etag}"
assert_status 304
assert_header Cache-Control "public, max-age=300, stale-if-error=86400"
assert_header ETag "${summary_etag}"
test -n "$(header_value X-Request-ID)"
test -z "${LAST_BODY}"

request "/api/v1/public/presence?window=custom"
assert_status 400
assert_header Cache-Control "no-store"

request "/api/v1/public/summary?release=1"
assert_status 400
assert_header Cache-Control "no-store"

request "/api/v1/analytics/quality?window=last_30_days"
assert_status 401
assert_header Cache-Control "no-store"

request \
  "/api/v1/analytics/quality?window=last_30_days" \
  "cumuru-local-platform-read"
assert_status 403
assert_header Cache-Control "no-store"

request \
  "/api/v1/analytics/quality?window=last_30_days" \
  "cumuru-local-analytics-quality"
assert_status 200
assert_header Cache-Control "no-store"
grep --fixed-strings --quiet '"silent_accommodations"' <<<"${LAST_BODY}"
grep --fixed-strings --quiet '"reason_code":"pseudonym_not_approved"' <<<"${LAST_BODY}"

request \
  "/api/v1/analytics/quality?window=custom" \
  "cumuru-local-analytics-quality"
assert_status 400
assert_header Cache-Control "no-store"

CAPABILITY_CANARY="phase4-capability-canary-4ca7f2"
QUERY_CANARY="phase4-query-canary-19b83e"
request \
  "/api/v1/public/summary?release=${QUERY_CANARY}" \
  "" \
  "" \
  "${CAPABILITY_CANARY}"
assert_status 400
assert_header Cache-Control "no-store"

if grep --extended-regexp --quiet \
  '"(id|publication_version|build_fingerprint|cell_key|sample_size|accommodation_count|reason|reason_code|suppression_reason|free_text|encrypted_free_text)"[[:space:]]*:' \
  "${PUBLIC_BODIES}"; then
  echo "public analytics runtime exposed a forbidden key" >&2
  exit 1
fi
assert_absent \
  "${PUBLIC_BODIES}" \
  "0197f4cf-2a69-7000-8000-000000000201" \
  "public analytics runtime exposed a private UUID"
assert_absent \
  "${PUBLIC_BODIES}" \
  "private-runtime-canary-f4-72f51c" \
  "public analytics runtime exposed a private database canary"

RUNTIME_LOG="${WORK_DIR}/runtime.log"
"${COMPOSE[@]}" logs --no-color api web >"${RUNTIME_LOG}" 2>&1
assert_absent \
  "${RUNTIME_LOG}" \
  "${CAPABILITY_CANARY}" \
  "phase 4 runtime logs leaked the synthetic capability"
assert_absent \
  "${RUNTIME_LOG}" \
  "${QUERY_CANARY}" \
  "phase 4 runtime logs leaked the synthetic query"
assert_absent \
  "${RUNTIME_LOG}" \
  "private-runtime-canary-f4-72f51c" \
  "phase 4 runtime logs leaked private database data"

echo "PHASE4_PROXY_RUNTIME=PASS"
