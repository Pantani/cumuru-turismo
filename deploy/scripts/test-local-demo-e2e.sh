#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-local-e2e-${PPID}-$$"
LOCAL_E2E_PORT="$((20000 + ($$ % 20000)))"
export LOCAL_E2E_PORT

. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet local-demo-e2e

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

# Compose lê `.env` do diretório do projeto, e o serviço de seed exige as
# credenciais fictícias do demo local. Em clone limpo — e na CI — esse arquivo
# não existe, então o exemplo versionado entra como origem explícita. Um `.env`
# do desenvolvedor continua tendo precedência.
LOCAL_ENV_FILE="${ROOT_DIR}/.env"
test -f "${LOCAL_ENV_FILE}" || LOCAL_ENV_FILE="${ROOT_DIR}/.env.example"

COMPOSE=(
  docker compose
  --env-file "${LOCAL_ENV_FILE}"
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/compose.local.yaml"
  --file "${ROOT_DIR}/deploy/compose.analytics-full-stack.yaml"
  --file "${ROOT_DIR}/deploy/compose.local-e2e.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local status=$?
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
  )" || inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    cleanup_status=1
  fi
  inspect_status=0
  residual="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )" || inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    cleanup_status=1
  fi
  inspect_status=0
  residual="$(
    docker volume ls --quiet \
      --filter "name=^${PROJECT_NAME}_postgres-data$"
  )" || inspect_status=$?
  if test "${inspect_status}" -ne 0 || test -n "${residual}"; then
    cleanup_status=1
  fi
  set -e
  # O lock só cai depois do teardown: é isso que impede a execução seguinte de
  # pedir um endereço que ainda está sendo liberado.
  cumuru_release_subnet
  if test "${status}" -ne 0; then
    exit "${status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    echo "local demo E2E cleanup failed" >&2
    exit "${cleanup_status}"
  fi
}
trap cleanup EXIT

"${COMPOSE[@]}" up --build --detach --wait web

LOCAL_E2E_BASE_URL="http://127.0.0.1:${LOCAL_E2E_PORT}" \
  npx playwright test \
  --config "${ROOT_DIR}/deploy/playwright.local.config.ts"

echo "LOCAL_DEMO_BROWSER_E2E=PASS"
