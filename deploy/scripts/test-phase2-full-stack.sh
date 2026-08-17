#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-phase2-full-stack-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet phase2-full-stack
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.phase2-full-stack.yaml"
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
    echo "full-stack cleanup could not inspect containers" >&2
    cleanup_status=1
  elif test -n "${residual_containers}"; then
    echo "full-stack cleanup left containers behind" >&2
    cleanup_status=1
  fi
  residual_networks="$(
    docker network ls --quiet --filter "name=^${PROJECT_NAME}_private$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0; then
    echo "full-stack cleanup could not inspect networks" >&2
    cleanup_status=1
  elif test -n "${residual_networks}"; then
    echo "full-stack cleanup left its network behind" >&2
    cleanup_status=1
  fi
  residual_volumes="$(
    docker volume ls --quiet --filter "name=^${PROJECT_NAME}_postgres-data$"
  )"
  inspect_status=$?
  if test "${inspect_status}" -ne 0; then
    echo "full-stack cleanup could not inspect volumes" >&2
    cleanup_status=1
  elif test -n "${residual_volumes}"; then
    echo "full-stack cleanup left its volume behind" >&2
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
  echo "full-stack cleanup confirmed"
}
trap cleanup EXIT

require_running() {
  local service="$1"
  if ! "${COMPOSE[@]}" ps --status running --services |
    grep --fixed-strings --line-regexp --quiet "${service}"; then
    echo "full-stack service is not running: ${service}" >&2
    return 1
  fi
}

assert_log_absent() {
  local path="$1"
  local value="$2"
  if grep --fixed-strings --quiet -- "${value}" "${path}"; then
    echo "full-stack proxy log leaked synthetic capability data" >&2
    return 1
  fi
}

"${COMPOSE[@]}" up --build --detach --wait
for service in postgres api worker web; do
  require_running "${service}"
done

web_address="$("${COMPOSE[@]}" port web 8080)"
web_port="${web_address##*:}"
case "${web_port}" in
  "" | *[!0-9]*)
    echo "could not resolve the ephemeral web port" >&2
    exit 1
    ;;
esac
web_base_url="http://127.0.0.1:${web_port}"

"${COMPOSE[@]}" exec -T api \
  sh -c 'test -f /usr/share/zoneinfo/America/Bahia && TZ=America/Bahia date +%z' |
  grep --fixed-strings --line-regexp --quiet -- "-0300"

WEB_BASE_URL="${web_base_url}" \
  API_BASE_URL="${web_base_url}/api/v1" \
  "${ROOT_DIR}/deploy/scripts/smoke.sh"

service_worker="$(
  curl --fail --silent --show-error "${web_base_url}/service-worker.js"
)"
test "${service_worker}" = "$(<"${ROOT_DIR}/apps/web/public/service-worker.js")"
printf '%s' "${service_worker}" |
  grep --fixed-strings --quiet 'url.search === ""'

canary="c2-full-stack-capability-canary-7724"
query_canary="c2-full-stack-query-canary-91bf"
"${COMPOSE[@]}" stop api
status="$(
  curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
    "${web_base_url}/api/v1/invites/${canary}?token=${query_canary}"
)"
test "${status}" = "502" || test "${status}" = "504"

proxy_log="$(mktemp "${TMPDIR:-/tmp}/cumuru-full-stack-proxy.XXXXXX")"
"${COMPOSE[@]}" logs --no-color web >"${proxy_log}" 2>&1
assert_log_absent "${proxy_log}" "${canary}"
assert_log_absent "${proxy_log}" "${query_canary}"
rm -f "${proxy_log}"

"${COMPOSE[@]}" up --detach --wait api
require_running api
WEB_BASE_URL="${web_base_url}" \
  API_BASE_URL="${web_base_url}/api/v1" \
  "${ROOT_DIR}/deploy/scripts/smoke.sh"

echo "phase 2 ephemeral full-stack passed"
