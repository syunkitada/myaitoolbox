#!/usr/bin/env bash
# CLI end-to-end test: exercises the mybox CLI against an isolated temp project.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
PROJ="$TMP/proj"
mkdir -p "$PROJ"

export MYBOX_CONFIG="$TMP/config.yaml"

if [ ! -x "$ROOT/mybox" ]; then
  make -C "$ROOT" build
fi

BIN="$ROOT/mybox"
PASS=0
FAIL=0

step() { printf '\n== %s ==\n' "$1"; }
ok() { printf 'ok: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
check() { # check <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then ok "$1"; else fail "$1 (expected [$2], got [$3])"; fi
}

step "project add / list"
"$BIN" project add "$PROJ"
projects="$("$BIN" project list)"
case "$projects" in *proj*) ok "project listed";; *) fail "project listed";; esac

step "task create / list / show"
id="$("$BIN" task create --project proj --name 'Test task' | tail -1)"
case "$id" in *test-task*) ok "task created id";; *) fail "task created id";; esac
tasks="$("$BIN" task list --project proj)"
case "$tasks" in *test-task*) ok "task listed";; *) fail "task listed";; esac
show="$("$BIN" task show --project proj "$id")"
case "$show" in *Test\ task*) ok "task shown";; *) fail "task shown";; esac

step "task set / archive"
"$BIN" task set --project proj "$id" --status doing --priority high >/dev/null
front="$(cat "$PROJ/tasks/$id/task.md")"
case "$front" in *"status: doing"*) ok "task set wrote status";; *) fail "task set wrote status";; esac
case "$front" in *"priority: high"*) ok "task set wrote priority";; *) fail "task set wrote priority";; esac
"$BIN" task archive --project proj "$id" >/dev/null
archived="$("$BIN" task list --project proj --all)"
case "$archived" in *test-task*) ok "archived task visible with --all";; *) fail "archived task visible with --all";; esac
active="$("$BIN" task list --project proj)"
case "$active" in *test-task*) fail "archived task hidden by default";; *) ok "archived task hidden by default";; esac

step "legacy adhoc layout read / create"
adhoc_id="$("$BIN" task create --project proj --name 'Review PR #123' | tail -1)"
[ -f "$PROJ/tasks/$adhoc_id/task.md" ] && ok "task stored in regular layout" || fail "task stored in regular layout"
adhoc_front="$(cat "$PROJ/tasks/$adhoc_id/task.md")"
case "$adhoc_front" in *"task_kind"*) fail "no task_kind in frontmatter";; *) ok "no task_kind in frontmatter";; esac
adhoc_list="$("$BIN" task list --project proj)"
case "$adhoc_list" in *review-pr-123*) ok "task listed";; *) fail "task listed";; esac
"$BIN" task archive --project proj "$adhoc_id" >/dev/null
[ -f "$PROJ/archives/tasks/$adhoc_id/task.md" ] && ok "task archived" || fail "task archived"
archived_adhoc="$("$BIN" task list --project proj --all)"
case "$archived_adhoc" in *review-pr-123*) ok "archived task listed with --all";; *) fail "archived task listed with --all";; esac

step "files create / show / mkdir / move / rename"
path="$("$BIN" files create --project proj 'notes/alpha.md' | tail -1)"
check "files created path" "notes/alpha.md" "$path"
printf '# Alpha\n' >"$PROJ/notes/alpha.md"
show="$("$BIN" files show --project proj "$path")"
case "$show" in *Alpha*) ok "files shown";; *) fail "files shown";; esac
"$BIN" files mkdir --project proj 'docs' >/dev/null
[ -d "$PROJ/docs" ] && ok "files mkdir" || fail "files mkdir"
"$BIN" files move --project proj "$path" 'notes/beta.md' >/dev/null
[ -f "$PROJ/notes/beta.md" ] && [ ! -f "$PROJ/notes/alpha.md" ] && ok "files moved" || fail "files moved"
"$BIN" files rename --project proj 'notes/beta.md' 'gamma.md' >/dev/null
[ -f "$PROJ/notes/gamma.md" ] && ok "files renamed" || fail "files renamed"

step "serve smoke"
"$BIN" serve --project proj --port 18099 --no-browser &
SERVE_PID=$!
trap 'kill $SERVE_PID 2>/dev/null' EXIT
for _ in $(seq 1 50); do
  if curl -sf "http://127.0.0.1:18099/api/meta" >/dev/null 2>&1; then break; fi
  sleep 0.1
done
meta="$(curl -sf "http://127.0.0.1:18099/api/meta")"
case "$meta" in *proj*) ok "serve /api/meta";; *) fail "serve /api/meta";; esac
kill $SERVE_PID 2>/dev/null
wait $SERVE_PID 2>/dev/null
trap - EXIT

printf '\nCLI E2E: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
