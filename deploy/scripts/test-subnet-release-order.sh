#!/usr/bin/env bash
#
# Prova que nenhum gate libera o lock de sub-rede antes de capturar o status da
# execução.
#
# `cumuru_release_subnet` sempre devolve 0 — o caminho sem lock faz `return` e o
# caminho com lock termina em `CUMURU_SUBNET_LOCK_HELD=false`. Um `cleanup()`
# que a chame ANTES de `local primary_status=$?` grava 0 em `primary_status` e
# passa a sair com o status do teardown, engolindo qualquer falha real do teste.
# Foi exatamente o que aconteceu em onze gates, e o defeito é invisível: os
# scripts continuam verdes, só param de conseguir ficar vermelhos.
#
# Este teste não roda Docker e não sobe nada; é estático mais uma verificação do
# valor de retorno da função.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/deploy/scripts"

STATUS_CAPTURE='^[[:space:]]*local[[:space:]]+[a-z_]+=\$\?[[:space:]]*$'
RELEASE_CALL='^[[:space:]]*cumuru_release_subnet[[:space:]]*$'

failures=0

fail() {
  echo "SUBNET_RELEASE_ORDER: $*" >&2
  failures=$((failures + 1))
}

# Primeira linha que casa com o padrão, ou vazio.
first_line() {
  grep -nE "$2" "$1" | head -n 1 | cut -d: -f1
}

check_script() {
  local file="$1"
  local name="${file##*/}"
  local release_line=""
  local status_line=""

  release_line="$(first_line "${file}" "${RELEASE_CALL}")"
  if test -z "${release_line}"; then
    fail "${name} adquire a sub-rede e nunca chama cumuru_release_subnet"
    return
  fi

  status_line="$(first_line "${file}" "${STATUS_CAPTURE}")"
  if test -z "${status_line}"; then
    fail "${name} não captura \$? em nenhum 'local <nome>=\$?'"
    return
  fi

  if test "${status_line}" -ge "${release_line}"; then
    fail "${name}: captura de \$? na linha ${status_line} vem depois da" \
      "liberação do lock na linha ${release_line}; mova a captura para a" \
      "primeira linha do cleanup"
    return
  fi

  echo "  ok ${name} (captura L${status_line} < liberação L${release_line})"
}

# `cumuru_release_subnet` devolvendo 0 é o que torna a ordem obrigatória. Se
# algum dia ela passar a propagar erro, este teste avisa que a premissa mudou.
check_release_always_zero() {
  local lock_dir=""
  lock_dir="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-release-order.XXXXXX")"

  # shellcheck source=deploy/scripts/lib/compose-subnet.sh
  . "${SCRIPT_DIR}/lib/compose-subnet.sh"

  CUMURU_SUBNET_LOCK_HELD=false
  cumuru_release_subnet ||
    fail "cumuru_release_subnet falhou sem lock; premissa do teste mudou"

  CUMURU_SUBNET_LOCK_DIR="${lock_dir}"
  CUMURU_SUBNET_LOCK_HELD=true
  printf '%s\n' "$$" >"${lock_dir}/pid"
  cumuru_release_subnet ||
    fail "cumuru_release_subnet falhou com lock; premissa do teste mudou"

  rm -rf -- "${lock_dir}"
  trap - EXIT
}

scripts=()
while IFS= read -r file; do
  scripts+=("${file}")
done < <(grep -lE '^[[:space:]]*cumuru_acquire_subnet[[:space:]]' \
  "${SCRIPT_DIR}"/*.sh | sort)

if test "${#scripts[@]}" -eq 0; then
  echo "SUBNET_RELEASE_ORDER: nenhum gate usa cumuru_acquire_subnet" >&2
  exit 1
fi

for script in "${scripts[@]}"; do
  check_script "${script}"
done

check_release_always_zero

if test "${failures}" -ne 0; then
  echo "SUBNET_RELEASE_ORDER=FAIL failures=${failures}" >&2
  exit 1
fi

echo "SUBNET_RELEASE_ORDER=PASS scripts=${#scripts[@]}"
