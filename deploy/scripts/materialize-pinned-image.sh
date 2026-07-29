#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "pinned image: $*" >&2
  exit 2
}

test "$#" -gt 0 || fail "at least one image reference is required"

for image in "$@"; do
  [[ "${image}" =~ ^[^[:space:]@]+:[^[:space:]@]+@sha256:[a-f0-9]{64}$ ]] ||
    fail "image reference must contain an exact tag and sha256 digest: ${image}"

  expected_digest="${image##*@}"

  # Pulling tag@digest lets the registry client verify content before any local
  # inspection. A missing registry, mismatched digest or unavailable manifest
  # fails this command and therefore the caller's gate.
  docker image pull "${image}" >/dev/null ||
    fail "could not materialize pinned image: ${image}"

  repo_digests="$(
    docker image inspect "${image}" \
      --format '{{range .RepoDigests}}{{println .}}{{end}}'
  )" || fail "could not inspect materialized image: ${image}"

  if ! grep -Fq "@${expected_digest}" <<<"${repo_digests}"; then
    fail "materialized image does not expose expected digest: ${image}"
  fi
done
