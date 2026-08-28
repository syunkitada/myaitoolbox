#!/usr/bin/env bash
# Starts the mybox API + Web UI for Playwright E2E tests with an isolated
# temp project that has some seed data.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TMP="$(mktemp -d)"
PROJ="$TMP/proj"
mkdir -p "$PROJ"

# Stub `herdr` CLI so e2e tests are deterministic and independent of a live
# herdr server on this machine. Prepended to PATH for the mybox process.
# Stateful: tab/pane/workspace mutations are persisted under $STUB_DIR.
STUB="$TMP/herdr-stub"
mkdir -p "$STUB"
cat >"$STUB/herdr" <<'STUBEOF'
#!/usr/bin/env python3
import json, os, sys

STATE = os.environ["STUB_DIR"]

def load(name, default):
    path = os.path.join(STATE, name)
    if not os.path.exists(path):
        return default
    with open(path) as f:
        return json.load(f)

def save(name, data):
    with open(os.path.join(STATE, name), "w") as f:
        json.dump(data, f)

def out(envelope_id, result_type, key, items):
    print(json.dumps({"id": envelope_id, "result": {key: items, "type": result_type}}))

def seed():
    save("workspaces.json", [
        {"active_tab_id": "w5:t1", "agent_status": "unknown", "focused": False, "label": "home_ex", "number": 1, "pane_count": 1, "tab_count": 1, "workspace_id": "w5"},
        {"active_tab_id": "w7:t1", "agent_status": "working", "focused": True, "label": "proj", "number": 2, "pane_count": 2, "tab_count": 2, "workspace_id": "w7"},
    ])
    save("tabs.json", [
        {"agent_status": "unknown", "focused": False, "label": "1", "number": 1, "pane_count": 1, "tab_id": "w5:t1", "workspace_id": "w5"},
        {"agent_status": "working", "focused": True, "label": "1", "number": 1, "pane_count": 1, "tab_id": "w7:t1", "workspace_id": "w7"},
        {"agent_status": "unknown", "focused": False, "label": "2", "number": 2, "pane_count": 1, "tab_id": "w7:t2", "workspace_id": "w7"},
    ])
    save("panes.json", [
        {"agent_status": "unknown", "cwd": "/home/stub/home-dir", "focused": False, "pane_id": "w5:p1", "tab_id": "w5:t1", "workspace_id": "w5"},
        {"agent_status": "working", "cwd": "/home/stub/proj-dir", "focused": True, "pane_id": "w7:p1", "tab_id": "w7:t1", "workspace_id": "w7"},
        {"agent_status": "unknown", "cwd": "/tmp/stub", "focused": False, "pane_id": "w7:p2", "tab_id": "w7:t2", "workspace_id": "w7"},
    ])

def opt(args, flag):
    if flag in args:
        i = args.index(flag)
        return args[i + 1]
    return None

args = sys.argv[1:]
cmd = args.pop(0) if args else ""

if cmd == "workspace" and args and args[0] == "list":
    if not os.path.exists(os.path.join(STATE, "workspaces.json")):
        seed()
    tabs = load("tabs.json", [])
    panes = load("panes.json", [])
    wss = []
    for w in load("workspaces.json", []):
        w = dict(w)
        w["tab_count"] = sum(1 for t in tabs if t["workspace_id"] == w["workspace_id"])
        w["pane_count"] = sum(1 for p in panes if p["workspace_id"] == w["workspace_id"])
        wss.append(w)
    out("cli:workspace:list", "workspace_list", "workspaces", wss)
elif cmd == "workspace" and args and args[0] == "create":
    # Mirrors the real CLI bootstrap used when herdr has no tabs/panes yet.
    cwd = opt(args, "--cwd") or "/tmp/stub"
    label = opt(args, "--label") or os.path.basename(cwd.rstrip("/")) or "workspace"
    wss = load("workspaces.json", [])
    num = max([int(w["workspace_id"][1:]) for w in wss], default=0) + 1
    ws_id = f"w{num}"
    tab_id = f"{ws_id}:t1"
    pane_id = f"{ws_id}:p1"
    ws = {"active_tab_id": tab_id, "agent_status": "unknown", "focused": True,
          "label": label, "number": len(wss) + 1, "pane_count": 1, "tab_count": 1,
          "workspace_id": ws_id}
    wss.append(ws)
    tabs = load("tabs.json", [])
    tabs.append({"agent_status": "unknown", "focused": True, "label": "1", "number": 1,
                 "pane_count": 1, "tab_id": tab_id, "workspace_id": ws_id})
    panes = load("panes.json", [])
    panes.append({"agent_status": "unknown", "cwd": cwd, "focused": True, "pane_id": pane_id,
                  "tab_id": tab_id, "workspace_id": ws_id})
    save("workspaces.json", wss)
    save("tabs.json", tabs)
    save("panes.json", panes)
    print(json.dumps({"id": "cli:workspace:create", "result": {"type": "workspace_created",
        "workspace": ws, "tab": {"tab_id": tab_id, "workspace_id": ws_id},
        "root_pane": {"pane_id": pane_id}}}))
elif cmd == "tab":
    sub = args.pop(0) if args else ""
    tabs = load("tabs.json", [])
    if not os.path.exists(os.path.join(STATE, "tabs.json")):
        seed()
        tabs = load("tabs.json", [])
    if sub == "list":
        out("cli:tab:list", "tab_list", "tabs", tabs)
    elif sub == "create":
        ws_arg = opt(args, "--workspace")
        if ws_arg is None:
            if not load("workspaces.json", []):
                # Like the real CLI with no tabs/panes at all.
                sys.stderr.write(json.dumps({"error": {"code": "workspace_not_found",
                                                       "message": "no active workspace"},
                                             "id": "cli:tab:create"}))
                sys.exit(1)
            ws = "w7"
        else:
            ws = ws_arg
        label = opt(args, "--label") or str(max([t["number"] for t in tabs if t["workspace_id"] == ws], default=0) + 1)
        num = max([t["number"] for t in tabs if t["workspace_id"] == ws], default=0) + 1
        panes = load("panes.json", [])
        seq = max(int(p["pane_id"].split(":")[1][1:]) for p in panes + [None] if p) + 1 if panes else 1
        pane_id = f"{ws}:p{seq}"
        tab_id = f"{ws}:t{num}"
        tabs.append({"agent_status": "unknown", "focused": False, "label": label, "number": num, "pane_count": 1, "tab_id": tab_id, "workspace_id": ws})
        panes.append({"agent_status": "unknown", "cwd": "/tmp/stub", "focused": False, "pane_id": pane_id, "tab_id": tab_id, "workspace_id": ws})
        save("tabs.json", tabs); save("panes.json", panes)
        print(json.dumps({"id": "cli:tab:create", "result": {"type": "tab_create", "tab": {"tab_id": tab_id}, "root_pane": {"pane_id": pane_id}}}))
    elif sub == "rename":
        tid, label = args[0], args[1]
        next(t for t in tabs if t["tab_id"] == tid)["label"] = label
        save("tabs.json", tabs)
    elif sub == "close":
        tid = args[0]
        tabs = [t for t in tabs if t["tab_id"] != tid]
        panes = [p for p in load("panes.json", []) if p["tab_id"] != tid]
        save("tabs.json", tabs); save("panes.json", panes)
        ws_tabs = [t for t in tabs if t["workspace_id"] == tid.split(":")[0]]
        if not ws_tabs:
            save("workspaces.json", [w for w in load("workspaces.json", []) if w["workspace_id"] != tid.split(":")[0]])
elif cmd == "pane":
    sub = args.pop(0) if args else ""
    if not os.path.exists(os.path.join(STATE, "panes.json")):
        seed()
    panes = load("panes.json", [])
    if sub == "list":
        out("cli:pane:list", "pane_list", "panes", panes)
    elif sub == "split":
        pid = opt(args, "--pane")
        direction = opt(args, "--direction") or "right"
        cwd = opt(args, "--cwd") or "/tmp/stub-split-" + direction
        src = next(p for p in panes if p["pane_id"] == pid)
        seq = max(int(p["pane_id"].split(":")[1][1:]) for p in panes) + 1
        panes.append({"agent_status": "unknown", "cwd": cwd, "focused": False,
                      "pane_id": f"{src['workspace_id']}:p{seq}", "tab_id": src["tab_id"],
                      "workspace_id": src["workspace_id"]})
        tabs = load("tabs.json", [])
        next(t for t in tabs if t["tab_id"] == src["tab_id"])["pane_count"] += 1
        save("panes.json", panes); save("tabs.json", tabs)
        print(json.dumps({"id": "cli:pane:split", "result": {"type": "pane_split"}}))
    elif sub == "rename":
        pid, label = args[0], args[1]
        next(p for p in panes if p["pane_id"] == pid)["title"] = label
        save("panes.json", panes)
    elif sub == "read":
        cnt = load("read-count.json", {"n": 0})
        cnt["n"] += 1
        save("read-count.json", cnt)
        lines = [f"stub pane output for {args[0]} (read {cnt['n']})"]
        lines += [f"log line {i} of {args[0]}" for i in range(1, 61)]
        print("\n".join(lines))
    elif sub == "close":
        panes = [p for p in panes if p["pane_id"] != args[0]]
        save("panes.json", panes)
elif cmd == "agent":
    sub = args.pop(0) if args else ""
    if sub == "list":
        agents = [{"agent": "opencode", "agent_status": "working", "cwd": "/home/stub/proj-dir",
                   "focused": True, "pane_id": "w7:p1", "revision": 6, "screen_detection_skipped": True,
                   "state_change_seq": 25, "tab_id": "w7:t1", "terminal_id": "term_1",
                   "terminal_title": "OC | stub agent", "terminal_title_stripped": "OC | stub agent",
                   "workspace_id": "w7"}]
        out("cli:agent:list", "agent_list", "agents", agents)
    elif sub == "read":
        cnt = load("agent-read-count.json", {"n": 0})
        cnt["n"] += 1
        save("agent-read-count.json", cnt)
        last = ""
        lp = os.path.join(STATE, "last-prompt")
        if os.path.exists(lp):
            last = open(lp).read().strip()
        print(f"stub output for {args[0]} (read {cnt['n']})")
        print(f"last prompt: {last}")
    elif sub == "prompt":
        with open(os.path.join(STATE, "last-prompt"), "w") as f:
            f.write(args[1])
        print("ok")
else:
    print(f"stub: unsupported command {cmd}", file=sys.stderr)
    sys.exit(1)
STUBEOF
chmod +x "$STUB/herdr"
export STUB_DIR="$STUB"
export PATH="$STUB:$PATH"

export MYBOX_CONFIG="$TMP/config.yaml"

if [ ! -x "$ROOT/mybox" ]; then
  make -C "$ROOT" build
fi

"$ROOT/mybox" project add "$PROJ"
printf '# Mybox\n\nThis project tracks tasks and knowledge.\n' >"$PROJ/README.md"
"$ROOT/mybox" task create --project proj --name "Ship the web UI"
"$ROOT/mybox" task create --project proj --name "Write E2E tests"
"$ROOT/mybox" task create --project proj --name "E2E status change target"
mkdir -p "$PROJ/tasks/e2e-status-change-target"
printf -- '---\nstatus: doing\n---\n\n# Drag me to done\n' \
  >"$PROJ/tasks/e2e-status-change-target/task.md"
mkdir -p "$PROJ/knowledge/notes"
printf '# Mybox\n\nWelcome to the workspace.\n\nSee [[notes/phase6]] and [[tasks]].\n' \
  >"$PROJ/knowledge/index.md"
printf '# Knowledge\n\nAll knowledge lives here.\n\nSee [docs](docs/).\n' \
  >"$PROJ/knowledge/README.md"
mkdir -p "$PROJ/knowledge/docs/recipes"
printf '# Guide\n\nDeep docs.\n' >"$PROJ/knowledge/docs/guide.md"
printf '# Pizza\n\nMargherita.\n' >"$PROJ/knowledge/docs/recipes/pizza.md"
printf '# Phase 6\n\n## Overview\n\nThe HTTP API is done.\n\n## Diagram\n\n```mermaid\ngraph LR\n  A[API] --> B[UI]\n```\n' >"$PROJ/knowledge/notes/phase6.md"
cat >"$PROJ/tasks.md" <<'EOF'
# Tasks

Task tracking lives here.

## Getting started

To get started open the task list and pick something small. Keep the board
updated as you move between lanes so the whole team sees progress at a glance.

## Non-goals

A few things we are deliberately not doing right now:

- No notifications yet.
- No offline mode.
- No mobile client.

## Release checklist

The steps below are repeated for every release so they are written down once.

### Create the tag

Tag the current commit and push it. Then start the release build.

### Run the smoke tests

Smoke tests run against a fresh checkout so nothing is left over.

### Publish the bundle

Upload the artifact and verify the checksums match the signed manifest.

## Operations

Everything in this section is about keeping the service healthy in production.

### Backups

Backups run nightly and are kept for thirty days. Restore is tested monthly.

### Alerts

On-call is paged on failed deploys and saturated queues.

## Outage post-mortems

Each incident gets a short write-up so the same mistake is not made twice.

### Incident summary

What happened, when it happened, and whom it affected.

### Timeline

A minute-by-minute reconstruction of the incident.

### Actions

The follow-up items that were agreed upon in the review.
EOF

exec "$ROOT/mybox" serve --project proj --port 19090 --no-browser
