#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-proxy-test.XXXXXX")"
NGINX_CONTAINER="cumuru-nginx-proxy-test-${PPID}-$$"
NGINX_IMAGE="nginxinc/nginx-unprivileged:1.29.8-alpine3.23@sha256:0c79d56aee561a1d81c63f00eee5fb5fe29279560cdc55e91425133104c7fbe6"
UPSTREAM_PID=""
VITE_PID=""

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  trap - EXIT
  set +e
  if test -n "${VITE_PID}"; then
    kill "${VITE_PID}" >/dev/null 2>&1
    wait "${VITE_PID}" >/dev/null 2>&1
  fi
  if test -n "${UPSTREAM_PID}"; then
    kill "${UPSTREAM_PID}" >/dev/null 2>&1
    wait "${UPSTREAM_PID}" >/dev/null 2>&1
  fi
  docker rm --force "${NGINX_CONTAINER}" >/dev/null 2>&1
  if test "$?" -ne 0; then
    cleanup_status=1
  fi
  residual_container="$(
    docker container ls --all --quiet \
      --filter "name=^/${NGINX_CONTAINER}$"
  )"
  if test "$?" -ne 0 || test -n "${residual_container}"; then
    cleanup_status=1
  fi
  if test -e "${WORK_DIR}" && ! rm -rf "${WORK_DIR}"; then
    cleanup_status=1
  fi
  set -e
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  exit "${cleanup_status}"
}
trap cleanup EXIT

free_port() {
  node -e '
    const net = require("node:net");
    const server = net.createServer();
    server.listen(0, "127.0.0.1", () => {
      console.log(server.address().port);
      server.close();
    });
  '
}

wait_for_url() {
  local url="$1"
  for ignored in 1 2 3 4 5 6 7 8 9 10; do
    if curl --fail --silent --show-error "${url}" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  echo "timed out waiting for local test server" >&2
  return 1
}

assert_absent() {
  local path="$1"
  local value="$2"
  local description="$3"
  if grep --fixed-strings --quiet -- "${value}" "${path}"; then
    echo "${description}" >&2
    return 1
  fi
}

CANARY="c2-capability-canary-9f74d4"
QUERY_CANARY="query-canary-44d8"

docker run --detach \
  --name "${NGINX_CONTAINER}" \
  --add-host api:127.0.0.1 \
  --publish 127.0.0.1::8080 \
  --volume "${ROOT_DIR}/deploy/nginx:/etc/nginx/conf.d:ro" \
  "${NGINX_IMAGE}" >/dev/null
nginx_address="$(docker port "${NGINX_CONTAINER}" 8080/tcp)"
nginx_port="${nginx_address##*:}"
wait_for_url "http://127.0.0.1:${nginx_port}/"
curl --silent --show-error \
  "http://127.0.0.1:${nginx_port}/api/v1/invites/${CANARY}?token=${QUERY_CANARY}" \
  >/dev/null 2>&1 || true
docker logs "${NGINX_CONTAINER}" >"${WORK_DIR}/nginx.stdout" 2>"${WORK_DIR}/nginx.stderr"
assert_absent "${WORK_DIR}/nginx.stdout" "${CANARY}" \
  "Nginx stdout leaked the synthetic capability"
assert_absent "${WORK_DIR}/nginx.stderr" "${CANARY}" \
  "Nginx stderr leaked the synthetic capability"
assert_absent "${WORK_DIR}/nginx.stdout" "${QUERY_CANARY}" \
  "Nginx stdout leaked the synthetic capability query"
assert_absent "${WORK_DIR}/nginx.stderr" "${QUERY_CANARY}" \
  "Nginx stderr leaked the synthetic capability query"

capture_path="${WORK_DIR}/upstream.json"
upstream_port="$(free_port)"
CAPTURE_PATH="${capture_path}" UPSTREAM_PORT="${upstream_port}" node -e '
  const fs = require("node:fs");
  const http = require("node:http");
  http.createServer((request, response) => {
    fs.writeFileSync(
      process.env.CAPTURE_PATH,
      JSON.stringify({
        forwarded: request.headers.forwarded ?? null,
        referer: request.headers.referer ?? null,
        xForwardedFor: request.headers["x-forwarded-for"] ?? null,
        xRealIp: request.headers["x-real-ip"] ?? null,
      }),
    );
    response.writeHead(200, { "content-type": "application/json" });
    response.end("{\"status\":\"ok\"}");
  }).listen(Number(process.env.UPSTREAM_PORT), "127.0.0.1");
' >"${WORK_DIR}/upstream.stdout" 2>"${WORK_DIR}/upstream.stderr" &
UPSTREAM_PID=$!

vite_port="$(free_port)"
CUMURU_VITE_PROXY_TARGET="http://127.0.0.1:${upstream_port}" \
  npm --workspace @cumuru/web exec -- \
    vite --host 127.0.0.1 --port "${vite_port}" --strictPort \
    >"${WORK_DIR}/vite.stdout" 2>"${WORK_DIR}/vite.stderr" &
VITE_PID=$!
wait_for_url "http://127.0.0.1:${vite_port}/"
curl --fail --silent --show-error \
  --header "Forwarded: for=198.51.100.8;proto=https" \
  --header "Referer: https://sensitive.invalid/invites/token" \
  --header "X-Forwarded-For: 198.51.100.8" \
  --header "X-Real-IP: 198.51.100.8" \
  "http://127.0.0.1:${vite_port}/api/v1/platform/health" >/dev/null

CAPTURE_PATH="${capture_path}" node -e '
  const fs = require("node:fs");
  const captured = JSON.parse(fs.readFileSync(process.env.CAPTURE_PATH, "utf8"));
  const loopback = new Set(["127.0.0.1", "::1", "::ffff:127.0.0.1"]);
  if (!loopback.has(captured.xForwardedFor) || !loopback.has(captured.xRealIp)) {
    process.exit(1);
  }
  if (captured.forwarded !== null || captured.referer !== null) {
    process.exit(1);
  }
'

kill "${UPSTREAM_PID}"
wait "${UPSTREAM_PID}" >/dev/null 2>&1 || true
UPSTREAM_PID=""
curl --silent --show-error \
  "http://127.0.0.1:${vite_port}/api/v1/invites/${CANARY}?token=${QUERY_CANARY}" \
  >/dev/null 2>&1 || true
sleep 1
assert_absent "${WORK_DIR}/vite.stdout" "${CANARY}" \
  "Vite stdout leaked the synthetic capability"
assert_absent "${WORK_DIR}/vite.stderr" "${CANARY}" \
  "Vite stderr leaked the synthetic capability"
assert_absent "${WORK_DIR}/vite.stdout" "${QUERY_CANARY}" \
  "Vite stdout leaked the synthetic capability query"
assert_absent "${WORK_DIR}/vite.stderr" "${QUERY_CANARY}" \
  "Vite stderr leaked the synthetic capability query"
assert_absent "${WORK_DIR}/vite.stdout" "ECONNREFUSED" \
  "Vite stdout leaked proxy error data"
assert_absent "${WORK_DIR}/vite.stderr" "ECONNREFUSED" \
  "Vite stderr leaked proxy error data"

grep --fixed-strings --quiet \
  'proxy_set_header X-Forwarded-For $remote_addr;' \
  "${ROOT_DIR}/deploy/nginx/default.conf"
grep --fixed-strings --quiet \
  'proxy_set_header X-Real-IP $remote_addr;' \
  "${ROOT_DIR}/deploy/nginx/default.conf"
grep --fixed-strings --quiet \
  'proxy_set_header Forwarded "";' \
  "${ROOT_DIR}/deploy/nginx/default.conf"

echo "Nginx and Vite proxy hardening passed"
