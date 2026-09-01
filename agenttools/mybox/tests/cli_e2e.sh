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

step "adhoc task create / list / filter"
adhoc_id="$("$BIN" task create --project proj --name 'Review PR #123' --adhoc | tail -1)"
[ -f "$PROJ/tasks/adhoc/$adhoc_id.md" ] && ok "adhoc task stored as single file" || fail "adhoc task stored as single file"
adhoc_front="$(cat "$PROJ/tasks/adhoc/$adhoc_id.md")"
case "$adhoc_front" in *"type: adhoc"*) ok "adhoc task frontmatter type";; *) fail "adhoc task frontmatter type";; esac
adhoc_list="$("$BIN" task list --project proj --type adhoc)"
case "$adhoc_list" in *review-pr-123*) ok "adhoc task listed with --type adhoc";; *) fail "adhoc task listed with --type adhoc";; esac
regular_list="$("$BIN" task list --project proj --type regular)"
case "$regular_list" in *review-pr-123*) fail "regular filter excludes adhoc task";; *) ok "regular filter excludes adhoc task";; esac
if "$BIN" task archive --project proj "$adhoc_id" >/dev/null 2>&1; then
  fail "adhoc archive rejected"
else
  ok "adhoc archive rejected"
fi

step "knowledge create / show / move / rename"
path="$("$BIN" knowledge create --project proj 'notes/alpha' | tail -1)"
check "knowledge created path" "notes/alpha" "$path"
show="$("$BIN" knowledge show --project proj "$path")"
case "$show" in *alpha*) ok "knowledge shown";; *) fail "knowledge shown";; esac
"$BIN" knowledge move --project proj "$path" 'notes/beta' >/dev/null
[ -f "$PROJ/knowledge/notes/beta.md" ] && [ ! -f "$PROJ/knowledge/notes/alpha.md" ] && ok "knowledge moved" || fail "knowledge moved"
"$BIN" knowledge rename --project proj 'notes/beta' 'gamma' >/dev/null
[ -f "$PROJ/knowledge/notes/gamma.md" ] && ok "knowledge renamed" || fail "knowledge renamed"

step "search (task + knowledge)"
"$BIN" task create --project proj --name 'OAuth login handler' >/dev/null
"$BIN" knowledge create --project proj 'docs/oauth' >/dev/null
printf '# OAuth notes\n\nImplement the oauth flow.\n' >"$PROJ/knowledge/docs/oauth.md"
task_hits="$("$BIN" search --project proj oauth --type task)"
case "$task_hits" in *oauth-login-handler*) ok "task search hit";; *) fail "task search hit";; esac
k_hits="$("$BIN" search --project proj oauth --type knowledge)"
case "$k_hits" in *docs/oauth*) ok "knowledge search hit";; *) fail "knowledge search hit";; esac
both="$("$BIN" search --project proj oauth --json)"
case "$both" in *oauth-login-handler*) ok "cross-type search includes task";; *) fail "cross-type search includes task";; esac
case "$both" in *docs/oauth*) ok "cross-type search includes knowledge";; *) fail "cross-type search includes knowledge";; esac

step "serve smoke (read-only rejects writes)"
"$BIN" serve --project proj --port 18099 --no-browser --read-only &
SERVE_PID=$!
trap 'kill $SERVE_PID 2>/dev/null' EXIT
for _ in $(seq 1 50); do
  if curl -sf "http://127.0.0.1:18099/api/meta" >/dev/null 2>&1; then break; fi
  sleep 0.1
done
meta="$(curl -sf "http://127.0.0.1:18099/api/meta")"
case "$meta" in *proj*) ok "serve /api/meta";; *) fail "serve /api/meta";; esac
code="$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:18099/api/tasks" -H 'Content-Type: application/json' -d '{"name":"x"}')"
check "read-only rejects writes (403)" "403" "$code"
kill $SERVE_PID 2>/dev/null
wait $SERVE_PID 2>/dev/null
trap - EXIT

printf '\nCLI E2E: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
