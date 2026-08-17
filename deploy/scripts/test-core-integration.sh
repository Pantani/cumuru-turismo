#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-core-integration-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet core-integration
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.core-integration.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans
  cleanup_status=$?
  residual_containers="$(
    docker container ls --all --quiet \
      --filter "label=com.docker.compose.project=${PROJECT_NAME}"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0; then
    echo "core integration cleanup could not inspect containers" >&2
    cleanup_status=1
  elif test -n "${residual_containers}"; then
    echo "core integration cleanup left containers behind" >&2
    cleanup_status=1
  fi
  residual_networks="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0; then
    echo "core integration cleanup could not inspect networks" >&2
    cleanup_status=1
  elif test -n "${residual_networks}"; then
    echo "core integration cleanup left its network behind" >&2
    cleanup_status=1
  fi
  residual_volumes="$(
    docker volume ls --quiet --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0; then
    echo "core integration cleanup could not inspect volumes" >&2
    cleanup_status=1
  elif test -n "${residual_volumes}"; then
    echo "core integration cleanup left its volume behind" >&2
    cleanup_status=1
  fi
  set -e
  # O lock só cai depois do teardown: é isso que impede a execução seguinte de
  # pedir um endereço que ainda está sendo liberado.
  cumuru_release_subnet
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    exit "${cleanup_status}"
  fi
  echo "core integration cleanup confirmed"
}
trap cleanup EXIT

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate

published_address="$("${COMPOSE[@]}" port postgres 5432)"
database_port="${published_address##*:}"
case "${database_port}" in
  "" | *[!0-9]*)
    echo "could not resolve the ephemeral PostgreSQL port" >&2
    exit 1
    ;;
esac

admin_dsn="postgres://cumuru_migration:cumuru-local-migration-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"
runtime_dsn="postgres://cumuru_app:cumuru-local-app-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"

CUMURU_TEST_ADMIN_DATABASE_URL="${admin_dsn}" \
  CUMURU_TEST_DATABASE_URL="${runtime_dsn}" \
  go -C "${ROOT_DIR}/apps/api" test \
  -tags=integration -race -count=1 ./internal/platform/store

echo "core PostgreSQL integration passed with cumuru_app runtime role"
