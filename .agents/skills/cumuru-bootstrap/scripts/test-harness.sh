#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
HARNESS="${SCRIPT_DIR}/harness.sh"
TEST_WORKSPACE="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-harness-test.XXXXXX")"
TEST_OUTSIDE="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-harness-outside.XXXXXX")"

cleanup() {
  rm -R -- "${TEST_WORKSPACE}"
  rm -R -- "${TEST_OUTSIDE}"
}
trap cleanup EXIT

run_harness() {
  CUMURU_HARNESS_WORKSPACE_ROOT="${TEST_WORKSPACE}" bash "${HARNESS}" "$@"
}

write_phase_status() {
  local phase="$1"
  local status="$2"
  mkdir -p "${TEST_WORKSPACE}/phase-${phase}"
  printf '%s\n' "${status}" >"${TEST_WORKSPACE}/phase-${phase}/status.txt"
  printf '%s\n' \
    '# Phase gate' \
    '## Requirements' \
    '- Test fixture.' \
    '## Commands' \
    '- Test fixture.' \
    '## Risks' \
    '- Test fixture.' \
    "GATE_STATUS=${status}" \
    >"${TEST_WORKSPACE}/phase-${phase}/qa.md"
}

validation="$(run_harness validate)"
echo "${validation}"
grep -q 'HARNESS_STRUCTURE=PASS' <<<"${validation}"
grep -q 'TRIGGER_EVAL_CORPUS=PASS' <<<"${validation}"
grep -q 'TRIGGER_ACTIVATION=UNVERIFIED' <<<"${validation}"
grep -q 'POST_TASK_QUALITY_CONTRACT=PASS' <<<"${validation}"

for phase in 1 2 3 4 5 6 7; do
  prompt="$(run_harness prompt "${phase}")"
  [[ -n "${prompt}" ]]
done
prompt_seven="$(run_harness prompt 7)"
grep -q 'Implemente somente a Fase 7' <<<"${prompt_seven}"
grep -q 'THIRD_PARTY_IDENTITY_BASIS' <<<"${prompt_seven}"
grep -q 'Esta fase não sucede a Fase 6' <<<"${prompt_seven}"
grep -q 'Implemente somente a Fase 1' <<<"$(run_harness prompt 1)"
grep -q 'React 19.2' <<<"$(run_harness prompt 1)"
grep -q 'Se algum item faltar, pare e marque BLOCKED' <<<"$(run_harness prompt 5)"

phase_six="$(run_harness phase 6A)"
grep -q 'TITLE=6A Auditoria de prontidao' <<<"${phase_six}"
if run_harness prompt 6B >/dev/null 2>&1; then
  echo "operational pilot unexpectedly mapped to Prompt 6" >&2
  exit 1
fi

dry_run_one="$(run_harness dry-run 1)"
grep -q 'ELIGIBILITY=BLOCKED' <<<"${dry_run_one}"
grep -q '^POST_TASK_QUALITY_TARGET=make post-task-quality$' <<<"${dry_run_one}"
grep -q '^DONE_MARKER=POST_TASK_QUALITY=PASS$' <<<"${dry_run_one}"
printf 'PROTOTYPE_ONLY\n' >"${TEST_WORKSPACE}/governance-status.txt"
printf 'GOVERNANCE_STATUS=PROTOTYPE_ONLY\n' >"${TEST_WORKSPACE}/governance.md"
dry_run_one="$(run_harness dry-run 1)"
grep -q 'ELIGIBILITY=ELIGIBLE_PROTOTYPE_ONLY' <<<"${dry_run_one}"

mkdir -p "${TEST_WORKSPACE}/phase-1"
printf 'PASS\n' >"${TEST_WORKSPACE}/phase-1/status.txt"
dry_run_two="$(run_harness dry-run 2)"
grep -q 'ELIGIBILITY=BLOCKED' <<<"${dry_run_two}"
printf '%s\n' \
  '# Phase gate' \
  '## Requirements' \
  '- Test fixture.' \
  '## Commands' \
  '- Test fixture.' \
  '## Risks' \
  '- Test fixture.' \
  'GATE_STATUS=PASS' \
  >"${TEST_WORKSPACE}/phase-1/qa.md"
dry_run_two="$(run_harness dry-run 2)"
grep -q 'ELIGIBILITY=ELIGIBLE_PROTOTYPE_ONLY' <<<"${dry_run_two}"
grep -q 'MODE=DRY_RUN' <<<"${dry_run_two}"
grep -Eq 'SCM=(ABSENT|PRESENT)' <<<"${dry_run_two}"

write_phase_status 2 PASS
write_phase_status 3 PASS
write_phase_status 4 PASS

# Phase 7 depends on phases 2 and 3 only; phase 5 and 6 never gate it.
grep -q 'TITLE=Autoatendimento e aprovacao' <<<"$(run_harness phase 7)"
dry_run_seven="$(run_harness dry-run 7)"
grep -q 'ELIGIBILITY=ELIGIBLE_PROTOTYPE_ONLY' <<<"${dry_run_seven}"
grep -q '^EXTERNAL_GATES=third_party_identity_basis$' <<<"${dry_run_seven}"
grep -q '^EXTERNAL_GATES_STATUS=BLOCKED$' <<<"${dry_run_seven}"
grep -q '^REAL_DATA=BLOCKED$' <<<"${dry_run_seven}"
grep -q '^SUPERSEDES_AUDIT=6A_BECOMES_UNVERIFIED$' <<<"${dry_run_seven}"
write_phase_status 3 FAIL
grep -q 'ELIGIBILITY=BLOCKED' <<<"$(run_harness dry-run 7)"
write_phase_status 3 PASS

dry_run_five="$(run_harness dry-run 5)"
grep -q 'ELIGIBILITY=BLOCKED' <<<"${dry_run_five}"
grep -q 'EXTERNAL_GATES_STATUS=BLOCKED' <<<"${dry_run_five}"

mkdir -p "${TEST_WORKSPACE}/phase-5"
mkdir -p "${TEST_WORKSPACE}/evidence"
for evidence in \
  authorization.md \
  current-docs.md \
  homologation.md \
  credential-policy.md \
  approved-mapping.md; do
  printf 'Verified test attestation.\n' >"${TEST_WORKSPACE}/evidence/${evidence}"
done
printf '%s\n' \
  'AUTHORIZATION=PASS' \
  'AUTHORIZATION_EVIDENCE=evidence/authorization.md' \
  'CURRENT_DOCS=PASS' \
  'CURRENT_DOCS_EVIDENCE=evidence/current-docs.md' \
  'HOMOLOGATION=PASS' \
  'HOMOLOGATION_EVIDENCE=evidence/homologation.md' \
  'PER_ACCOMMODATION_CREDENTIALS=PASS' \
  'PER_ACCOMMODATION_CREDENTIALS_EVIDENCE=evidence/credential-policy.md' \
  'APPROVED_MAPPING=PASS' \
  'APPROVED_MAPPING_EVIDENCE=evidence/approved-mapping.md' \
  >"${TEST_WORKSPACE}/phase-5/external-gates.env"
dry_run_five="$(run_harness dry-run 5)"
grep -q 'ELIGIBILITY=BLOCKED' <<<"${dry_run_five}"
printf 'PASS\n' >"${TEST_WORKSPACE}/governance-status.txt"
printf 'GOVERNANCE_STATUS=PASS\n' >"${TEST_WORKSPACE}/governance.md"
dry_run_five="$(run_harness dry-run 5)"
grep -q 'ELIGIBILITY=ELIGIBLE' <<<"${dry_run_five}"
grep -q 'EXTERNAL_GATES_STATUS=PASS' <<<"${dry_run_five}"

mv "${TEST_WORKSPACE}/evidence" "${TEST_WORKSPACE}/evidence-local"
for evidence in \
  authorization.md \
  current-docs.md \
  homologation.md \
  credential-policy.md \
  approved-mapping.md; do
  printf 'External symlink target.\n' >"${TEST_OUTSIDE}/${evidence}"
done
ln -s "${TEST_OUTSIDE}" "${TEST_WORKSPACE}/evidence"
dry_run_five="$(run_harness dry-run 5)"
grep -q 'ELIGIBILITY=BLOCKED' <<<"${dry_run_five}"
grep -q 'EXTERNAL_GATES_STATUS=BLOCKED' <<<"${dry_run_five}"
rm -- "${TEST_WORKSPACE}/evidence"
mv "${TEST_WORKSPACE}/evidence-local" "${TEST_WORKSPACE}/evidence"
grep -q 'ELIGIBILITY=ELIGIBLE' <<<"$(run_harness dry-run 5)"

mkdir -p "${TEST_WORKSPACE}/phase-7"
printf 'Verified legal basis attestation.\n' \
  >"${TEST_WORKSPACE}/evidence/third-party-identity-basis.md"
printf '%s\n' \
  'THIRD_PARTY_IDENTITY_BASIS=PASS' \
  'THIRD_PARTY_IDENTITY_BASIS_EVIDENCE=evidence/third-party-identity-basis.md' \
  >"${TEST_WORKSPACE}/phase-7/external-gates.env"
dry_run_seven="$(run_harness dry-run 7)"
grep -q '^EXTERNAL_GATES_STATUS=PASS$' <<<"${dry_run_seven}"
grep -q '^REAL_DATA=ELIGIBLE$' <<<"${dry_run_seven}"

write_phase_status 5 FAIL
grep -q 'ELIGIBILITY=BLOCKED' <<<"$(run_harness dry-run 6A)"
write_phase_status 5 BLOCKED
grep -q 'ELIGIBILITY=ELIGIBLE_WITH_PHASE5_BLOCKED' <<<"$(run_harness dry-run 6A)"
write_phase_status 5 PASS
grep -q 'ELIGIBILITY=ELIGIBLE' <<<"$(run_harness dry-run 6A)"

printf 'version=one\n' >"${TEST_WORKSPACE}/phase-5/plan.md"
run_harness snapshot 5 test-attempt >/dev/null
printf 'version=two\n' >"${TEST_WORKSPACE}/phase-5/plan.md"
grep -q 'version=one' \
  "${TEST_WORKSPACE}/phase-5/attempts/test-attempt/plan.md"
grep -q '^UNVERIFIED$' "${TEST_WORKSPACE}/phase-5/status.txt"
grep -q '^GATE_STATUS=UNVERIFIED$' "${TEST_WORKSPACE}/phase-5/qa.md"
grep -q '^GOVERNANCE_STATUS=PASS$' \
  "${TEST_WORKSPACE}/phase-5/attempts/test-attempt/governance.md"
grep -q 'ELIGIBILITY=BLOCKED' <<<"$(run_harness dry-run 6A)"

should_trigger_count="$(
  awk '
    /^## Should trigger$/ { section = 1; next }
    /^## / && section { exit }
    section && /^[0-9]+\./ { count++ }
    END { print count + 0 }
  ' "${SCRIPT_DIR}/../references/trigger-evals.md"
)"
functional_test_count="$(
  awk '
    /^## Testes funcionais$/ { section = 1; next }
    /^## / && section { exit }
    section && /^[0-9]+\./ { count++ }
    END { print count + 0 }
  ' "${SCRIPT_DIR}/../references/trigger-evals.md"
)"
should_not_trigger_count="$(
  awk '
    /^## Should not trigger$/ { section = 1; next }
    /^## / && section { exit }
    section && /^[0-9]+\./ { count++ }
    END { print count + 0 }
  ' "${SCRIPT_DIR}/../references/trigger-evals.md"
)"
[[ "${functional_test_count}" == "4" ]]
[[ "${should_trigger_count}" == "11" ]]
[[ "${should_not_trigger_count}" == "11" ]]
grep -Fq 'Conclua o CLAIM do frontend sem rodar o gate global' \
  "${SCRIPT_DIR}/../references/trigger-evals.md"

if run_harness prompt 0 >/dev/null 2>&1; then
  echo "invalid phase unexpectedly succeeded" >&2
  exit 1
fi

echo "HARNESS_TESTS=PASS"
