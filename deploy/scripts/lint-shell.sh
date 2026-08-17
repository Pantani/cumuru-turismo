#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

files="$(mktemp "${TMPDIR:-/tmp}/cumuru-shell-lint.XXXXXX")"
trap 'rm -f -- "${files}"' EXIT HUP INT QUIT TERM

git ls-files -z --cached --others --exclude-standard -- '*.sh' >"${files}"

count=0
while IFS= read -r -d '' file; do
  test -f "${file}" || {
    echo "lint-shell: arquivo indexado ausente do worktree: ${file}" >&2
    exit 1
  }
  bash -n "${file}"
  count=$((count + 1))
done <"${files}"

test "${count}" -gt 0
echo "SHELL_SYNTAX=PASS files=${count}"
