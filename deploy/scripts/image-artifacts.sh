#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-}"
TRIVY_IMAGE="${2:-}"
API_IMAGE="${3:-}"
WEB_IMAGE="${4:-}"
OUTPUT_DIR="${ROOT_DIR}/artifacts/sbom"
MATERIALIZE_PINNED_IMAGE="${ROOT_DIR}/deploy/scripts/materialize-pinned-image.sh"

fail() {
  echo "image artifacts: $*" >&2
  exit 2
}

hash_file() {
  local path="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${path}" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${path}" | awk '{print $1}'
    return
  fi
  fail "shasum or sha256sum is required"
}

inspect_required() {
  local image="$1"
  docker image inspect "${image}" >/dev/null 2>&1 ||
    fail "required image is not available: ${image}"
}

repository_name() {
  local reference="${1%@*}"
  local final_segment="${reference##*/}"
  if [[ "${final_segment}" == *:* ]]; then
    printf '%s\n' "${reference%:*}"
    return
  fi
  printf '%s\n' "${reference}"
}

matching_repo_digest() {
  local image="$1"
  local repo_digests="$2"
  local expected
  if [[ "${image}" != *@sha256:* ]]; then
    return 0
  fi
  expected="$(repository_name "${image}")@${image##*@}"
  while IFS= read -r candidate; do
    if test "${candidate}" = "${expected}"; then
      printf '%s\n' "${candidate}"
      return
    fi
  done <<<"${repo_digests}"
  return 0
}

repo_digest() {
  local image="$1"
  local repo_digests
  repo_digests="$(
    docker image inspect "${image}" \
      --format '{{range .RepoDigests}}{{println .}}{{end}}'
  )"
  matching_repo_digest "${image}" "${repo_digests}"
}

label_value() {
  local image="$1"
  local label="$2"
  docker image inspect "${image}" \
    --format "{{index .Config.Labels \"${label}\"}}"
}

verify_build_labels() {
  local image="$1"
  local actual_version actual_revision actual_created
  actual_version="$(label_value "${image}" org.opencontainers.image.version)"
  actual_revision="$(label_value "${image}" org.opencontainers.image.revision)"
  actual_created="$(label_value "${image}" org.opencontainers.image.created)"
  test "${actual_version}" = "${CUMURU_BUILD_VERSION}" ||
    fail "${image} version label does not match build metadata"
  test "${actual_revision}" = "${CUMURU_BUILD_REVISION}" ||
    fail "${image} revision label does not match build metadata"
  test "${actual_created}" = "${CUMURU_BUILD_TIME}" ||
    fail "${image} created label does not match build metadata"
}

run_self_test() {
  local digest="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  local other_digest="sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  local image="registry.example:5000/cumuru/api:release@${digest}"
  local expected="registry.example:5000/cumuru/api@${digest}"
  local actual
  actual="$(
    matching_repo_digest "${image}" \
      "other.example/cumuru/api@${digest}
${expected}"
  )"
  test "${actual}" = "${expected}" ||
    fail "matching registry digest fixture was rejected"
  actual="$(matching_repo_digest "${image}" "other.example/cumuru/api@${digest}")"
  test -z "${actual}" ||
    fail "mismatched repository fixture was accepted"
  actual="$(
    matching_repo_digest \
      "${image}" \
      "registry.example:5000/cumuru/api@${other_digest}"
  )"
  test -z "${actual}" ||
    fail "mismatched digest fixture was accepted"
  actual="$(matching_repo_digest "cumuru-api:local" "${expected}")"
  test -z "${actual}" ||
    fail "local image fixture was assigned a registry digest"
  echo "IMAGE_ARTIFACTS_SELF_TEST=PASS"
}

if test "${MODE}" = "self-test"; then
  run_self_test
  exit 0
fi

test "${MODE}" = "scan" || test "${MODE}" = "sbom" ||
  fail "usage: image-artifacts.sh scan|sbom|self-test TRIVY_IMAGE API_IMAGE WEB_IMAGE"
test -n "${TRIVY_IMAGE}" || fail "TRIVY_IMAGE is required"
test -n "${API_IMAGE}" || fail "API_IMAGE is required"
test -n "${WEB_IMAGE}" || fail "WEB_IMAGE is required"
[[ "${TRIVY_IMAGE}" == *@sha256:* ]] ||
  fail "Trivy image must be pinned by digest"
: "${CUMURU_BUILD_VERSION:?missing CUMURU_BUILD_VERSION}"
: "${CUMURU_BUILD_REVISION:?missing CUMURU_BUILD_REVISION}"
: "${CUMURU_BUILD_TIME:?missing CUMURU_BUILD_TIME}"

"${MATERIALIZE_PINNED_IMAGE}" "${TRIVY_IMAGE}"
inspect_required "${TRIVY_IMAGE}"
inspect_required "${API_IMAGE}"
inspect_required "${WEB_IMAGE}"
verify_build_labels "${API_IMAGE}"
verify_build_labels "${WEB_IMAGE}"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-image-artifacts.XXXXXX")"
mkdir -p "${WORK_DIR}/cache"
cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

docker image save --output "${WORK_DIR}/api.tar" "${API_IMAGE}"
docker image save --output "${WORK_DIR}/web.tar" "${WEB_IMAGE}"

run_trivy() {
  local tar_name="$1"
  shift
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    --volume "${WORK_DIR}:/input:ro" \
    --volume "${WORK_DIR}/cache:/cache" \
    --volume "${OUTPUT_DIR}:/output" \
    "${TRIVY_IMAGE}" image \
    --input "/input/${tar_name}" \
    --cache-dir /cache \
    --skip-version-check \
    --no-progress \
    "$@"
}

if [[ "${MODE}" == "scan" ]]; then
  mkdir -p "${OUTPUT_DIR}"
  run_trivy api.tar \
    --scanners vuln --exit-code 1 --severity HIGH,CRITICAL
  run_trivy web.tar \
    --scanners vuln --exit-code 1 --severity HIGH,CRITICAL
  exit 0
fi

mkdir -p "${OUTPUT_DIR}"
run_trivy api.tar \
  --scanners vuln --format cyclonedx --output /output/api-image.cdx.json
run_trivy web.tar \
  --scanners vuln --format cyclonedx --output /output/web-image.cdx.json

api_id="$(docker image inspect "${API_IMAGE}" --format '{{.Id}}')"
web_id="$(docker image inspect "${WEB_IMAGE}" --format '{{.Id}}')"
api_repo_digest="$(repo_digest "${API_IMAGE}")"
web_repo_digest="$(repo_digest "${WEB_IMAGE}")"

[[ "${api_id}" == sha256:* ]] || fail "API image has no immutable content ID"
[[ "${web_id}" == sha256:* ]] || fail "web image has no immutable content ID"
[[ -z "${api_repo_digest}" || "${api_repo_digest}" =~ @sha256:[a-f0-9]{64}$ ]] ||
  fail "API image exposes an invalid RepoDigest"
[[ -z "${web_repo_digest}" || "${web_repo_digest}" =~ @sha256:[a-f0-9]{64}$ ]] ||
  fail "web image exposes an invalid RepoDigest"
if [[ "${API_IMAGE}" == *@sha256:* && -z "${api_repo_digest}" ]]; then
  fail "registry-pinned API image has no RepoDigest"
fi
if [[ "${WEB_IMAGE}" == *@sha256:* && -z "${web_repo_digest}" ]]; then
  fail "registry-pinned web image has no RepoDigest"
fi

export API_IMAGE WEB_IMAGE api_id web_id api_repo_digest web_repo_digest
export API_SBOM_SHA256="$(hash_file "${OUTPUT_DIR}/api-image.cdx.json")"
export WEB_SBOM_SHA256="$(hash_file "${OUTPUT_DIR}/web-image.cdx.json")"
export MANIFEST_PATH="${OUTPUT_DIR}/image-manifest.json"

node <<'NODE'
const fs = require("node:fs");

const optional = (value) => value === "" ? null : value;
const distributionDigest = (imageRef, repoDigest) =>
  imageRef.includes("@sha256:") ? optional(repoDigest) : null;
const provenanceScope = (imageRef) =>
  imageRef.includes("@sha256:") ? "registry" : "local-build";
const manifest = {
  schema_version: 2,
  build: {
    version: process.env.CUMURU_BUILD_VERSION,
    revision: process.env.CUMURU_BUILD_REVISION,
    built_at: process.env.CUMURU_BUILD_TIME,
  },
  artifacts: [
    {
      role: "api-worker",
      image_ref: process.env.API_IMAGE,
      content_digest: process.env.api_id,
      repo_digest: distributionDigest(
        process.env.API_IMAGE,
        process.env.api_repo_digest,
      ),
      provenance_scope: provenanceScope(process.env.API_IMAGE),
      sbom: {
        path: "api-image.cdx.json",
        sha256: process.env.API_SBOM_SHA256,
      },
    },
    {
      role: "web",
      image_ref: process.env.WEB_IMAGE,
      content_digest: process.env.web_id,
      repo_digest: distributionDigest(
        process.env.WEB_IMAGE,
        process.env.web_repo_digest,
      ),
      provenance_scope: provenanceScope(process.env.WEB_IMAGE),
      sbom: {
        path: "web-image.cdx.json",
        sha256: process.env.WEB_SBOM_SHA256,
      },
    },
  ],
};

fs.writeFileSync(
  process.env.MANIFEST_PATH,
  `${JSON.stringify(manifest, null, 2)}\n`,
  { encoding: "utf8", mode: 0o644 },
);
NODE
