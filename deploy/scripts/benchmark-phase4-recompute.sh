#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_FILE="$(mktemp "${TMPDIR:-/tmp}/cumuru-phase4-benchmark.XXXXXX")"

cleanup() {
  rm -f -- "${OUTPUT_FILE}"
}
trap cleanup EXIT

logical_cpu_count() {
  getconf _NPROCESSORS_ONLN 2>/dev/null ||
    sysctl -n hw.ncpu 2>/dev/null ||
    echo unknown
}

host_memory_bytes() {
  if sysctl -n hw.memsize 2>/dev/null; then
    return
  fi
  awk '/^MemTotal:/ { print $2 * 1024; found=1 } END { if (!found) print "unknown" }' \
    /proc/meminfo 2>/dev/null
}

echo "PHASE4_BENCHMARK_HARDWARE system=$(uname -srm) logical_cpus=$(logical_cpu_count) memory_bytes=$(host_memory_bytes) toolchain=$(go version)"

go -C "${ROOT_DIR}/apps/api" test \
  -run '^TestPhase4ThreeYearRecomputeBudget$' \
  -count=1 \
  -v \
  ./internal/analytics |
  tee "${OUTPUT_FILE}"

grep --fixed-strings --quiet "PHASE4_RECOMPUTE_BENCHMARK=PASS" "${OUTPUT_FILE}"

echo "PHASE4_BENCHMARK=PASS previous_snapshot_gate=PHASE4_LAST_VALID_SNAPSHOT"
