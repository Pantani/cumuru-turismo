#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-local-e2e-${PPID}-$$"
LOCAL_E2E_PORT="$((20000 + ($$ % 20000)))"
export LOCAL_E2E_PORT

# O subnet do e2e era fixo em 172.30.6.0/24. Duas execuções próximas pediam o
# mesmo endereço, e enquanto a rede órfã da primeira ainda estava em teardown o
# daemon recusava a segunda com "Pool overlaps with other one on this address
# space". A rede nem chegava a existir e o Playwright nunca rodava: vermelho que
# não é falha de teste, e que já custou duas investigações.
#
# A correção tem duas metades, porque uma só não basta:
#
#   1. escolher um octeto LIVRE na faixa reservada, conferindo as redes que o
#      daemon realmente tem — não sorteando e torcendo;
#   2. serializar execuções do mesmo gate com lock, no padrão de
#      test-migrations.sh. Sem o lock, duas execuções podem conferir a mesma
#      faixa no mesmo instante, ver o mesmo octeto livre e pedi-lo juntas.
#
# A faixa começa em 16 para não encostar nos octetos 0-9, todos fixados por
# overlays (base, migration-test, core, questionnaire, analytics, e2e antigo,
# local-test, dev, self-service).
E2E_SUBNET_FIRST_OCTET=16
E2E_SUBNET_LAST_OCTET=215

subnets_in_use() {
  local networks=""
  networks="$(docker network ls --quiet)" || return 1
  if test -z "${networks}"; then
    return 0
  fi
  # shellcheck disable=SC2086
  docker network inspect ${networks} \
    --format '{{range .IPAM.Config}}{{println .Subnet}}{{end}}' 2>/dev/null |
    sed -n 's#^172\.30\.\([0-9]\{1,3\}\)\.0/24$#\1#p' |
    sort -u
}

select_free_subnet_octet() {
  local span=$((E2E_SUBNET_LAST_OCTET - E2E_SUBNET_FIRST_OCTET + 1))
  local start=$((E2E_SUBNET_FIRST_OCTET + ($$ % span)))
  local used=""
  used="$(subnets_in_use)" || {
    echo "could not inspect Docker networks to pick a free subnet" >&2
    return 1
  }
  local offset=0
  local candidate=0
  while test "${offset}" -lt "${span}"; do
    candidate=$((E2E_SUBNET_FIRST_OCTET + ((start - E2E_SUBNET_FIRST_OCTET + offset) % span)))
    if ! printf '%s\n' "${used}" | grep -qx "${candidate}"; then
      printf '%s' "${candidate}"
      return 0
    fi
    offset=$((offset + 1))
  done
  echo "no free /24 between 172.30.${E2E_SUBNET_FIRST_OCTET} and 172.30.${E2E_SUBNET_LAST_OCTET}" >&2
  return 1
}

GATE_LOCK_DIR="${TMPDIR:-/tmp}/cumuru-local-demo-e2e.lock"
GATE_LOCK_HELD=false

release_gate_lock() {
  if test "${GATE_LOCK_HELD}" != "true"; then
    return
  fi
  rm -f "${GATE_LOCK_DIR}/pid"
  rmdir "${GATE_LOCK_DIR}" 2>/dev/null || true
  GATE_LOCK_HELD=false
}

acquire_gate_lock() {
  local deadline=$((SECONDS + 900))
  local owner_pid=""
  while ! mkdir "${GATE_LOCK_DIR}" 2>/dev/null; do
    if test -f "${GATE_LOCK_DIR}/pid"; then
      owner_pid="$(tr -cd '0-9' <"${GATE_LOCK_DIR}/pid")"
      if test -n "${owner_pid}" && ! kill -0 "${owner_pid}" 2>/dev/null; then
        rm -f "${GATE_LOCK_DIR}/pid"
        rmdir "${GATE_LOCK_DIR}" 2>/dev/null || true
        continue
      fi
    fi
    if test "${SECONDS}" -ge "${deadline}"; then
      echo "local demo E2E timed out waiting for the gate lock" >&2
      return 1
    fi
    sleep 1
  done
  printf '%s\n' "$$" >"${GATE_LOCK_DIR}/pid"
  GATE_LOCK_HELD=true
}

acquire_gate_lock
# Trap mínimo até o cleanup completo assumir, na linha do `trap cleanup EXIT`
# abaixo: sem ele, uma falha entre a aquisição e aquele ponto deixaria o lock
# para trás. O reaper de PID morto cobriria o caso, mas só na próxima execução.
trap release_gate_lock EXIT
LOCAL_E2E_SUBNET_OCTET="$(select_free_subnet_octet)"
export LOCAL_E2E_SUBNET_OCTET
echo "local demo E2E using 172.30.${LOCAL_E2E_SUBNET_OCTET}.0/24" >&2

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
  release_gate_lock
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
