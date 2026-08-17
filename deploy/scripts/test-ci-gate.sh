#!/usr/bin/env bash
set -euo pipefail

# Cobertura de regressão da porta única da CI.
#
# `ci-gate.sh` decide se o pipeline inteiro passa, e até aqui não tinha teste
# algum: a única forma de exercitá-lo era empurrar commit até a CI falhar do
# jeito certo. Este script o roda contra workflows sintéticos, sem CI e sem
# Docker, de modo que uma regressão apareça no gate local.
#
# Os dois últimos cenários vêm do achado D-12: o parser não reconhecia nome de
# job com comentário na mesma linha nem entre aspas, e um job irreconhecível
# saía da expectativa — a porta passava ignorando-o. Falha ABERTA dentro da
# guarda escrita para fechar falha aberta.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="${ROOT_DIR}/deploy/scripts/ci-gate.sh"
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-ci-gate-test.XXXXXX")"
trap 'rm -rf "${WORKDIR}"' EXIT

failures=0

# Cada caso declara o que espera: `pass`, ou um trecho que a mensagem de falha
# precisa conter. Exigir o trecho é o que impede o teste de aceitar vermelho
# pelo motivo errado — a mesma disciplina de `expect_constraint_violation`.
check() {
  local name="$1"
  local workflow="$2"
  local payload="$3"
  local expectation="$4"
  local output=""
  local status=0

  output="$(GATE_WORKFLOW="${workflow}" GATE_NEEDS_JSON="${payload}" \
    bash "${GATE}" 2>&1)" || status=$?

  if test "${expectation}" = "pass"; then
    if test "${status}" -ne 0; then
      echo "FAIL ${name}: expected the gate to pass, exit=${status}: ${output}" >&2
      failures=$((failures + 1))
      return
    fi
    case "${output}" in
      *CI_GATE=PASS*) printf '  ok   %s\n' "${name}" ;;
      *) echo "FAIL ${name}: passed without the PASS marker: ${output}" >&2
         failures=$((failures + 1)) ;;
    esac
    return
  fi

  if test "${status}" -eq 0; then
    echo "FAIL ${name}: the gate passed but should have refused: ${output}" >&2
    failures=$((failures + 1))
    return
  fi
  case "${output}" in
    *"${expectation}"*) printf '  ok   %s\n' "${name}" ;;
    *) echo "FAIL ${name}: refused for the wrong reason; wanted [${expectation}], got: ${output}" >&2
       failures=$((failures + 1)) ;;
  esac
}

# Workflow canônico: três jobs mais a própria porta.
cat >"${WORKDIR}/plain.yml" <<'YAML'
name: exemplo
jobs:
  alfa:
    runs-on: ubuntu-latest
  beta:
    runs-on: ubuntu-latest
  gama:
    runs-on: ubuntu-latest
  ci:
    needs: [alfa, beta, gama]
YAML

# D-12: as duas formas que o parser antigo não enxergava.
cat >"${WORKDIR}/exotic.yml" <<'YAML'
name: exemplo
jobs:
  alfa:
    runs-on: ubuntu-latest
  beta:  # comentário na mesma linha
    runs-on: ubuntu-latest
  "gama":
    runs-on: ubuntu-latest
  ci:
    needs: [alfa, beta, gama]
YAML

# Sem job algum além da porta: a expectativa seria vazia e tudo passaria.
cat >"${WORKDIR}/empty.yml" <<'YAML'
name: exemplo
jobs:
  ci:
    needs: []
YAML

ALL='{"alfa":{"result":"success"},"beta":{"result":"success"},"gama":{"result":"success"}}'

echo "porta única da CI:"
check "todos com success"              "${WORKDIR}/plain.yml"  "${ALL}" pass
check "nomes com comentário e aspas"   "${WORKDIR}/exotic.yml" "${ALL}" pass
check "payload vazio"                  "${WORKDIR}/plain.yml"  ""       "received no needs payload"
check "objeto vazio"                   "${WORKDIR}/plain.yml"  "{}"     "zero job results"
check "payload que não é objeto"       "${WORKDIR}/plain.yml"  '["a"]'  "not an object"
check "um job faltando" "${WORKDIR}/plain.yml" \
  '{"alfa":{"result":"success"},"beta":{"result":"success"}}' "did not receive a result for: gama"
check "dois jobs faltando" "${WORKDIR}/plain.yml" \
  '{"alfa":{"result":"success"}}' "did not receive a result for: beta gama"
check "resultado vazio" "${WORKDIR}/plain.yml" \
  '{"alfa":{"result":"success"},"beta":{"result":""},"gama":{"result":"success"}}' "beta=<empty>"
check "resultado failure" "${WORKDIR}/plain.yml" \
  '{"alfa":{"result":"success"},"beta":{"result":"failure"},"gama":{"result":"success"}}' "beta=failure"
check "resultado cancelled" "${WORKDIR}/plain.yml" \
  '{"alfa":{"result":"success"},"beta":{"result":"success"},"gama":{"result":"cancelled"}}' "gama=cancelled"
check "workflow sem jobs"              "${WORKDIR}/empty.yml"  "${ALL}" "no job declared"
check "workflow inexistente"           "${WORKDIR}/ausente.yml" "${ALL}" "cannot read the workflow"

# O workflow real precisa continuar legível: se alguém acrescentar um job numa
# forma que o parser não entenda, isto quebra antes da CI.
real_jobs="$(
  GATE_WORKFLOW="${ROOT_DIR}/.github/workflows/ci.yml" GATE_NEEDS_JSON='{}' \
    bash "${GATE}" 2>&1 || true
)"
real_jobs="$(printf '%s' "${real_jobs}" | sed -n 's/.*declares \([0-9]*\);.*/\1/p')"
if test -z "${real_jobs}" || test "${real_jobs}" -lt 1; then
  echo "FAIL o parser não reconheceu job algum em .github/workflows/ci.yml" >&2
  failures=$((failures + 1))
else
  printf '  ok   workflow real legível (%s jobs)\n' "${real_jobs}"
fi

if test "${failures}" -ne 0; then
  echo "CI_GATE_REGRESSION=FAIL failures=${failures}" >&2
  exit 1
fi
echo "CI_GATE_REGRESSION=PASS"
