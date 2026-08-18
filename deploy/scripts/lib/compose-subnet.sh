# shellcheck shell=bash
#
# Seleção verificada de sub-rede para as stacks Compose dos gates.
#
# Todo overlay fixava um /24 enquanto o nome do projeto é escopado por PID. Duas
# execuções do MESMO gate pediam o mesmo endereço, e enquanto a rede órfã da
# primeira ainda estava em teardown o daemon recusava a segunda com
# "Pool overlaps with other one on this address space" — vermelho que não é
# falha de teste. Não era só entre gates diferentes: era o mesmo gate impedido
# de rodar duas vezes.
#
# Todos os overlays declaram a MESMA rede (`private`), e o de cima vence na
# mesclagem. Por isso uma única variável, `CUMURU_SUBNET_OCTET`, serve a todos:
# quando um script empilha dois overlays que declaram a rede — o e2e empilha
# `analytics-full-stack` e `local-e2e` —, ambos resolvem para o mesmo valor em vez
# de disputarem.
#
# Cada overlay usa `${CUMURU_SUBNET_OCTET:-<octeto original>}`, então um script
# que ainda não adotou a seleção continua com o comportamento anterior.
#
# Uso:
#   . "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
#   cumuru_acquire_subnet "nome-do-gate"     # trava, escolhe, exporta
#   trap cumuru_release_subnet EXIT          # ou chame no cleanup existente

# A faixa começa em 16 para não encostar nos octetos 0-9, fixados pelos
# overlays como default.
CUMURU_SUBNET_FIRST_OCTET="${CUMURU_SUBNET_FIRST_OCTET:-16}"
CUMURU_SUBNET_LAST_OCTET="${CUMURU_SUBNET_LAST_OCTET:-215}"
CUMURU_SUBNET_LOCK_DIR=""
CUMURU_SUBNET_LOCK_HELD=false

cumuru_subnets_in_use() {
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

# Verificação, não sorteio: consulta o daemon e devolve o primeiro octeto livre
# a partir de um ponto derivado do PID. Sem nenhum livre, falha dizendo isso em
# vez de pedir um ocupado.
cumuru_select_subnet_octet() {
  local span=$((CUMURU_SUBNET_LAST_OCTET - CUMURU_SUBNET_FIRST_OCTET + 1))
  local start=$((CUMURU_SUBNET_FIRST_OCTET + ($$ % span)))
  local used=""
  used="$(cumuru_subnets_in_use)" || {
    echo "could not inspect Docker networks to pick a free subnet" >&2
    return 1
  }
  local offset=0
  local candidate=0
  while test "${offset}" -lt "${span}"; do
    candidate=$((CUMURU_SUBNET_FIRST_OCTET + ((start - CUMURU_SUBNET_FIRST_OCTET + offset) % span)))
    if ! printf '%s\n' "${used}" | grep -qx "${candidate}"; then
      printf '%s' "${candidate}"
      return 0
    fi
    offset=$((offset + 1))
  done
  echo "no free /24 between 172.30.${CUMURU_SUBNET_FIRST_OCTET} and 172.30.${CUMURU_SUBNET_LAST_OCTET}" >&2
  return 1
}

cumuru_release_subnet() {
  if test "${CUMURU_SUBNET_LOCK_HELD}" != "true"; then
    return
  fi
  rm -f "${CUMURU_SUBNET_LOCK_DIR}/pid"
  rmdir "${CUMURU_SUBNET_LOCK_DIR}" 2>/dev/null || true
  CUMURU_SUBNET_LOCK_HELD=false
}

# D-19: a janela vulnerável fica entre esta chamada e o `trap cleanup EXIT`
# que o script chamador instala depois — ali o único trap ativo é o mínimo
# abaixo. Tentei primeiro preservar `$?` dentro do handler (capturar em
# `local status="$?"` e terminar com `return "${status}"`, para que o último
# comando do trap seja o que define o status de saída do processo). Não
# resolve: sob bash 3.2 (o bash padrão do macOS, o mesmo que roda estes
# scripts fora do container) com `set -e` ativo, um erro de PARSE (não de
# runtime) já zera `$?` para 0 antes do trap começar a rodar — o handler nunca
# vê o 2 do parser, então não há status para preservar. Reproduzido com:
#   set -euo pipefail; trap 't() { echo $?; }; t' EXIT; fi
# que imprime `status=0` no bash 3.2 e `status=2` no bash 5. É bug conhecido
# de versões antigas do bash nessa combinação específica (parser abort +
# errexit + trap), não algo corrigível só lendo `$?` dentro do handler.
#
# A correção real está em cumuru_acquire_subnet: validar a sintaxe do script
# inteiro ANTES de instalar qualquer trap ou adquirir o lock. `bash -n` é uma
# checagem estática — enxerga erro em `cleanup()` mesmo que essa função ainda
# não tenha sido chamada — então nenhum abort de parse pode mais acontecer
# dentro da janela vulnerável: o script já teria sido recusado antes dela
# começar, com um status real e sem trap nenhum envolvido.
cumuru_release_subnet_exit_trap() {
  cumuru_release_subnet
}

# O lock serializa execuções do mesmo gate. Só a verificação continuaria
# probabilística: duas execuções podem consultar o daemon no mesmo instante,
# ver o mesmo octeto livre e pedi-lo juntas. Ele é devolvido depois do teardown,
# que é o que impede a execução seguinte de pedir endereço ainda em liberação.
cumuru_acquire_subnet() {
  local gate="$1"
  if ! bash -n "$0" 2>&1; then
    echo "${gate}: $0 has a bash syntax error; refusing to acquire the subnet lock or install any trap" >&2
    return 1
  fi
  CUMURU_SUBNET_LOCK_DIR="${TMPDIR:-/tmp}/cumuru-subnet-${gate}.lock"
  local deadline=$((SECONDS + 900))
  local owner_pid=""
  while ! mkdir "${CUMURU_SUBNET_LOCK_DIR}" 2>/dev/null; do
    if test -f "${CUMURU_SUBNET_LOCK_DIR}/pid"; then
      owner_pid="$(tr -cd '0-9' <"${CUMURU_SUBNET_LOCK_DIR}/pid")"
      if test -n "${owner_pid}" && ! kill -0 "${owner_pid}" 2>/dev/null; then
        rm -f "${CUMURU_SUBNET_LOCK_DIR}/pid"
        rmdir "${CUMURU_SUBNET_LOCK_DIR}" 2>/dev/null || true
        continue
      fi
    fi
    if test "${SECONDS}" -ge "${deadline}"; then
      echo "${gate} timed out waiting for the subnet lock" >&2
      return 1
    fi
    sleep 1
  done
  printf '%s\n' "$$" >"${CUMURU_SUBNET_LOCK_DIR}/pid"
  CUMURU_SUBNET_LOCK_HELD=true
  # Trap mínimo até o cleanup completo do script assumir.
  trap cumuru_release_subnet_exit_trap EXIT
  CUMURU_SUBNET_OCTET="$(cumuru_select_subnet_octet)"
  export CUMURU_SUBNET_OCTET
  echo "${gate} using 172.30.${CUMURU_SUBNET_OCTET}.0/24" >&2
}
