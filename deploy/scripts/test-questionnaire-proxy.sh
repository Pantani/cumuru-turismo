#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

bash "${ROOT_DIR}/deploy/scripts/test-proxy-hardening.sh"

grep --fixed-strings --quiet \
  "Survey-Capability:" \
  "${ROOT_DIR}/contracts/openapi.yaml"
grep --fixed-strings --quiet \
  "Survey-Capability" \
  "${ROOT_DIR}/apps/web/src/generated/schema.ts"
grep --fixed-strings --quiet \
  "access_log off;" \
  "${ROOT_DIR}/deploy/nginx/default.conf"

if grep --fixed-strings --quiet \
  "Survey-Capability" \
  "${ROOT_DIR}/deploy/nginx/default.conf"; then
  echo "Nginx must not log or rewrite the survey capability" >&2
  exit 1
fi

echo "questionnaire capability proxy contract passed"
