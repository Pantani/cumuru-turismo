#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CADDY_IMAGE="caddy:2.10.2-alpine@sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d"
OTEL_IMAGE="otel/opentelemetry-collector-contrib:0.157.0@sha256:f2f01157055a9b2aab9df7118e1f1c9abf345e99b23bc7a2bc791db374a7d0f6"
PROMETHEUS_IMAGE="prom/prometheus:v3.13.0@sha256:c6b27ea434f8389bfe233fbc7be381cf50587c286e871bc842008f5a1b1908a7"
TEMPO_IMAGE="grafana/tempo:2.10.5@sha256:ee21727732c7a7199cb71c3eee9153bbf23f9b0b87619f0555a0cf21a67f1a33"
VALIDATION_DIGEST="sha256:0000000000000000000000000000000000000000000000000000000000000000"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

fail() {
  echo "infrastructure validation: $*" >&2
  exit 2
}

require_pinned_image() {
  local image="$1"
  local source="$2"
  [[ "${image}" =~ ^[^@[:space:]]+:[^@[:space:]]+@sha256:[a-f0-9]{64}$ ]] ||
    fail "mutable or invalid image in ${source}: ${image}"
}

validate_image_list() {
  local source="$1"
  local images="$2"
  local image
  test -n "${images}" || fail "no images rendered from ${source}"
  while IFS= read -r image; do
    case "${image}" in
      cumuru-api:* | cumuru-web:*) continue ;;
    esac
    require_pinned_image "${image}" "${source}"
  done <<<"${images}"
}

if (
  require_pinned_image "example.invalid/mutable:latest" "negative fixture"
) >/dev/null 2>&1; then
  fail "mutable image negative fixture was accepted"
fi

proxy_image="$(
  sed -n 's/^NGINX_IMAGE="\([^"]*\)"$/\1/p' \
    "${ROOT_DIR}/deploy/scripts/test-proxy-hardening.sh"
)"
test -n "${proxy_image}" ||
  fail "proxy hardening test does not declare NGINX_IMAGE"
require_pinned_image "${proxy_image}" "proxy hardening test"
"${ROOT_DIR}/deploy/scripts/image-artifacts.sh" self-test

cp "${ROOT_DIR}/deploy/ansible/inventory/hosts.yml.example" \
  "${tmp_dir}/hosts.yml"

terraform -chdir="${ROOT_DIR}/deploy/terraform/bootstrap-state" fmt -check -recursive
terraform -chdir="${ROOT_DIR}/deploy/terraform/bootstrap-state" init -backend=false
terraform -chdir="${ROOT_DIR}/deploy/terraform/bootstrap-state" validate

terraform -chdir="${ROOT_DIR}/deploy/terraform/aws" fmt -check -recursive
terraform -chdir="${ROOT_DIR}/deploy/terraform/aws" init -backend=false
terraform -chdir="${ROOT_DIR}/deploy/terraform/aws" validate

ANSIBLE_CONFIG="${ROOT_DIR}/deploy/ansible/ansible.cfg" \
  ansible-playbook \
    --inventory "${tmp_dir}/hosts.yml" \
    "${ROOT_DIR}/deploy/ansible/playbooks/site.yml" \
    --syntax-check

"${ROOT_DIR}/deploy/scripts/local-infra.sh" config
local_images="$(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh" \
    docker compose \
      --project-directory "${ROOT_DIR}" \
      --file "${ROOT_DIR}/compose.yaml" \
      --file "${ROOT_DIR}/deploy/compose.observability.yaml" \
      config --images
)"
validate_image_list "local Compose" "${local_images}"

while IFS= read -r image; do
  require_pinned_image "${image}" "Dockerfile"
done < <(awk '$1 == "FROM" { print $2 }' \
  "${ROOT_DIR}/apps/api/Dockerfile" \
  "${ROOT_DIR}/apps/web/Dockerfile")

while IFS= read -r image; do
  require_pinned_image "${image}" "Ansible defaults"
done < <(awk '
  $1 ~ /^cumuru_(postgres_admin|migrate|caddy)_image:$/ { print $2 }
' "${ROOT_DIR}/deploy/ansible/roles/cumuru_host/defaults/main.yml")

docker run --rm \
  --env CUMURU_DOMAIN=example.invalid \
  --env ACME_EMAIL=infra@example.invalid \
  --volume "${ROOT_DIR}/deploy/ansible/roles/cumuru_host/templates/Caddyfile.j2:/etc/caddy/Caddyfile:ro" \
  "${CADDY_IMAGE}" \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile

docker run --rm \
  --volume "${ROOT_DIR}/deploy/observability/otel-collector.yaml:/etc/config.yaml:ro" \
  "${OTEL_IMAGE}" \
  validate --config=/etc/config.yaml

docker run --rm \
  --volume "${ROOT_DIR}/deploy/observability/prometheus.yaml:/etc/prometheus/prometheus.yml:ro" \
  --entrypoint /bin/promtool \
  "${PROMETHEUS_IMAGE}" \
  check config /etc/prometheus/prometheus.yml

docker run --rm \
  --volume "${ROOT_DIR}/deploy/observability/tempo.yaml:/etc/tempo.yaml:ro" \
  "${TEMPO_IMAGE}" \
  -config.file=/etc/tempo.yaml \
  -config.verify=true

touch "${tmp_dir}/runtime.env"
mkdir -p "${tmp_dir}/migrations" "${tmp_dir}/secrets"
touch "${tmp_dir}/Caddyfile" "${tmp_dir}/secrets/rds-global-bundle.pem"

CUMURU_API_IMAGE="example.invalid/cumuru-api:validation@${VALIDATION_DIGEST}" \
CUMURU_WEB_IMAGE="example.invalid/cumuru-web:validation@${VALIDATION_DIGEST}" \
CUMURU_DOMAIN=example.invalid \
ACME_EMAIL=infra@example.invalid \
MIGRATION_DATABASE_URL='postgresql://validation:validation@db.example.invalid:5432/cumuru?sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem' \
  docker compose \
    --project-directory "${tmp_dir}" \
    --file "${ROOT_DIR}/deploy/compose.production.yaml" \
    config \
    --quiet

production_images="$(
  CUMURU_API_IMAGE="example.invalid/cumuru-api:validation@${VALIDATION_DIGEST}" \
  CUMURU_WEB_IMAGE="example.invalid/cumuru-web:validation@${VALIDATION_DIGEST}" \
  CUMURU_DOMAIN=example.invalid \
  ACME_EMAIL=infra@example.invalid \
  MIGRATION_DATABASE_URL='postgresql://validation:validation@db.example.invalid:5432/cumuru?sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem' \
    docker compose \
      --project-directory "${tmp_dir}" \
      --file "${ROOT_DIR}/deploy/compose.production.yaml" \
      config --images
)"
validate_image_list "production Compose" "${production_images}"

echo "INFRA_VALIDATION=PASS"
