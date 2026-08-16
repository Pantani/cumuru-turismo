#!/usr/bin/env bash
set -euo pipefail

# Phase 7 (autoatendimento e aprovação) owns this script. The wave's platform
# builder replaces this body with the real PostgreSQL integration run covering
# reusable accommodation invites, self-service stay provenance, the approval
# queue and the presence filter.
#
# It fails closed on purpose: a green pipeline must never imply self-service
# coverage that does not exist yet.

echo "PHASE7_INTEGRATION=UNIMPLEMENTED" >&2
echo "owner=cumuru-platform-builder" >&2
echo "spec=.agents/skills/cumuru-bootstrap/references/phase-matrix.md" >&2
exit 2
