#!/usr/bin/env bash
set -euo pipefail

WEB_BASE_URL="${WEB_BASE_URL:-http://127.0.0.1:4173}"
API_BASE_URL="${API_BASE_URL:-${WEB_BASE_URL}/api/v1}"
LOCAL_FAKE_OIDC_TOKEN="${LOCAL_FAKE_OIDC_TOKEN:-cumuru-local-platform-read}"

curl --fail --silent --show-error \
  "${API_BASE_URL}/platform/health" >/dev/null
curl --fail --silent --show-error \
  "${API_BASE_URL}/platform/readiness" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/registro" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/acesso" >/dev/null
curl --fail --silent --show-error \
  "${WEB_BASE_URL}/nao-existe" >/dev/null

proxied_health="$(
  curl --fail --silent --show-error \
    "${WEB_BASE_URL}/api/v1/platform/health"
)"
test "${proxied_health}" = '{"status":"ok"}'

authenticated_phase2="$(
  curl --fail --silent --show-error \
    --header "Authorization: Bearer ${LOCAL_FAKE_OIDC_TOKEN}" \
    "${WEB_BASE_URL}/api/v1/accommodations?limit=1"
)"
AUTHENTICATED_PHASE2="${authenticated_phase2}" node -e '
  const response = JSON.parse(process.env.AUTHENTICATED_PHASE2 ?? "null");
  if (response === null || !Array.isArray(response.items)) {
    process.exit(1);
  }
'

echo "local smoke checks passed"
