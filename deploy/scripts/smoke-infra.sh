#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

application_ready=false
for _ in $(seq 1 20); do
  if "${ROOT_DIR}/deploy/scripts/smoke.sh"; then
    application_ready=true
    break
  fi
  sleep 1
done

if [[ "${application_ready}" != "true" ]]; then
  echo "local infrastructure smoke checks failed: application is not ready" >&2
  exit 1
fi

wait_for_url() {
  curl \
    --retry 20 \
    --retry-all-errors \
    --retry-delay 1 \
    --fail \
    --silent \
    --show-error \
    "$1" >/dev/null
}

wait_for_url http://127.0.0.1:13133/
wait_for_url http://127.0.0.1:9092/-/ready
wait_for_url http://127.0.0.1:3200/ready
wait_for_url http://127.0.0.1:3000/api/health

metrics_ready=false
for _ in $(seq 1 30); do
  api_target="$(
    curl --fail --silent --show-error \
      'http://127.0.0.1:9092/api/v1/query?query=up%7Bjob%3D%22cumuru-api%22%7D'
  )"
  worker_target="$(
    curl --fail --silent --show-error \
      'http://127.0.0.1:9092/api/v1/query?query=up%7Bjob%3D%22cumuru-worker%22%7D'
  )"

  if API_TARGET="${api_target}" WORKER_TARGET="${worker_target}" node -e '
    for (const name of ["API_TARGET", "WORKER_TARGET"]) {
      const response = JSON.parse(process.env[name] ?? "null");
      const value = response?.data?.result?.[0]?.value?.[1];
      if (response?.status !== "success" || value !== "1") {
        process.exit(1);
      }
    }
  '; then
    metrics_ready=true
    break
  fi
  sleep 1
done

if [[ "${metrics_ready}" != "true" ]]; then
  echo "local infrastructure smoke checks failed: metrics targets are not up" >&2
  exit 1
fi

# O sampler do aplicativo retém 10%; este lote torna a prova de exportação
# efetiva sem alterar a política do runtime.
for _ in $(seq 1 100); do
  curl --fail --silent --show-error \
    http://127.0.0.1:4173/api/v1/platform/health >/dev/null
done

for _ in $(seq 1 30); do
  traces="$(
    curl --fail --silent --show-error \
      'http://127.0.0.1:3200/api/search?limit=1'
  )"
  if TRACES="${traces}" node -e '
    const response = JSON.parse(process.env.TRACES ?? "null");
    if (!Array.isArray(response?.traces) || response.traces.length === 0) {
      process.exit(1);
    }
  '; then
    echo "local infrastructure smoke checks passed"
    exit 0
  fi
  sleep 1
done

echo "local infrastructure smoke checks failed: Tempo received no trace" >&2
exit 1
