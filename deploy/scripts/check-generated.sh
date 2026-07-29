#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-generated-check.XXXXXX")"
QUERY_DIR="${ROOT_DIR}/apps/api/internal/platform/store/queries"
TEMP_QUERY_DIR="${TEMP_DIR}/project/apps/api/internal/platform/store/queries"

cleanup() {
  rm -rf -- "${TEMP_DIR}"
}
trap cleanup EXIT

mkdir -p \
  "${TEMP_QUERY_DIR}" \
  "${TEMP_DIR}/project/database"

(
  cd "${ROOT_DIR}"
  npm exec -- openapi-typescript contracts/openapi.yaml \
    -o "${TEMP_DIR}/schema.ts"
)
cmp "${TEMP_DIR}/schema.ts" \
  "${ROOT_DIR}/apps/web/src/generated/schema.ts"

cp "${ROOT_DIR}/apps/api/sqlc.yaml" \
  "${TEMP_DIR}/project/apps/api/sqlc.yaml"
cp "${ROOT_DIR}/database/schema.sql" \
  "${TEMP_DIR}/project/database/schema.sql"

query_count=0
while IFS= read -r -d '' query_file; do
  relative_path="${query_file#"${QUERY_DIR}/"}"
  mkdir -p "$(dirname "${TEMP_QUERY_DIR}/${relative_path}")"
  cp -- "${query_file}" "${TEMP_QUERY_DIR}/${relative_path}"
  query_count=$((query_count + 1))
done < <(find "${QUERY_DIR}" -type f -name '*.sql' -print0)

if [[ "${query_count}" -eq 0 ]]; then
  echo "no human SQL queries found under ${QUERY_DIR}" >&2
  exit 2
fi

(
  cd "${TEMP_DIR}/project/apps/api"
  go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
)

diff -ru \
  "${ROOT_DIR}/apps/api/internal/platform/store/generated" \
  "${TEMP_DIR}/project/apps/api/internal/platform/store/generated"

echo "generated artifacts are reproducible"
