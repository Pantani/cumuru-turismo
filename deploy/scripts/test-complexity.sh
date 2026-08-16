#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WEB_DIR="${ROOT_DIR}/apps/web"
WEB_CONFIG="${WEB_DIR}/.oxlintrc.json"
GOCYCLO="${1:?gocyclo path is required}"
GOCOGNIT="${2:?gocognit path is required}"
RG="${RG:-rg}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-complexity.XXXXXX")"
ANALYZER_OUTPUT=""
ANALYZER_STATUS=0
OXLINT_OPTIONS=(--config .oxlintrc.json --deny-warnings)
WEB_FILES=()
WEB_PASS_FILES=()
WEB_CYCLO_FAIL_FILES=()
WEB_COGNIT_FAIL_FILES=()

cleanup() {
  rm -rf -- "${TEMP_DIR}"
}
trap cleanup EXIT

capture_command() {
  set +e
  ANALYZER_OUTPUT="$("$@" 2>&1)"
  ANALYZER_STATUS=$?
  set -e
}

run_from_web() {
  (
    cd "${WEB_DIR}"
    "$@"
  )
}

require_tool() {
  local tool="$1"
  if [[ ! -x "${tool}" ]]; then
    echo "required complexity tool is unavailable: ${tool}" >&2
    return 2
  fi
}

find_suppressions() {
  local output
  local status
  set +e
  output="$(
    "${RG}" -n \
      --glob '!**/node_modules/**' \
      --glob '!**/dist/**' \
      --glob '*.go' \
      --glob '*.js' \
      --glob '*.jsx' \
      --glob '*.mjs' \
      --glob '*.cjs' \
      --glob '*.ts' \
      --glob '*.tsx' \
      --glob '*.mts' \
      --glob '*.cts' \
      '(oxlint|eslint|sonarjs)-(disable|ignore)|(gocognit|gocyclo)(:|-)(ignore|disable)|@ts-(ignore|nocheck)|nolint:[^\r\n]*(gocognit|gocyclo)' \
      "$@" 2>&1
  )"
  status=$?
  set -e
  case "${status}" in
    0)
      echo "${output}"
      ;;
    1)
      return 0
      ;;
    *)
      echo "suppression scan failed with status ${status}:" >&2
      echo "${output}" >&2
      return "${status}"
      ;;
  esac
}

reject_suppressions() {
  local findings
  local status
  set +e
  findings="$(find_suppressions "${ROOT_DIR}/apps/api" "${ROOT_DIR}/apps/web")"
  status=$?
  set -e
  if [[ "${status}" -ne 0 ]]; then
    return 1
  fi
  if [[ -n "${findings}" ]]; then
    echo "complexity/type-check suppressions are forbidden:" >&2
    echo "${findings}" >&2
    return 1
  fi
}

reject_web_config_exclusions() {
  local config_path="${1:-${WEB_CONFIG}}"
  local output
  local status
  set +e
  output="$(
    node - "${config_path}" <<'NODE'
const fs = require("node:fs");

const path = process.argv[2];
const config = JSON.parse(fs.readFileSync(path, "utf8"));
const forbidden = ["ignorePatterns", "overrides", "extends"];
const present = forbidden.filter((key) =>
  Object.prototype.hasOwnProperty.call(config, key),
);

if (present.length > 0) {
  console.error(`forbidden Oxlint exclusion keys: ${present.join(",")}`);
  process.exit(1);
}
NODE
  )"
  status=$?
  set -e
  if [[ "${status}" -ne 0 ]]; then
    echo "${output}" >&2
    return 1
  fi
}

prove_web_config_exclusion_detection() {
  local fixture="${TEMP_DIR}/forbidden-oxlint-config.json"
  local content
  for content in \
    '{"ignorePatterns":["src/**"]}' \
    '{"overrides":[]}' \
    '{"extends":[]}'; do
    printf '%s\n' "${content}" >"${fixture}"
    if reject_web_config_exclusions "${fixture}" >/dev/null 2>&1; then
      echo "a forbidden Oxlint exclusion key was accepted: ${content}" >&2
      return 1
    fi
  done
}

prepare_oxlint_options() {
  capture_command run_from_web npm exec -- oxlint --help
  if [[ "${ANALYZER_STATUS}" -ne 0 ]]; then
    echo "unable to inspect Oxlint options:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi
  if ! grep -Fq -- '--no-ignore' <<<"${ANALYZER_OUTPUT}"; then
    echo "Oxlint does not support the required --no-ignore option" >&2
    return 1
  fi
  if ! grep -Fq -- '--disable-nested-config' <<<"${ANALYZER_OUTPUT}"; then
    echo "Oxlint does not support the required --disable-nested-config option" >&2
    return 1
  fi
  OXLINT_OPTIONS+=(--no-ignore --disable-nested-config)
}

collect_web_files() {
  local file
  local inventory="${TEMP_DIR}/web-files.txt"
  local unsorted="${TEMP_DIR}/web-files.unsorted"
  if ! find "${WEB_DIR}" -type f \
    \( -name '*.js' -o -name '*.jsx' -o -name '*.mjs' -o -name '*.cjs' \
    -o -name '*.ts' -o -name '*.tsx' -o -name '*.mts' -o -name '*.cts' \) \
    -not -path '*/node_modules/*' \
    -not -path '*/dist/*' \
    -print >"${unsorted}"; then
    echo "unable to enumerate owned web JavaScript/TypeScript files" >&2
    return 1
  fi
  if ! sort "${unsorted}" >"${inventory}"; then
    echo "unable to sort the owned web JavaScript/TypeScript inventory" >&2
    return 1
  fi
  while IFS= read -r file; do
    WEB_FILES+=("${file}")
  done <"${inventory}"
  if [[ "${#WEB_FILES[@]}" -eq 0 ]]; then
    echo "no owned web JavaScript/TypeScript files were found" >&2
    return 1
  fi
}

run_oxlint_files() {
  capture_command run_from_web npm exec -- oxlint \
    "${OXLINT_OPTIONS[@]}" "$@"
}

write_suppression_fixtures() {
  mkdir -p "${TEMP_DIR}/suppressions"
  printf '%s\n' \
    'package fixture' \
    '' \
    '//gocognit:ignore' \
    'func cognitiveSuppressionMustFail() {}' \
    >"${TEMP_DIR}/suppressions/gocognit_ignore.go"
  printf '%s\n' \
    'package fixture' \
    '' \
    '//gocyclo:ignore' \
    'func cyclomaticSuppressionMustFail() {}' \
    >"${TEMP_DIR}/suppressions/gocyclo_ignore.go"
  printf '%s\n' \
    'package fixture' \
    '' \
    '//nolint:gocyclo' \
    'func nolintSuppressionMustFail() {}' \
    >"${TEMP_DIR}/suppressions/nolint_gocyclo.go"
}

prove_suppression_detection() {
  local output
  local status
  set +e
  output="$(find_suppressions "${TEMP_DIR}/suppressions")"
  status=$?
  set -e
  if [[ "${status}" -ne 0 ]]; then
    return 1
  fi
  grep -Fq "gocognit_ignore.go" <<<"${output}"
  grep -Fq "gocyclo_ignore.go" <<<"${output}"
  grep -Fq "nolint_gocyclo.go" <<<"${output}"
}

prove_suppression_scanner_failure_detection() {
  local fake="${TEMP_DIR}/rg-operational-failure"
  local original_rg="${RG}"
  local output
  local status
  printf '%s\n' '#!/bin/sh' 'exit 74' >"${fake}"
  chmod +x "${fake}"
  RG="${fake}"
  set +e
  output="$(find_suppressions "${TEMP_DIR}/suppressions" 2>&1)"
  status=$?
  set -e
  RG="${original_rg}"
  if [[ "${status}" -le 1 ]]; then
    echo "an operational suppression scanner failure was accepted" >&2
    echo "${output}" >&2
    return 1
  fi
}

write_go_fixture() {
  local directory="$1"
  local score="$2"
  local suffix="$3"
  local branch=1
  local path="${directory}/complexity_test.go"

  mkdir -p "${directory}"
  {
    echo "package fixture"
    echo
    echo "func cyclomatic${suffix}(value int) bool {"
    while [[ "${branch}" -le "${score}" ]]; do
      echo "	if value == ${branch} {"
      echo "		return true"
      echo "	}"
      branch="$((branch + 1))"
    done
    echo "	return false"
    echo "}"
  } >"${path}"
}

write_web_fixture() {
  local path="$1"
  local score="$2"
  local suffix="$3"
  local branch=1
  local export_prefix="export "

  case "${path}" in
    *.cjs|*.cts) export_prefix="" ;;
  esac

  {
    echo "${export_prefix}function cyclomatic${suffix}(value) {"
    while [[ "${branch}" -le "${score}" ]]; do
      echo "  if (value === ${branch}) return true;"
      branch="$((branch + 1))"
    done
    echo "  return false;"
    echo "}"
    if [[ -z "${export_prefix}" ]]; then
      echo
      echo "module.exports = { cyclomatic${suffix} };"
    fi
  } >"${path}"
}

write_threshold_fixtures() {
  local extension
  local pass_path

  # A run of N sequential top-level ifs scores N+1 cyclomatic and N cognitive,
  # so each threshold is proved on its own boundary instead of a shared shape.
  write_go_fixture "${TEMP_DIR}/go-cyclo-pass" 4 CycloPass
  write_go_fixture "${TEMP_DIR}/go-cyclo-fail" 5 CycloFail
  write_go_fixture "${TEMP_DIR}/go-cognit-pass" 8 CognitPass
  write_go_fixture "${TEMP_DIR}/go-cognit-fail" 9 CognitFail
  mkdir -p "${TEMP_DIR}/web-pass" "${TEMP_DIR}/web-fail"
  for extension in js jsx mjs cjs ts tsx mts cts; do
    pass_path="${TEMP_DIR}/web-pass/complexity-pass.${extension}"
    write_web_fixture "${pass_path}" 4 Pass
    write_web_fixture "${TEMP_DIR}/web-fail/cyclomatic-six.${extension}" 5 CycloFail
    write_web_fixture "${TEMP_DIR}/web-fail/cognitive-nine.${extension}" 9 CognitFail
    WEB_PASS_FILES+=("${pass_path}")
    WEB_CYCLO_FAIL_FILES+=("${TEMP_DIR}/web-fail/cyclomatic-six.${extension}")
    WEB_COGNIT_FAIL_FILES+=("${TEMP_DIR}/web-fail/cognitive-nine.${extension}")
  done
}

prove_go_threshold() {
  capture_command "${GOCYCLO}" -over 5 "${TEMP_DIR}/go-cyclo-pass"
  if [[ "${ANALYZER_STATUS}" -ne 0 || -n "${ANALYZER_OUTPUT}" ]]; then
    echo "gocyclo rejected the score-5 fixture:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi

  capture_command "${GOCOGNIT}" -test -over 8 "${TEMP_DIR}/go-cognit-pass"
  if [[ "${ANALYZER_STATUS}" -ne 0 || -n "${ANALYZER_OUTPUT}" ]]; then
    echo "gocognit rejected the score-8 fixture:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi

  capture_command "${GOCYCLO}" -over 5 "${TEMP_DIR}/go-cyclo-fail"
  if [[ "${ANALYZER_STATUS}" -ne 1 ]] ||
    ! grep -Eq '6 .*cyclomaticCycloFail .*complexity_test\.go:' \
    <<<"${ANALYZER_OUTPUT}"; then
    echo "gocyclo did not reject the score-6 fixture" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi

  capture_command "${GOCOGNIT}" -test -over 8 "${TEMP_DIR}/go-cognit-fail"
  if [[ "${ANALYZER_STATUS}" -ne 1 ]] ||
    ! grep -Eq '9 .*cyclomaticCognitFail .*complexity_test\.go:' \
    <<<"${ANALYZER_OUTPUT}"; then
    echo "gocognit did not reject the score-9 fixture" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi
}

prove_web_threshold() {
  local file

  run_oxlint_files "${WEB_PASS_FILES[@]}"
  if [[ "${ANALYZER_STATUS}" -ne 0 ]]; then
    echo "Oxlint rejected a passing web fixture:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi

  for file in "${WEB_CYCLO_FAIL_FILES[@]}"; do
    run_oxlint_files "${file}"
    if [[ "${ANALYZER_STATUS}" -ne 1 ]] ||
      ! grep -Fq "eslint(complexity)" <<<"${ANALYZER_OUTPUT}"; then
      echo "Oxlint did not reject cyclomatic 6 for ${file}" >&2
      echo "${ANALYZER_OUTPUT}" >&2
      return 1
    fi
  done

  for file in "${WEB_COGNIT_FAIL_FILES[@]}"; do
    run_oxlint_files "${file}"
    if [[ "${ANALYZER_STATUS}" -ne 1 ]] ||
      ! grep -Fq "sonarjs(cognitive-complexity)" <<<"${ANALYZER_OUTPUT}"; then
      echo "Oxlint did not reject cognitive 9 for ${file}" >&2
      echo "${ANALYZER_OUTPUT}" >&2
      return 1
    fi
  done
}

check_analyzer_clean() {
  local label="$1"
  shift
  capture_command "$@"
  if [[ "${ANALYZER_STATUS}" -ne 0 || -n "${ANALYZER_OUTPUT}" ]]; then
    echo "${label} failed with status ${ANALYZER_STATUS}:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi
}

prove_operational_failure_detection() {
  local fake="${TEMP_DIR}/operational-failure"
  printf '%s\n' '#!/bin/sh' 'exit 73' >"${fake}"
  chmod +x "${fake}"
  if check_analyzer_clean "synthetic operational analyzer" "${fake}" \
    >/dev/null 2>&1; then
    echo "an operational analyzer failure was accepted" >&2
    return 1
  fi
}

prove_web_coverage() {
  local actual
  local expected
  local required
  run_oxlint_files --debug files "${WEB_FILES[@]}"
  if [[ "${ANALYZER_STATUS}" -ne 0 ]]; then
    echo "unable to enumerate Oxlint coverage:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi
  expected="$(
    printf '%s\n' "${WEB_FILES[@]#${WEB_DIR}/}" |
      sort
  )"
  actual="$(
    printf '%s\n' "${ANALYZER_OUTPUT}" |
      sed '/^[[:space:]]*$/d' |
      sort
  )"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "Oxlint coverage does not match the owned web inventory:" >&2
    diff -u <(printf '%s\n' "${expected}") \
      <(printf '%s\n' "${actual}") >&2 || true
    return 1
  fi
  for required in \
    "public/service-worker.js" \
    "src/generated/schema.ts" \
    "src/app/App.test.tsx" \
    "vite.config.ts"; do
    if ! grep -Fq "${required}" <<<"${ANALYZER_OUTPUT}"; then
      echo "required web complexity surface is missing: ${required}" >&2
      return 1
    fi
  done
}

# Production and generated Go code is held to cyclomatic 5 and cognitive 8.
# Go test code is held to 9: a table-driven test with several assertions is
# clear at 6 or 7, and splitting those assertions into helpers costs the
# locality that makes a failure readable. Web code, tests included, meets 5/8.
check_go_baseline() {
  local failed=0
  check_analyzer_clean "Go cyclomatic complexity" \
    "${GOCYCLO}" -over 5 -ignore '_test\.go' "${ROOT_DIR}/apps/api" || failed=1
  check_analyzer_clean "Go cognitive complexity" \
    "${GOCOGNIT}" -over 8 -ignore '_test\.go' "${ROOT_DIR}/apps/api" || failed=1
  check_analyzer_clean "Go cyclomatic complexity in tests" \
    "${GOCYCLO}" -over 9 "${ROOT_DIR}/apps/api" || failed=1
  check_analyzer_clean "Go cognitive complexity in tests" \
    "${GOCOGNIT}" -test -over 9 "${ROOT_DIR}/apps/api" || failed=1
  if [[ "${failed}" -ne 0 ]]; then
    return 1
  fi
  echo "Go complexity passed: 5/8 for source and generated code, 9 for tests"
}

check_web_baseline() {
  run_oxlint_files "${WEB_FILES[@]}"
  if [[ "${ANALYZER_STATUS}" -ne 0 ]]; then
    echo "web complexity failed with status ${ANALYZER_STATUS}:" >&2
    echo "${ANALYZER_OUTPUT}" >&2
    return 1
  fi
  echo "web complexity passed for ${#WEB_FILES[@]} owned JavaScript/TypeScript files"
}

require_tool "${GOCYCLO}"
require_tool "${GOCOGNIT}"
prove_web_config_exclusion_detection
reject_web_config_exclusions
echo "Oxlint ignorePatterns, overrides, and extends were rejected"
prepare_oxlint_options
collect_web_files
write_suppression_fixtures
write_threshold_fixtures
prove_go_threshold
prove_web_threshold
echo "cyclomatic 5 and cognitive 8 passed; 6 and 9 failed for Go and JS/JSX/MJS/CJS/TS/TSX/MTS/CTS"
prove_operational_failure_detection
echo "synthetic operational analyzer failure was rejected"
prove_suppression_detection
prove_suppression_scanner_failure_detection
reject_suppressions
echo "inline/nolint suppressions and suppression scanner failures were rejected"
prove_web_coverage
echo "web complexity coverage includes public, generated, config, and tests"

failed=0
check_go_baseline || failed=1
check_web_baseline || failed=1
if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi

echo "complexity gate passed fail-closed for all owned Go and web code"
