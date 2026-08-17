#!/usr/bin/env bash
set -euo pipefail

# Porta única da CI.
#
# A versão anterior era um laço sobre `join(needs.*.result, ' ')`. Com a lista
# vazia — coleta quebrada, job renomeado, contexto ausente — o laço não executava
# nenhuma iteração e a porta imprimia `CI_GATE=PASS` sobre conjunto vazio. É a
# mesma forma que esta fase combateu no e2e com
# `expect(documentPolicies.length).toBeGreaterThan(0)`: ausência de evidência
# lida como evidência de ausência de problema.
#
# A expectativa não pode ser constante, senão quem acrescenta um job esquece de
# atualizá-la e a porta passa a ignorá-lo em silêncio. Ela é derivada do próprio
# workflow: **todo job declarado em `jobs:`, exceto a própria porta, tem de
# aparecer nos resultados recebidos**. Acrescentar job sem ligá-lo ao `needs:`
# passa a ser vermelho.
#
# A lógica vive aqui, e não embutida no YAML, para ser exercitável fora da CI.

GATE_WORKFLOW="${GATE_WORKFLOW:-.github/workflows/ci.yml}"
GATE_JOB_ID="${GATE_JOB_ID:-ci}"

fail() {
  echo "$1" >&2
  echo "CI_GATE=FAIL" >&2
  exit 1
}

# Jobs declarados no workflow: chaves de dois espaços sob `jobs:`, parando na
# próxima chave de coluna zero.
# O parser precisa reconhecer as três formas que YAML permite para a mesma
# chave, senão um job que ele não sabe ler sai da expectativa e a porta passa a
# ignorá-lo — falha ABERTA dentro da guarda escrita para fechar falha aberta:
#
#   alfa:
#   beta:   # comentário na mesma linha
#   "gama":
#
# Aspas simples e duplas são aceitas; comentário à direita é descartado.
declared_jobs() {
  awk '
    /^jobs:[[:space:]]*$/ { inside = 1; next }
    inside && /^[^[:space:]#]/ { inside = 0 }
    inside && /^  ["'"'"']?[A-Za-z0-9_.-]+["'"'"']?:([[:space:]]*(#.*)?)?$/ {
      sub(/^  /, "")
      sub(/:.*$/, "")
      gsub(/^["'"'"']|["'"'"']$/, "")
      print
    }
  ' "${GATE_WORKFLOW}" | { grep -vx "${GATE_JOB_ID}" || true; } | sort
}

if ! test -f "${GATE_WORKFLOW}"; then
  fail "the gate cannot read the workflow at ${GATE_WORKFLOW}"
fi

expected="$(declared_jobs)"
if test -z "${expected}"; then
  fail "the gate found no job declared in ${GATE_WORKFLOW}; the expectation would be vacuous"
fi

if test -z "${GATE_NEEDS_JSON:-}"; then
  fail "the gate received no needs payload: GATE_NEEDS_JSON is empty"
fi

if ! printf '%s' "${GATE_NEEDS_JSON}" | jq -e 'type == "object"' >/dev/null 2>&1; then
  fail "the gate received a needs payload that is not an object"
fi

received="$(printf '%s' "${GATE_NEEDS_JSON}" | jq -r 'keys[]' | sort)"
if test -z "${received}"; then
  fail "the gate received zero job results while ${GATE_WORKFLOW} declares $(printf '%s\n' "${expected}" | wc -l | tr -d ' '); refusing to pass on an empty set"
fi

# Cada job declarado precisa ter reportado. Sem isto, remover um job do `needs:`
# o tiraria silenciosamente da porta.
missing=""
for job in ${expected}; do
  if ! printf '%s\n' "${received}" | grep -qx "${job}"; then
    missing="${missing} ${job}"
  fi
done
if test -n "${missing}"; then
  fail "the gate did not receive a result for:${missing}"
fi

# Resultado vazio é tão inaceitável quanto resultado ruim: era por onde o laço
# antigo passava batido.
failures=""
for job in ${received}; do
  result="$(printf '%s' "${GATE_NEEDS_JSON}" | jq -r --arg job "${job}" '.[$job].result // ""')"
  if test -z "${result}"; then
    failures="${failures} ${job}=<empty>"
  elif test "${result}" != "success"; then
    failures="${failures} ${job}=${result}"
  fi
done
if test -n "${failures}"; then
  fail "the gate refuses these results:${failures}"
fi

echo "CI_GATE=PASS jobs=$(printf '%s\n' "${received}" | wc -l | tr -d ' ')"
