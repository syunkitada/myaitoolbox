# agentrun

## Overview

Taskディレクトリ全体を監視、または一括処理し、Agentをサブプロセスとして起動・管理する中央ランタイム (Orchestrator)。

デーモンモードでは fsnotify のイベント監視を利用し、1プロセスがすべてのTaskのライフサイクルを管理する。

---

## CLI Reference

```
agentrun --dir <path> [--watch] [--pretty] [--timeout <duration>] [--agents-dir <path>]
```

| フラグ | 必須 | デフォルト | 説明 |
|--------|------|------------|------|
| `--dir` | Yes | - | ワークスペースルートディレクトリ |
| `--watch` | No | false | daemon モード (fsnotify による常時監視) |
| `--pretty` | No | false | ログを人間が読める形式で出力 |
| `--timeout` | No | 10m | Agent サブプロセスのタイムアウト |
| `--agents-dir` | No | agents | agents ディレクトリのパス (workspace 相対) |

---

## Modes

### oneshot モード

```bash
agentrun --dir $WORKSPACE_ROOT --pretty
```

1. crashed task のリカバリ
2. inbox タスクの一括処理
3. retryable タスク (inprogress, ロックなし) のリトライ
4. 終了

### daemon モード

```bash
agentrun --watch --dir $WORKSPACE_ROOT --pretty
```

1. 起動時に oneshot 相当の処理を実行
2. fsnotify で `tasks/` ディレクトリを監視
3. 新規ファイル作成/変更時にタスクを処理
4. 5分ごとに定期リカバリチェック
5. SIGINT/SIGTERM でグレースフルシャットダウン

---

## Task Processing Flow

```
Detect inbox / retryable tasks
  ↓
For each task:
  1. Lock 取得 (mkdir .lock)
  2. metadata.yaml → status: inprogress
  3. history: XXXX-started.md 記録
  4. Agent サブプロセス起動 (timeout 付き)
  5. Exit code 確認
     ├─ 成功 (exit 0):
     │   ├─ artifacts 検証
     │   ├─ handoff.yaml あり → history 記録 → metadata 更新 (inbox, next_assignee) → unlock
     │   └─ handoff.yaml なし → status: done → archive 移動 → unlock
     └─ 失敗 (exit != 0):
         ├─ retry_count++ 
         ├─ max_retry 超過 → assignee: human → unlock
         └─ max_retry 未達 → retry_count 更新 → unlock (次回リトライ)
```

---

## Lock

### 方式

`mkdir .lock` によるアトミックロック。

### owner.yaml

```yaml
daemon_pid: 12345
worker_pid: 12346
hostname: worker-node-01
acquired_at: 2026-07-18T22:00:00Z
```

### Stale Lock 判定

| 条件 | 判定 |
|------|------|
| PID が存在しない | Stale |
| TTL (40分) 超過 | Stale |
| owner.yaml が存在しない / パース不可 | Stale |

### ForceRelease

Stale Lock は強制解除し、`worker_pid` のプロセスが生存していれば `SIGKILL` で終了する。

---

## Retry

| 項目 | 値 |
|------|-----|
| 最大 Retry 回数 | 3 (デフォルト) |
| タイムアウト | 10分 (デフォルト) |
| retry_count 初期値 | 0 |

### リトライフロー

1. Agent エラー終了
2. `retry_count++`
3. `retry_count > max_retry` の場合:
   - `current_assignee: human` に変更
   - Lock 解放
   - 人間の介入を待つ
4. `retry_count <= max_retry` の場合:
   - metadata 更新 (retry_count のみ)
   - Lock 解放
   - 次回の oneshot/daemon で自動リトライ

### 人間介入後の再開

```yaml
# metadata.yaml を手動編集
current_assignee: system-operator   # human → 対象Agent
retry_count: 0                       # リセット
```

---

## Handoff

### 処理フロー

1. `handoff.yaml` の読み取り
2. History への記録 (`history/XXXX-handoff.md`)
3. `metadata.yaml` の更新:
   - `current_assignee: <next_assignee>`
   - `status: inbox`
   - `retry_count: 0`

### Tolerant YAML Parse

LLMがMarkdownコードブロックで囲んで出力しても対応可能。

```
入力:  ```yaml\nnext_assignee: code-developer*\nreason: ...\n```
出力:  { next_assignee: "code-developer*", reason: "..." }
```

正規表現 `` ```yaml\n([\s\S]*?)\n``` `` でYAMLブロックを抽出する。

---

## Recovery

### Crashed Task 検出

以下の条件に該当するタスクをリカバリ対象とする。

1. `status: inprogress`
2. `current_assignee` が Agent (human ではない)
3. `.lock/` が存在しない または Stale Lock

### リカバリ処理

1. orphan プロセスの強制終了 (該当する場合)
2. Stale Lock の強制解除
3. `status: inbox` にリセット
4. History に `recovered` エントリを記録

### リカバリ対象外

- `current_assignee: human` のタスク (人間が作業中とみなす)

---

## Agent Execution

### OpenCode Adapter

Phase 1 では OpenCode を利用する。

```bash
opencode run \
    --dir agents/<agent-class> \
    --file tasks/<task-id>/task.md \
    --thinking \
    --format json
```

### agent-class の抽出

`current_assignee` から末尾の `*` および数字を除去する。

```
task-manager*   → task-manager
system-operator1 → system-operator
code-developer* → code-developer
```

### LLM 切り替え

将来のLLM切り替え時は、`internal/infrastructure/opencode_adapter.go` を差し替える。
`domain.AgentSpawner` インターフェースを実装する新しいアダプタを作成するだけで、変更がアダプタ層に局所化される。

---

## Artifact Validation

Agent 実行後に artifacts/ を検証する。

| 検証項目 | 条件 |
|----------|------|
| ディレクトリ存在 | `artifacts/` が存在すること |
| ファイル存在 | 少なくとも1つのファイルが存在すること |
| ファイルサイズ | すべてのファイルが > 0 バイトであること |

検証失敗時は warning ログを出力し、タスク処理は継続する。

---

## History

### ファイル名形式

```
XXXX-<type>.md
```

`XXXX` は4桁の連番 (0001, 0002, ...)。

### type 一覧

| type | 説明 |
|------|------|
| `started` | Agent 実行開始 |
| `finished` | タスク完了 |
| `handoff` | 次の担当者へ委譲 |
| `error` | エラー発生 (リトライ情報含む) |
| `recovered` | crashed task のリカバリ |

### 例

```
history/
├── 0001-started.md
├── 0002-error.md
├── 0003-started.md
├── 0004-error.md
├── 0005-started.md
├── 0006-handoff.md
└── 0007-finished.md
```
