#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAILURE_LOG="$(mktemp "${TMPDIR:-/tmp}/cumuru-local-build.XXXXXX")"
trap 'rm -f -- "${FAILURE_LOG}"' EXIT

cd "${ROOT_DIR}"

if VITE_LOCAL_DEMO_MODE=true \
  VITE_LOCAL_DEMO_IDENTITY='' \
  npm --workspace @cumuru/web run build >"${FAILURE_LOG}" 2>&1; then
  echo "local demo build accepted a missing identity" >&2
  exit 1
fi
grep --fixed-strings --quiet \
  "VITE_LOCAL_DEMO_IDENTITY is required" "${FAILURE_LOG}"

if VITE_LOCAL_DEMO_MODE=false \
  VITE_LOCAL_DEMO_IDENTITY=cumuru-local-platform-read \
  npm --workspace @cumuru/web run build >"${FAILURE_LOG}" 2>&1; then
  echo "default build accepted a local demo identity" >&2
  exit 1
fi
grep --fixed-strings --quiet \
  "VITE_LOCAL_DEMO_IDENTITY requires local demo mode" "${FAILURE_LOG}"

VITE_LOCAL_DEMO_MODE=true \
VITE_LOCAL_DEMO_IDENTITY=cumuru-local-platform-read \
  npm --workspace @cumuru/web run build
grep -rqF "cumuru-local-platform-read" apps/web/dist
grep -rqF "PROTOTYPE_ONLY" apps/web/dist

VITE_LOCAL_DEMO_MODE=false \
VITE_LOCAL_DEMO_IDENTITY='' \
  npm --workspace @cumuru/web run build
if grep -rqF "cumuru-local-platform-read" apps/web/dist; then
  echo "default web bundle contains the local demo identity" >&2
  exit 1
fi

echo "LOCAL_DEMO_BUILD_GUARDS=PASS"
echo "DEFAULT_BUNDLE_AUTHORITY_ABSENT=PASS"
