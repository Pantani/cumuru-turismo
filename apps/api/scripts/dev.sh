#!/bin/sh
# Recompiles and restarts one Go process whenever a watched file changes.
#
# It polls checksums instead of using an inotify-based reloader: a bind mount
# from macOS or Windows does not deliver filesystem events reliably, and a
# missed event in a hot reload loop looks exactly like a bug in the application.
# Polling is slower to react and always reacts.

set -eu

target="${CUMURU_DEV_TARGET:-api}"
interval="${CUMURU_DEV_POLL_INTERVAL:-2}"
binary="/tmp/cumuru-${target}"
server_pid=""
snapshot=""

log() {
  echo "[dev-${target}] $1"
}

take_snapshot() {
  find ./cmd ./internal -type f -name '*.go' -print 2>/dev/null |
    sort |
    while IFS= read -r file; do
      [ -n "${file}" ] || continue
      cksum "${file}" 2>/dev/null || true
    done
  for file in ./go.mod ./go.sum; do
    [ -f "${file}" ] && cksum "${file}" 2>/dev/null
  done
  return 0
}

stop_process() {
  [ -n "${server_pid}" ] || return 0
  if kill -0 "${server_pid}" 2>/dev/null; then
    kill "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" 2>/dev/null || true
  fi
  server_pid=""
}

# A failed build keeps the previous process alive: a syntax error in the editor
# must not take the whole stack down while it is being typed.
start_process() {
  log "compilando…"
  if ! go build -o "${binary}" "./cmd/${target}"; then
    log "falha ao compilar; mantendo o processo anterior."
    return 0
  fi
  stop_process
  "${binary}" &
  server_pid=$!
  log "processo reiniciado (pid ${server_pid})."
}

shutdown() {
  stop_process
  exit 0
}

trap shutdown INT TERM

snapshot="$(take_snapshot)"
start_process

while true; do
  sleep "${interval}"
  current="$(take_snapshot)"
  if [ "${current}" != "${snapshot}" ]; then
    snapshot="${current}"
    log "mudança detectada."
    start_process
  fi
done
