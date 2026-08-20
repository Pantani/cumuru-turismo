#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(CDPATH='' cd -- "${SCRIPT_DIR}/../../../.." && pwd)"
PROMPT_FILE="${PROJECT_ROOT}/prompts/BOOTSTRAP-CODEX.md"
WORKSPACE_ROOT="${CUMURU_HARNESS_WORKSPACE_ROOT:-${PROJECT_ROOT}/_workspace/cumuru-bootstrap}"

usage() {
  echo "usage: harness.sh {validate|status|prompt|phase|dry-run|snapshot} [phase] [attempt-id]"
}

normalize_phase() {
  local value="${1:-}"
  case "${value}" in
    1 | 01) echo "1" ;;
    2 | 02) echo "2" ;;
    3 | 03) echo "3" ;;
    4 | 04) echo "4" ;;
    5 | 05) echo "5" ;;
    6 | 06 | 6A | 6a | 06A | 06a) echo "6" ;;
    7 | 07) echo "7" ;;
    8 | 08) echo "8" ;;
    6B | 6b | 06B | 06b)
      echo "phase 6B is an operational pilot and is not executable by Prompt 6" >&2
      return 2
      ;;
    *)
      echo "invalid phase: ${value}" >&2
      return 2
      ;;
  esac
}

phase_title() {
  case "$1" in
    1) echo "Fundacao tecnica" ;;
    2) echo "Hospedagens e estadias" ;;
    3) echo "Questionario" ;;
    4) echo "Analytics e dashboard" ;;
    5) echo "FNRH piloto" ;;
    6) echo "6A Auditoria de prontidao" ;;
    7) echo "Autoatendimento e aprovacao" ;;
    8) echo "Contexto externo" ;;
  esac
}

phase_dependencies() {
  case "$1" in
    1) echo "Fase 0 PASS ou PROTOTYPE_ONLY" ;;
    2) echo "Fase 1 PASS" ;;
    3) echo "Fase 2 PASS" ;;
    4) echo "Fase 3 PASS" ;;
    5) echo "Fase 4 PASS + cinco gates externos FNRH" ;;
    6) echo "Fases 1-4 PASS; Fase 5 PASS ou BLOCKED documentado" ;;
    7) echo "Fases 2 e 3 PASS; gate SELF_SERVICE_LEGAL_BASIS para dados reais" ;;
    8) echo "Fases 1 e 4 PASS; gate EXTERNAL_SOURCE_LICENSE por fonte" ;;
  esac
}

status_for_phase() {
  local phase="$1"
  local path="${WORKSPACE_ROOT}/phase-${phase}/status.txt"
  local qa_path="${WORKSPACE_ROOT}/phase-${phase}/qa.md"
  local status
  if [[ ! -f "${path}" ]]; then
    echo "PENDING"
    return
  fi
  status="$(tr -d '[:space:]' <"${path}")"
  case "${status}" in
    PASS | FAIL | BLOCKED | UNVERIFIED | N/A)
      if [[ ! -s "${qa_path}" ]] ||
        [[ "$(grep -c "^GATE_STATUS=${status}$" "${qa_path}")" != "1" ]] ||
        [[ "$(grep -c '^GATE_STATUS=' "${qa_path}")" != "1" ]] ||
        [[ "$(awk 'NF { line = $0 } END { print line }' "${qa_path}")" != "GATE_STATUS=${status}" ]] ||
        ! grep -q '^## Requirements$' "${qa_path}" ||
        ! grep -q '^## Commands$' "${qa_path}" ||
        ! grep -q '^## Risks$' "${qa_path}"; then
        echo "INVALID(EVIDENCE_MISMATCH)"
      else
        echo "${status}"
      fi
      ;;
    *) echo "INVALID(UNKNOWN_STATUS)" ;;
  esac
}

governance_status() {
  local path="${WORKSPACE_ROOT}/governance-status.txt"
  local evidence_path="${WORKSPACE_ROOT}/governance.md"
  local status
  if [[ ! -f "${path}" ]]; then
    echo "UNVERIFIED"
    return
  fi
  status="$(tr -d '[:space:]' <"${path}")"
  case "${status}" in
    PASS | PROTOTYPE_ONLY | BLOCKED | UNVERIFIED)
      if [[ ! -s "${evidence_path}" ]] ||
        ! grep -q "^GOVERNANCE_STATUS=${status}$" "${evidence_path}"; then
        echo "INVALID"
      else
        echo "${status}"
      fi
      ;;
    *) echo "INVALID" ;;
  esac
}

evidence_is_contained() {
  local relative="$1"
  local component
  local current="${WORKSPACE_ROOT}"
  local -a components

  if [[ ! -d "${WORKSPACE_ROOT}" || -L "${WORKSPACE_ROOT}" ]]; then
    return 1
  fi
  IFS='/' read -r -a components <<<"${relative}"
  for component in "${components[@]}"; do
    if [[ -z "${component}" || "${component}" == "." || "${component}" == ".." ]]; then
      return 1
    fi
    current="${current}/${component}"
    if [[ -L "${current}" ]]; then
      return 1
    fi
  done
  [[ -s "${current}" ]]
}

gates_status_for() {
  local phase="$1"
  shift
  local path="${WORKSPACE_ROOT}/phase-${phase}/external-gates.env"
  local evidence
  local evidence_path
  local gate
  if [[ ! -f "${path}" ]]; then
    echo "BLOCKED"
    return
  fi
  for gate in "$@"; do
    if [[ "$(grep -c "^${gate}=" "${path}")" != "1" ]] ||
      ! grep -q "^${gate}=PASS$" "${path}" ||
      [[ "$(grep -c "^${gate}_EVIDENCE=" "${path}")" != "1" ]]; then
      echo "BLOCKED"
      return
    fi
    evidence="$(sed -n "s/^${gate}_EVIDENCE=//p" "${path}")"
    case "${evidence}" in
      "" | /* | *..*)
        echo "BLOCKED"
        return
        ;;
    esac
    evidence_path="${WORKSPACE_ROOT}/${evidence}"
    if [[ ! -s "${evidence_path}" ]] ||
      ! evidence_is_contained "${evidence}"; then
      echo "BLOCKED"
      return
    fi
  done
  echo "PASS"
}

external_gates_status() {
  gates_status_for 5 \
    AUTHORIZATION \
    CURRENT_DOCS \
    HOMOLOGATION \
    PER_ACCOMMODATION_CREDENTIALS \
    APPROVED_MAPPING
}

# Phase 7 collects full identity submitted by a third party about a possibly
# absent data subject. The gate governs real data only; the prototype stays
# implementable with fictional data and reports REAL_DATA=BLOCKED.
self_service_gate_status() {
  gates_status_for 7 SELF_SERVICE_LEGAL_BASIS
}

# Phase 8 publishes third-party data. The licence gate is per source and is not
# waived by PROTOTYPE_ONLY, because CC-BY attribution does not depend on the
# software being a prototype. The tide gate is a rights gate, not a data gate:
# the CHM harmonic constants require written permission to publish derived
# predictions, so the card stays unavailable until a human obtains it.
external_context_gate_status() {
  gates_status_for 8 EXTERNAL_SOURCE_LICENSE TIDE_HARMONIC_CONSTANTS
}

scm_state() {
  if git -C "${PROJECT_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "PRESENT"
  else
    echo "ABSENT"
  fi
}

eligibility_for_phase() {
  local phase="$1"
  local governance
  local phase_five
  local previous
  if [[ "${phase}" == "1" ]]; then
    governance="$(governance_status)"
    case "${governance}" in
      PASS) echo "ELIGIBLE" ;;
      PROTOTYPE_ONLY) echo "ELIGIBLE_PROTOTYPE_ONLY" ;;
      *) echo "BLOCKED" ;;
    esac
    return
  fi
  if [[ "${phase}" == "6" ]]; then
    if [[ "$(status_for_phase 1)" == "PASS" &&
    "$(status_for_phase 2)" == "PASS" &&
    "$(status_for_phase 3)" == "PASS" &&
    "$(status_for_phase 4)" == "PASS" ]]; then
      phase_five="$(status_for_phase 5)"
      case "${phase_five}" in
        PASS) echo "ELIGIBLE" ;;
        BLOCKED) echo "ELIGIBLE_WITH_PHASE5_BLOCKED" ;;
        *) echo "BLOCKED" ;;
      esac
    else
      echo "BLOCKED"
    fi
    return
  fi
  if [[ "${phase}" == "7" ]]; then
    if [[ "$(status_for_phase 2)" != "PASS" ||
    "$(status_for_phase 3)" != "PASS" ]]; then
      echo "BLOCKED"
      return
    fi
    case "$(governance_status)" in
      PASS) echo "ELIGIBLE" ;;
      PROTOTYPE_ONLY) echo "ELIGIBLE_PROTOTYPE_ONLY" ;;
      *) echo "BLOCKED" ;;
    esac
    return
  fi
  if [[ "${phase}" == "8" ]]; then
    if [[ "$(status_for_phase 1)" != "PASS" ||
    "$(status_for_phase 4)" != "PASS" ]]; then
      echo "BLOCKED"
      return
    fi
    case "$(governance_status)" in
      PASS) echo "ELIGIBLE" ;;
      PROTOTYPE_ONLY) echo "ELIGIBLE_PROTOTYPE_ONLY" ;;
      *) echo "BLOCKED" ;;
    esac
    return
  fi
  previous="$((phase - 1))"
  if [[ "$(status_for_phase "${previous}")" == "PASS" ]]; then
    if [[ "${phase}" == "5" ]]; then
      if [[ "$(governance_status)" == "PASS" &&
      "$(external_gates_status)" == "PASS" ]]; then
        echo "ELIGIBLE"
      else
        echo "BLOCKED"
      fi
    else
      governance="$(governance_status)"
      case "${governance}" in
        PASS) echo "ELIGIBLE" ;;
        PROTOTYPE_ONLY) echo "ELIGIBLE_PROTOTYPE_ONLY" ;;
        *) echo "BLOCKED" ;;
      esac
    fi
  else
    echo "BLOCKED"
  fi
}

extract_prompt() {
  local phase="$1"
  awk -v phase="${phase}" '
    BEGIN { section = 0; block = 0; complete = 0 }
    $0 ~ "^## Prompt " phase " " { section = 1; next }
    section && !block && $0 ~ "^## Prompt [1-8] " { exit 4 }
    section && $0 == "```text" { block = 1; next }
    block && $0 == "```" { complete = 1; exit }
    block { print }
    END {
      if (!section || !complete) {
        exit 3
      }
    }
  ' "${PROMPT_FILE}"
}

validate_frontmatter() {
  local file="$1"
  local expected_name="$2"
  [[ "$(sed -n '1p' "${file}")" == "---" ]]
  [[ "$(grep -c '^---$' "${file}")" -ge 2 ]]
  grep -q "^name: ${expected_name}$" "${file}"
  grep -Eq '^description: .+' "${file}"
}

make_target_body() {
  local target="$1"
  awk -v target="${target}" '
    /^[^[:space:]#][^:]*:/ {
      if (capture) {
        exit
      }
      name = $0
      sub(/:.*/, "", name)
      capture = (name == target)
    }
    capture { print }
  ' "${PROJECT_ROOT}/Makefile"
}

require_completion_marker() {
  local file="$1"
  grep -Fq 'make post-task-quality' "${file}" &&
    grep -Fq 'POST_TASK_QUALITY=PASS' "${file}"
}

validate_completion_contract() {
  local body
  local file
  local writer
  local shell_lint_script
  local shell_lint_body

  for writer in backend frontend platform; do
    for file in \
      "${PROJECT_ROOT}/.codex/agents/cumuru-${writer}-builder.toml" \
      "${PROJECT_ROOT}/.claude/agents/cumuru-${writer}-builder.md"; do
      require_completion_marker "${file}" || {
        echo "POST_TASK_AGENT_DRIFT=${file}" >&2
        return 1
      }
    done
  done

  for file in \
    "${PROJECT_ROOT}/AGENTS.md" \
    "${PROJECT_ROOT}/README.md" \
    "${PROJECT_ROOT}/CLAUDE.md" \
    "${PROJECT_ROOT}/docs/decisoes/ADR-024-gate-global-de-conclusao-pos-tarefa.md" \
    "${PROJECT_ROOT}/.agents/skills/cumuru-bootstrap/SKILL.md" \
    "${PROJECT_ROOT}/.agents/skills/cumuru-bootstrap/references/execution-protocol.md" \
    "${PROJECT_ROOT}/.agents/skills/cumuru-phase-orchestrator/SKILL.md" \
    "${PROJECT_ROOT}/.agents/skills/cumuru-integration-qa/SKILL.md" \
    "${PROJECT_ROOT}/.codex/agents/cumuru-integration-qa.toml" \
    "${PROJECT_ROOT}/.claude/agents/cumuru-integration-qa.md"; do
    require_completion_marker "${file}" || {
      echo "POST_TASK_CONTRACT_DRIFT=${file}" >&2
      return 1
    }
  done

  body="$(make_target_body post-task-quality)"
  grep -Fxq $'\t@$(MAKE) --no-print-directory complexity' <<<"${body}"
  grep -Fxq $'\t@$(MAKE) --no-print-directory lint' <<<"${body}"
  grep -Fxq $'\t@echo "POST_TASK_QUALITY=PASS"' <<<"${body}"
  if ! awk '
    /--no-print-directory complexity/ { complexity = NR }
    /--no-print-directory lint/ { lint = NR }
    /POST_TASK_QUALITY=PASS/ { marker = NR }
    END { exit !(complexity && complexity < lint && lint < marker) }
  ' <<<"${body}"; then
    echo "POST_TASK_QUALITY_ORDER_DRIFT=FAIL" >&2
    return 1
  fi

  body="$(make_target_body lint-shell)"
  shell_lint_script="${PROJECT_ROOT}/deploy/scripts/lint-shell.sh"
  grep -Fxq $'\t@bash deploy/scripts/lint-shell.sh' <<<"${body}"
  test -f "${shell_lint_script}"
  shell_lint_body="$(cat "${shell_lint_script}")"
  grep -Fq 'set -euo pipefail' <<<"${shell_lint_body}"
  grep -Fq "git ls-files -z --cached --others --exclude-standard -- '*.sh'" \
    <<<"${shell_lint_body}"
  grep -Fq 'while IFS= read -r -d ' <<<"${shell_lint_body}"
  grep -Fq $'bash -n "${file}"' <<<"${shell_lint_body}"
  grep -Fq $'test "${count}" -gt 0' <<<"${shell_lint_body}"
  grep -Fq 'SHELL_SYNTAX=PASS' <<<"${shell_lint_body}"
  if grep -Eq '(^|[[:space:]])(\|\|[[:space:]]+true|-[[:space:]]*@?(bash|git))' \
    <<<"${body}"$'\n'"${shell_lint_body}"; then
    echo "SHELL_LINT_FAIL_OPEN_DRIFT=FAIL" >&2
    return 1
  fi
  body="$(make_target_body lint)"
  grep -Eq '^lint:[[:space:]]+lint-shell([[:space:]]+##[[:space:]].*)?$' <<<"${body}"

  body="$(make_target_body ci)"
  grep -Fxq $'\t@$(MAKE) --no-print-directory post-task-quality' <<<"${body}"
  if grep -Eq '\$\(MAKE\).* (complexity|lint)([[:space:]]|$)' <<<"${body}"; then
    echo "MAKE_CI_QUALITY_DRIFT=separate-complexity-or-lint" >&2
    return 1
  fi
  if ! awk '
    /--no-print-directory typecheck/ { typecheck = NR }
    /--no-print-directory post-task-quality/ { quality = NR }
    /--no-print-directory build/ { build = NR }
    END { exit !(typecheck && typecheck < quality && quality < build) }
  ' <<<"${body}"; then
    echo "MAKE_CI_QUALITY_ORDER_DRIFT=FAIL" >&2
    return 1
  fi

  grep -Eq '^[[:space:]]+(run:[[:space:]]+)?make post-task-quality[[:space:]]*$' \
    "${PROJECT_ROOT}/.github/workflows/ci.yml"
  if grep -Eq '^[[:space:]]+(run:[[:space:]]+)?make (complexity|lint)[[:space:]]*$' \
    "${PROJECT_ROOT}/.github/workflows/ci.yml"; then
    echo "CI_QUALITY_DRIFT=separate-complexity-or-lint" >&2
    return 1
  fi

  echo "POST_TASK_QUALITY_CONTRACT=PASS"
}

validate_harness() {
  local required_files=(
    "AGENTS.md"
    "CLAUDE.md"
    "docs/decisoes/ADR-011-harness-codex-faseado.md"
    "docs/decisoes/ADR-024-gate-global-de-conclusao-pos-tarefa.md"
    ".codex/config.toml"
    ".agents/skills/cumuru-bootstrap/SKILL.md"
    ".agents/skills/cumuru-bootstrap/references/phase-matrix.md"
    ".agents/skills/cumuru-bootstrap/references/execution-protocol.md"
    ".agents/skills/cumuru-bootstrap/references/trigger-evals.md"
    ".agents/skills/cumuru-phase-orchestrator/SKILL.md"
    ".agents/skills/cumuru-integration-qa/SKILL.md"
    ".claude/skills/cumuru-bootstrap/SKILL.md"
    ".claude/skills/cumuru-phase-orchestrator/SKILL.md"
    ".claude/skills/cumuru-integration-qa/SKILL.md"
  )
  local agent
  local claude_agents
  local codex_agents
  local file
  local phase
  local prompt_count

  for agent in \
    cumuru-phase-analyst \
    cumuru-contract-reviewer \
    cumuru-privacy-reviewer \
    cumuru-compliance-gatekeeper \
    cumuru-backend-builder \
    cumuru-frontend-builder \
    cumuru-platform-builder \
    cumuru-integration-qa; do
    required_files+=(".codex/agents/${agent}.toml")
    required_files+=(".claude/agents/${agent}.md")
  done

  for file in "${required_files[@]}"; do
    if [[ ! -f "${PROJECT_ROOT}/${file}" ]]; then
      echo "MISSING=${file}" >&2
      return 1
    fi
  done

  validate_frontmatter \
    "${PROJECT_ROOT}/.agents/skills/cumuru-bootstrap/SKILL.md" \
    "cumuru-bootstrap"
  validate_frontmatter \
    "${PROJECT_ROOT}/.agents/skills/cumuru-phase-orchestrator/SKILL.md" \
    "cumuru-phase-orchestrator"
  validate_frontmatter \
    "${PROJECT_ROOT}/.agents/skills/cumuru-integration-qa/SKILL.md" \
    "cumuru-integration-qa"
  validate_frontmatter \
    "${PROJECT_ROOT}/.claude/skills/cumuru-bootstrap/SKILL.md" \
    "cumuru-bootstrap"
  validate_frontmatter \
    "${PROJECT_ROOT}/.claude/skills/cumuru-phase-orchestrator/SKILL.md" \
    "cumuru-phase-orchestrator"
  validate_frontmatter \
    "${PROJECT_ROOT}/.claude/skills/cumuru-integration-qa/SKILL.md" \
    "cumuru-integration-qa"

  for file in "${PROJECT_ROOT}"/.codex/agents/*.toml; do
    grep -q '^name = ' "${file}"
    grep -q '^description = ' "${file}"
    grep -q '^developer_instructions = """' "${file}"
  done

  for file in "${PROJECT_ROOT}"/.claude/agents/*.md; do
    validate_frontmatter "${file}" "$(basename "${file}" .md)"
    grep -q '^model: opus$' "${file}"
    grep -q '^## Papel central$' "${file}"
    grep -q '^## Entrada e saída$' "${file}"
    grep -q '^## Protocolo de equipe$' "${file}"
    grep -q '^## Erros$' "${file}"
    grep -q '^## Colaboração$' "${file}"
  done

  codex_agents="$(
    find "${PROJECT_ROOT}/.codex/agents" -maxdepth 1 -type f -name '*.toml' \
      -exec basename {} .toml \; | sort
  )"
  claude_agents="$(
    find "${PROJECT_ROOT}/.claude/agents" -maxdepth 1 -type f -name '*.md' \
      -exec basename {} .md \; | sort
  )"
  if [[ "${codex_agents}" != "${claude_agents}" ]]; then
    echo "AGENT_SURFACE_DRIFT=FAIL" >&2
    return 1
  fi

  if [[ -d "${PROJECT_ROOT}/.claude/commands" ]] &&
    find "${PROJECT_ROOT}/.claude/commands" -type f -print -quit | grep -q .; then
    echo "FORBIDDEN=.claude/commands" >&2
    return 1
  fi

  prompt_count="$(grep -Ec '^## Prompt [1-8] ' "${PROMPT_FILE}")"
  if [[ "${prompt_count}" != "8" ]]; then
    echo "INVALID_PROMPT_COUNT=${prompt_count}" >&2
    return 1
  fi
  for phase in 1 2 3 4 5 6 7 8; do
    extract_prompt "${phase}" >/dev/null
  done

  grep -q '^max_concurrent_threads_per_session = 3$' \
    "${PROJECT_ROOT}/.codex/config.toml"
  for file in "${PROJECT_ROOT}"/.agents/skills/*/SKILL.md; do
    if [[ "$(wc -l <"${file}")" -gt 500 ]]; then
      echo "SKILL_TOO_LARGE=${file}" >&2
      return 1
    fi
  done
  bash -n "${SCRIPT_DIR}/harness.sh"
  bash -n "${SCRIPT_DIR}/test-harness.sh"
  validate_completion_contract

  if command -v python3 >/dev/null 2>&1 &&
    python3 -c 'import tomllib' >/dev/null 2>&1; then
    python3 - "${PROJECT_ROOT}" <<'PY'
import pathlib
import sys
import tomllib

root = pathlib.Path(sys.argv[1])
for path in [root / ".codex/config.toml", *sorted((root / ".codex/agents").glob("*.toml"))]:
    with path.open("rb") as stream:
        tomllib.load(stream)
print("TOML_PARSE=PASS")
PY
  else
    echo "TOML_PARSE=UNVERIFIED"
  fi

  if command -v shellcheck >/dev/null 2>&1; then
    shellcheck "${SCRIPT_DIR}/harness.sh" "${SCRIPT_DIR}/test-harness.sh"
    echo "SHELLCHECK=PASS"
  else
    echo "SHELLCHECK=UNVERIFIED"
  fi
  if command -v shfmt >/dev/null 2>&1; then
    shfmt -d "${SCRIPT_DIR}/harness.sh" "${SCRIPT_DIR}/test-harness.sh"
    echo "SHFMT=PASS"
  else
    echo "SHFMT=UNVERIFIED"
  fi
  echo "TRIGGER_EVAL_CORPUS=PASS"
  echo "TRIGGER_ACTIVATION=UNVERIFIED"
  echo "HARNESS_STRUCTURE=PASS"
}

print_status() {
  local phase
  echo "PROJECT_ROOT=${PROJECT_ROOT}"
  echo "SCM=$(scm_state)"
  echo "GOVERNANCE=$(governance_status)"
  echo "FNRH_EXTERNAL_GATES=$(external_gates_status)"
  for phase in 1 2 3 4 5 6 7 8; do
    echo "PHASE_${phase}=$(status_for_phase "${phase}")"
  done
}

print_phase() {
  local phase="$1"
  echo "PHASE=${phase}"
  echo "TITLE=$(phase_title "${phase}")"
  echo "DEPENDENCIES=$(phase_dependencies "${phase}")"
  echo "STATUS=$(status_for_phase "${phase}")"
  echo "ELIGIBILITY=$(eligibility_for_phase "${phase}")"
}

dry_run() {
  local phase="$1"
  local scm
  scm="$(scm_state)"
  echo "MODE=DRY_RUN"
  echo "SCM=${scm}"
  if [[ "${scm}" == "ABSENT" ]]; then
    echo "WRITE_PARALLELISM=1"
  else
    echo "WRITE_PARALLELISM=DISJOINT_OWNERS_ONLY"
  fi
  echo "STUDY_PARALLELISM=3"
  echo "NESTED_ORCHESTRATORS=DISABLED"
  echo "POST_TASK_QUALITY_TARGET=make post-task-quality"
  echo "DONE_MARKER=POST_TASK_QUALITY=PASS"
  print_phase "${phase}"
  echo "PROMPT_SOURCE=${PROMPT_FILE}"
  if [[ "${phase}" == "1" ]]; then
    echo "GOVERNANCE=$(governance_status)"
  fi
  if [[ "${phase}" == "5" ]]; then
    echo "EXTERNAL_GATES_STATUS=$(external_gates_status)"
    echo "EXTERNAL_GATES=authorization,current_docs,homologation,per_accommodation_credentials,approved_mapping"
  fi
  if [[ "${phase}" == "6" ]]; then
    echo "ROADMAP_PILOT=NOT_EXECUTED_BY_PROMPT_6"
  fi
  if [[ "${phase}" == "7" ]]; then
    echo "EXTERNAL_GATES_STATUS=$(self_service_gate_status)"
    echo "EXTERNAL_GATES=third_party_identity_basis"
    if [[ "$(self_service_gate_status)" == "PASS" ]]; then
      echo "REAL_DATA=ELIGIBLE"
    else
      echo "REAL_DATA=BLOCKED"
    fi
    echo "SUPERSEDES_AUDIT=6A_BECOMES_UNVERIFIED"
  fi
  if [[ "${phase}" == "8" ]]; then
    echo "EXTERNAL_GATES_STATUS=$(external_context_gate_status)"
    echo "EXTERNAL_GATES=external_source_license,tide_harmonic_constants"
    if [[ "$(gates_status_for 8 EXTERNAL_SOURCE_LICENSE)" == "PASS" ]]; then
      echo "PUBLIC_CARDS=ELIGIBLE"
    else
      echo "PUBLIC_CARDS=BLOCKED"
    fi
    if [[ "$(gates_status_for 8 TIDE_HARMONIC_CONSTANTS)" == "PASS" ]]; then
      echo "TIDE_CARD=ELIGIBLE"
    else
      echo "TIDE_CARD=BLOCKED"
    fi
    echo "SUPERSEDES_AUDIT=6A_BECOMES_UNVERIFIED"
    echo "EGRESS=WORKER_ONLY"
  fi
  echo "MUTATIONS=NONE"
}

snapshot_phase() {
  local phase="$1"
  local attempt_id="${2:-$(date -u +%Y%m%dT%H%M%SZ)}"
  local phase_root="${WORKSPACE_ROOT}/phase-${phase}"
  local destination="${phase_root}/attempts/${attempt_id}"
  local item
  local copied=0

  if [[ ! "${attempt_id}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    echo "invalid attempt id: ${attempt_id}" >&2
    return 2
  fi
  if [[ -e "${destination}" ]]; then
    echo "attempt already exists: ${attempt_id}" >&2
    return 1
  fi
  mkdir -p "${destination}"
  for item in context.md plan.md qa.md status.txt external-gates.env study implementation; do
    if [[ -e "${phase_root}/${item}" ]]; then
      cp -R "${phase_root}/${item}" "${destination}/${item}"
      copied=1
    fi
  done
  for item in governance.md governance-status.txt; do
    if [[ -e "${WORKSPACE_ROOT}/${item}" ]]; then
      cp -R "${WORKSPACE_ROOT}/${item}" "${destination}/${item}"
      copied=1
    fi
  done
  if [[ "${copied}" == "0" ]]; then
    rmdir "${destination}"
    echo "SNAPSHOT=NO_CURRENT_ARTIFACTS"
    return
  fi
  printf 'UNVERIFIED\n' >"${phase_root}/status.txt"
  printf '%s\n' \
    "# Reexecution gate" \
    "" \
    "## Requirements" \
    "" \
    "- Revalidate the phase requirements." \
    "" \
    "## Commands" \
    "" \
    "- Pending." \
    "" \
    "## Risks" \
    "" \
    "The previous evidence is preserved in attempts/${attempt_id}." \
    "The phase must pass QA again before dependent work resumes." \
    "" \
    "GATE_STATUS=UNVERIFIED" \
    >"${phase_root}/qa.md"
  echo "SNAPSHOT=${destination}"
}

main() {
  local command="${1:-}"
  local phase
  case "${command}" in
    validate)
      validate_harness
      ;;
    status)
      print_status
      ;;
    prompt)
      phase="$(normalize_phase "${2:-}")"
      extract_prompt "${phase}"
      ;;
    phase)
      phase="$(normalize_phase "${2:-}")"
      print_phase "${phase}"
      ;;
    dry-run)
      phase="$(normalize_phase "${2:-}")"
      dry_run "${phase}"
      ;;
    snapshot)
      phase="$(normalize_phase "${2:-}")"
      snapshot_phase "${phase}" "${3:-}"
      ;;
    *)
      usage >&2
      return 2
      ;;
  esac
}

main "$@"
