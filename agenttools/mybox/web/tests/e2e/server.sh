#!/usr/bin/env bash
# Starts the mybox API + Web UI for Playwright E2E tests with an isolated
# temp project that has some seed data.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TMP="$(mktemp -d)"
PROJ="$TMP/proj"
mkdir -p "$PROJ"

export MYBOX_CONFIG="$TMP/config.yaml"

if [ ! -x "$ROOT/mybox" ]; then
  make -C "$ROOT" build
fi

"$ROOT/mybox" project add "$PROJ"
"$ROOT/mybox" task create --project proj --name "Ship the web UI"
"$ROOT/mybox" task create --project proj --name "Write E2E tests"
"$ROOT/mybox" task create --project proj --name "E2E status change target"
mkdir -p "$PROJ/tasks/e2e-status-change-target"
printf -- '---\nstatus: doing\n---\n\n# Drag me to done\n' \
  >"$PROJ/tasks/e2e-status-change-target/task.md"
mkdir -p "$PROJ/knowledge/notes"
printf '# Mybox\n\nWelcome to the workspace.\n\nSee [[notes/phase6]] and [[tasks]].\n' \
  >"$PROJ/knowledge/index.md"
printf '# Phase 6\n\n## Overview\n\nThe HTTP API is done.\n\n## Diagram\n\n```mermaid\ngraph LR\n  A[API] --> B[UI]\n```\n' >"$PROJ/knowledge/notes/phase6.md"
printf '# Tasks\n\nTask tracking lives here.\n' >"$PROJ/tasks.md"

exec "$ROOT/mybox" serve --project proj --port 19090 --no-browser
