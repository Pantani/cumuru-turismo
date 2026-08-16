#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BASE_COMPOSE="${ROOT_DIR}/compose.yaml"
OBSERVABILITY_COMPOSE="${ROOT_DIR}/deploy/compose.observability.yaml"
ACTION="${1:-}"

compose() {
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh" \
    docker compose \
    --project-directory "${ROOT_DIR}" \
    --file "${BASE_COMPOSE}" \
    --file "${OBSERVABILITY_COMPOSE}" \
    "$@"
}

usage() {
  echo "uso: $0 {config|up|smoke|down}" >&2
  exit 2
}

case "${ACTION}" in
  config)
    compose config --quiet
    ;;
  up)
    compose up --build --detach --wait
    ;;
  smoke)
    "${ROOT_DIR}/deploy/scripts/smoke-infra.sh"
    ;;
  down)
    compose down --remove-orphans
    ;;
  *)
    usage
    ;;
esac
