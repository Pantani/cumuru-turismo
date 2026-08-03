#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
METADATA_FILE="${ROOT_DIR}/deploy/build-metadata.env"

fail() {
  echo "build metadata: $*" >&2
  exit 2
}

hash_stream() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
    return
  fi
  fail "shasum or sha256sum is required"
}

hash_file() {
  local path="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${path}" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${path}" | awk '{print $1}'
    return
  fi
  fail "shasum or sha256sum is required"
}

source_revision() {
  local path relative
  {
    find \
      "${ROOT_DIR}/apps/api" \
      "${ROOT_DIR}/apps/web" \
      "${ROOT_DIR}/deploy/nginx" \
      "${ROOT_DIR}/deploy/postgres" \
      -type f \
      ! -path '*/node_modules/*' \
      ! -path '*/dist/*' \
      ! -path '*/.cache/*' \
      -print
    printf '%s\n' \
      "${ROOT_DIR}/package.json" \
      "${ROOT_DIR}/package-lock.json" \
      "${ROOT_DIR}/Makefile" \
      "${ROOT_DIR}/.dockerignore" \
      "${ROOT_DIR}/compose.yaml" \
      "${ROOT_DIR}/compose.local.yaml" \
      "${ROOT_DIR}/deploy/build-metadata.env" \
      "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  } | LC_ALL=C sort | while IFS= read -r path; do
    relative="${path#"${ROOT_DIR}/"}"
    printf '%s\0%s\n' "${relative}" "$(hash_file "${path}")"
  done | hash_stream
}

epoch_to_rfc3339() {
  local epoch="$1"
  if value="$(date -u -r "${epoch}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null)"; then
    printf '%s\n' "${value}"
    return
  fi
  if value="$(date -u -d "@${epoch}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null)"; then
    printf '%s\n' "${value}"
    return
  fi
  fail "cannot convert SOURCE_DATE_EPOCH=${epoch} to RFC 3339"
}

test -f "${METADATA_FILE}" || fail "missing ${METADATA_FILE}"
# The repository-owned file contains only simple KEY=VALUE assignments.
# shellcheck disable=SC1090
source "${METADATA_FILE}"

: "${CUMURU_BUILD_VERSION:?missing CUMURU_BUILD_VERSION}"
: "${SOURCE_DATE_EPOCH:?missing SOURCE_DATE_EPOCH}"
[[ "${CUMURU_BUILD_VERSION}" != "unknown" ]] ||
  fail "CUMURU_BUILD_VERSION cannot be unknown"
[[ "${CUMURU_BUILD_VERSION}" =~ ^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$ ]] ||
  fail "CUMURU_BUILD_VERSION contains unsupported characters"
[[ "${SOURCE_DATE_EPOCH}" =~ ^[0-9]+$ ]] ||
  fail "SOURCE_DATE_EPOCH must be an integer"

revision_source="source-tree"
if [[ -n "${CUMURU_BUILD_REVISION:-}" ]]; then
  [[ "${CUMURU_BUILD_REVISION}" != "unknown" ]] ||
    fail "CUMURU_BUILD_REVISION cannot be unknown"
  revision="${CUMURU_BUILD_REVISION}"
  revision_source="environment"
elif git -C "${ROOT_DIR}" rev-parse --is-inside-work-tree >/dev/null 2>&1 &&
  [[ -z "$(git -C "${ROOT_DIR}" status --porcelain --untracked-files=normal)" ]]; then
  revision="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
  revision_source="git"
else
  revision="source-$(source_revision)"
fi
[[ "${revision}" =~ ^[0-9A-Za-z][0-9A-Za-z._-]{0,127}$ ]] ||
  fail "CUMURU_BUILD_REVISION contains unsupported characters"

if [[ -n "${CUMURU_BUILD_TIME:-}" ]]; then
  built_at="${CUMURU_BUILD_TIME}"
elif [[ "${revision_source}" == "git" || "${revision_source}" == "environment" ]] &&
  git -C "${ROOT_DIR}" cat-file -e "${revision}^{commit}" >/dev/null 2>&1; then
  commit_epoch="$(git -C "${ROOT_DIR}" show -s --format=%ct "${revision}")"
  built_at="$(epoch_to_rfc3339 "${commit_epoch}")"
else
  built_at="$(epoch_to_rfc3339 "${SOURCE_DATE_EPOCH}")"
fi

[[ "${built_at}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] ||
  fail "CUMURU_BUILD_TIME must be RFC 3339 UTC without fractional seconds"

export CUMURU_BUILD_VERSION
export CUMURU_BUILD_REVISION="${revision}"
export CUMURU_BUILD_TIME="${built_at}"

test "$#" -gt 0 || fail "a command is required"
exec "$@"
