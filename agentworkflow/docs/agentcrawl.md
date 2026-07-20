# agentcrawl

## Overview

イベントを検知、または指定されたイベントを読み込み、Taskを生成するツール。

- Task生成のみ担当する
- Workflow の実行は行わない
- 生成したTaskはすべて `task-manager*` に初期アサインされる

---

## CLI Reference

```
agentcrawl --dir <path> [--watch] [--pretty]
```

| フラグ | 必須 | デフォルト | 説明 |
|--------|------|------------|------|
| `--dir` | Yes | - | イベントソースディレクトリ |
| `--watch` | No | false | daemon モード (fsnotify による常時監視) |
| `--pretty` | No | false | ログを人間が読める形式で出力 |

### 環境変数

| 変数 | 説明 |
|------|------|
| `WORKSPACE_ROOT` | ワークスペースルートパス (未設定時は `--dir` の2階層上を推定) |

---

## Modes

### oneshot モード

```bash
agentcrawl --dir $WORKSPACE_ROOT/events/incoming --pretty
```

現在のディレクトリ内のイベントを処理して即座に終了する。

### daemon モード

```bash
agentcrawl --watch --dir $WORKSPACE_ROOT/events/incoming --pretty
```

fsnotify を利用してイベントファイルのドロップを常時監視する。
新しいファイルが作成されると自動的にTaskを生成する。
500ms のデバウンスにより、短時間に複数ファイルがドロップされた場合はまとめて処理する。

---

## Task ID Generation

### 形式

```
YYYYMMDD-HHMMSS-<source>-<short-hash>
```

| 項目 | 説明 | 例 |
|------|------|-----|
| YYYYMMDD | 日付 | `20260720` |
| HHMMSS | 時刻 | `172209` |
| source | イベントソース識別子 | `file` |
| short-hash | eventData の SHA256 先頭3バイト (16進数) | `9cbba2` |

### source 一覧 (Phase 1)

| source | 説明 |
|--------|------|
| `file` | ファイルシステム上のイベント (Phase 1 のみ) |
| `github` | GitHub Issue/PR (Phase 2+) |
| `jira` | Jira Ticket (Phase 2+) |

### 例

```
20260720-172209-file-9cbba2
```

---

## Event Processing

### 読み込み対象

`--dir` 配下の以下のファイルを対象とする。

- 非隠蔵ファイル (先頭が `.` でない)
- ディレクトリは無視

### 二重処理防止

Taskディレクトリ (`event/`) を生成後、読み込んだ元のEventファイルを同ディレクトリ内へ移動 (mv) する。

```
events/incoming/disk_alert.json    ← 元ファイル
        ↓ (mv)
tasks/<task-id>/event/disk_alert.json  ← 移動後
```

### タイトル生成ルール

Event ファイル名から拡張子を除いたものを使用する。

```
disk_alert.json → "Process disk_alert"
cpu_alert.json  → "Process cpu_alert"
proposal.md     → "Process proposal"
```

---

## Output Structure

### Task ディレクトリ構造

```
tasks/<task-id>/
├── metadata.yaml    # Runtime 専用 (Agent 更新禁止)
├── task.md          # LLM への指示
├── event/           # イベント原本 (変更禁止)
│   └── disk_alert.json
├── artifacts/       # 成果物 (空ディレクトリで作成)
└── history/         # 履歴 (空ディレクトリで作成)
```

### metadata.yaml 初期値

```yaml
id: 20260720-172209-file-9cbba2
title: Process disk_alert
status: inbox
current_assignee: task-manager*
priority: normal
retry_count: 0
max_retry: 3
created_at: "2026-07-20T08:22:09Z"
updated_at: "2026-07-20T08:22:09Z"
source: file
```

### task.md テンプレート

```markdown
# Task: Process disk_alert

Event fileを解析してください。

結果を artifacts/report.md へ保存してください。

必要であれば handoff.yaml を書いてください。（純粋なYAML形式のみ出力し、Markdown装飾を含めないこと）
```

---

## Configuration

### ディレクトリ自動作成

初回実行時に以下のディレクトリを自動作成する (agentcrawl では作成しない、agentrun が担当)。

- `tasks/`
- `archive/`
- `events/`

### エージェントディレクトリ

agentcrawl は `agents/` ディレクトリにはアクセスしない。Task生成のみを担当する。
