#!/usr/bin/env bash
set -euo pipefail

# Phase 7 (autoatendimento e aprovação) owns this script. The wave's platform
# builder replaces this body with the real full-stack run: accommodation
# activation by single-use capability, open registration through the reusable
# QR, approval by the establishment and the aggregate filter that keeps
# unapproved stays out of public data.
#
# It fails closed on purpose: a green pipeline must never imply self-service
# coverage that does not exist yet.

echo "PHASE7_FULL_STACK=UNIMPLEMENTED" >&2
echo "owner=cumuru-platform-builder" >&2
echo "spec=.agents/skills/cumuru-bootstrap/references/phase-matrix.md" >&2
exit 2
