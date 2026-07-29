#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TERRAFORM_DIR="${TERRAFORM_DIR:-${ROOT_DIR}/deploy/terraform/aws}"
CUMURU_RELEASE_TAG="${CUMURU_RELEASE_TAG:-}"
CUMURU_PLATFORM="${CUMURU_PLATFORM:-linux/arm64}"

fail() {
  echo "publish images: $*" >&2
  exit 2
}

for command_name in aws docker terraform; do
  command -v "${command_name}" >/dev/null 2>&1 ||
    fail "${command_name} is required"
done

: "${CUMURU_BUILD_VERSION:?build metadata wrapper is required}"
: "${CUMURU_BUILD_REVISION:?build metadata wrapper is required}"
: "${CUMURU_BUILD_TIME:?build metadata wrapper is required}"

[[ "${CUMURU_RELEASE_TAG}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] ||
  fail "CUMURU_RELEASE_TAG must be an immutable tag"
[[ "${CUMURU_RELEASE_TAG}" != "latest" ]] ||
  fail "latest is forbidden"

output() {
  terraform -chdir="${TERRAFORM_DIR}" output -raw "$1"
}

region="$(output aws_region)"
api_repository="$(output api_repository_url)"
web_repository="$(output web_repository_url)"
registry="${api_repository%%/*}"

aws ecr get-login-password --region "${region}" |
  docker login --username AWS --password-stdin "${registry}" >/dev/null

docker buildx inspect >/dev/null 2>&1 ||
  fail "docker buildx is required"

docker buildx build \
  --platform "${CUMURU_PLATFORM}" \
  --build-arg "BUILD_VERSION=${CUMURU_BUILD_VERSION}" \
  --build-arg "BUILD_REVISION=${CUMURU_BUILD_REVISION}" \
  --build-arg "BUILD_TIME=${CUMURU_BUILD_TIME}" \
  --file "${ROOT_DIR}/apps/api/Dockerfile" \
  --tag "${api_repository}:${CUMURU_RELEASE_TAG}" \
  --push \
  "${ROOT_DIR}"

docker buildx build \
  --platform "${CUMURU_PLATFORM}" \
  --build-arg "BUILD_VERSION=${CUMURU_BUILD_VERSION}" \
  --build-arg "BUILD_REVISION=${CUMURU_BUILD_REVISION}" \
  --build-arg "BUILD_TIME=${CUMURU_BUILD_TIME}" \
  --build-arg "VITE_API_BASE_URL=/api/v1" \
  --file "${ROOT_DIR}/apps/web/Dockerfile" \
  --tag "${web_repository}:${CUMURU_RELEASE_TAG}" \
  --push \
  "${ROOT_DIR}"

for repository in "${api_repository##*/}" "${web_repository##*/}"; do
  digest="$(
    aws ecr describe-images \
      --region "${region}" \
      --repository-name "${repository}" \
      --image-ids "imageTag=${CUMURU_RELEASE_TAG}" \
      --query 'imageDetails[0].imageDigest' \
      --output text
  )"
  [[ "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]] ||
    fail "ECR did not return a digest for ${repository}"
  printf '%s:%s@%s\n' "${registry}/${repository}" "${CUMURU_RELEASE_TAG}" "${digest}"
done
