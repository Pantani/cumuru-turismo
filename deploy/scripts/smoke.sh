#!/usr/bin/env bash
set -euo pipefail

WEB_BASE_URL="${WEB_BASE_URL:-http://127.0.0.1:4173}"
API_BASE_URL="${API_BASE_URL:-${WEB_BASE_URL}/api/v1}"
LOCAL_FAKE_OIDC_TOKEN="${LOCAL_FAKE_OIDC_TOKEN:-cumuru-local-platform-read}"
SMOKE_PROFILE="${SMOKE_PROFILE:-base}"

case "${SMOKE_PROFILE}" in
  base|local-demo) ;;
  *)
    echo "unsupported smoke profile: ${SMOKE_PROFILE}" >&2
    exit 2
    ;;
esac

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-local-smoke.XXXXXX")"
trap 'rm -rf -- "${temporary_dir}"' EXIT

request_json() {
  local path="$1"
  local output="$2"
  curl --fail --silent --show-error \
    --dump-header "${output}.headers" \
    --output "${output}" \
    "${API_BASE_URL}${path}"
}

curl --fail --silent --show-error \
  "${API_BASE_URL}/platform/health" >/dev/null
curl --fail --silent --show-error \
  "${API_BASE_URL}/platform/readiness" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/acesso" >/dev/null

proxied_health="$(
  curl --fail --silent --show-error \
    "${WEB_BASE_URL}/api/v1/platform/health"
)"
test "${proxied_health}" = '{"status":"ok"}'

if test "${SMOKE_PROFILE}" = "base"; then
  echo "BASE_SMOKE=PASS"
  exit 0
fi

request_json "/public/summary" "${temporary_dir}/summary.json"
request_json \
  "/public/presence?window=recent_30_days" \
  "${temporary_dir}/presence-recent.json"
request_json \
  "/public/presence?window=next_30_days" \
  "${temporary_dir}/presence-next.json"
request_json \
  "/public/preferences?period=last_complete_month" \
  "${temporary_dir}/preferences.json"
request_json "/public/methodology" "${temporary_dir}/methodology.json"
curl --fail --silent --show-error \
  --output "${temporary_dir}/questionnaire.json" \
  "${API_BASE_URL}/questionnaires/tourism_profile/active"

authenticated_phase2="$(
  curl --fail --silent --show-error \
    --header "Authorization: Bearer ${LOCAL_FAKE_OIDC_TOKEN}" \
    "${WEB_BASE_URL}/api/v1/accommodations?limit=10"
)"

AUTHENTICATED_PHASE2="${authenticated_phase2}" \
SMOKE_DIRECTORY="${temporary_dir}" node <<'NODE'
const fs = require("node:fs");
const path = require("node:path");

const directory = process.env.SMOKE_DIRECTORY;
const read = (name) =>
  JSON.parse(fs.readFileSync(path.join(directory, `${name}.json`), "utf8"));
const summary = read("summary");
const recent = read("presence-recent");
const next = read("presence-next");
const preferences = read("preferences");
const methodology = read("methodology");
const questionnaire = read("questionnaire");
const accommodations = JSON.parse(process.env.AUTHENTICATED_PHASE2 ?? "null");

if (!Array.isArray(accommodations?.items) || accommodations.items.length < 1) {
  throw new Error("local operator has no accommodation fixture");
}
if (summary?.presence_today?.status !== "published") {
  throw new Error("public summary is not published");
}
const publicPayloads = [summary, recent, next, preferences, methodology];
for (const payload of publicPayloads) {
  if (payload?.metadata?.data_mode !== "prototype_fixtures") {
    throw new Error("public payload is not marked as prototype fixtures");
  }
  const coverage = payload?.metadata?.coverage;
  if (
    coverage?.status !== "published" ||
    !Number.isInteger(coverage?.ratio) ||
    coverage.ratio < 0 ||
    coverage.ratio > 100
  ) {
    throw new Error("public coverage is not a published percentage");
  }
}
const peak = summary?.forecast_peak_next_30_days;
if (
  peak?.status !== "published" ||
  peak?.kind !== "forecast" ||
  !Number.isInteger(peak?.lower) ||
  !Number.isInteger(peak?.central) ||
  !Number.isInteger(peak?.upper) ||
  peak.lower > peak.central ||
  peak.central > peak.upper
) {
  throw new Error("summary forecast bounds are invalid");
}
if (!Array.isArray(recent?.series) || recent.series.length !== 30) {
  throw new Error("recent presence does not contain 30 points");
}
if (
  !recent.series.some(
    (point) => point.kind === "observed" && point.status === "published",
  )
) {
  throw new Error("recent presence has no published observed point");
}
if (!Array.isArray(next?.series) || next.series.length !== 30) {
  throw new Error("future presence does not contain 30 points");
}
if (
  !next.series.some(
    (point) =>
      point.kind === "forecast" &&
      point.status === "published" &&
      Number.isInteger(point.lower) &&
      Number.isInteger(point.central) &&
      Number.isInteger(point.upper) &&
      point.lower <= point.central &&
      point.central <= point.upper,
  )
) {
  throw new Error("future presence has no valid published forecast");
}
if (
  !Array.isArray(preferences?.metrics) ||
  preferences.metrics.length !== 1 ||
  !Array.isArray(preferences.metrics[0]?.categories) ||
  preferences.metrics[0].categories.length !== 2 ||
  !preferences.metrics[0].categories.every(
    (category) => category.status === "published",
  )
) {
  throw new Error("public preferences are not fully published");
}
if (
  methodology?.metadata?.privacy_policy_version !== "prototype-v1" ||
  methodology?.metadata?.methodology_version !== "explainable-baseline-v1" ||
  JSON.stringify(methodology?.forecast_bounds_percent) !== "[85,115]" ||
  JSON.stringify(methodology?.forecast_fallback_bounds_percent) !== "[70,130]"
) {
  throw new Error("unexpected public methodology");
}
if (
  questionnaire?.stable_key !== "tourism_profile" ||
  questionnaire?.privacy_notice_version !== "prototype-v1" ||
  !Array.isArray(questionnaire?.questions) ||
  questionnaire.questions.length !== 1 ||
  questionnaire.questions[0]?.stable_key !== "visit_profile" ||
  !Array.isArray(questionnaire.questions[0]?.options) ||
  questionnaire.questions[0].options.map((option) => option.value).join(",") !==
    "first_visit,returning" ||
  !Array.isArray(questionnaire?.consent_requirements) ||
  questionnaire.consent_requirements.length !== 1
) {
  throw new Error("active tourism questionnaire fixture is unavailable");
}

const forbiddenKeys = new Set([
  "accommodation_id",
  "capability",
  "client_submission_id",
  "sample_size",
  "stay_id",
  "visitor_id",
]);
function assertPublicShape(value) {
  if (Array.isArray(value)) {
    value.forEach(assertPublicShape);
    return;
  }
  if (value === null || typeof value !== "object") {
    return;
  }
  for (const [key, nested] of Object.entries(value)) {
    if (forbiddenKeys.has(key)) {
      throw new Error(`forbidden public key: ${key}`);
    }
    assertPublicShape(nested);
  }
}
publicPayloads.forEach(assertPublicShape);
NODE

for headers in "${temporary_dir}"/*.headers; do
  grep -Eiq '^etag: "[^"]+"' "${headers}"
  grep -Fiq 'cache-control: public, max-age=300, stale-if-error=86400' \
    "${headers}"
  grep -Eiq '^x-request-id: [^[:space:]]+' "${headers}"
done

echo "LOCAL_SMOKE=PASS"
