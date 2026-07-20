# AI Agent Workflow System

## Overview

本プロジェクトは **myaitoolbox** に、AIエージェントによるワークフロー実行基盤を追加する。

> **Workflow lives in the File System. Agents only think.**

- Agentは単なるCLIである
- Workflowはファイルシステムで表現する
- Taskはディレクトリとして管理する
- Database, Queue, API を使用しない
- MarkdownとYAMLを唯一の管理フォーマットとする
- Gitによる履歴管理を前提とする

---

## Components

```
┌──────────────┐     ┌──────────┐     ┌──────────────────────────┐
│ External Event│────▶│agentcrawl│────▶│  $WORKSPACE_ROOT/tasks/  │
└──────────────┘     └──────────┘     └──────────────────────────┘
                                              │
                                     (inotify watch)
                                              │
                                              ▼
                                     ┌─────────────────┐
                                     │     agentrun     │
                                     └─────────────────┘
                                        │         │
                                        │         ▼
                                        │   [Agent subprocess]
                                        │         │
                                        ▼         ▼
                                   artifacts/  handoff.yaml
```

| Component | Responsibility |
|-----------|----------------|
| **agentcrawl** | 外部イベントをTaskへ変換し `task-manager` へアサインする |
| **agentrun** | Workflowを実行・管理する |
| **Agent** | Taskを理解し成果物を生成・委譲する |
| **Human** | 判断・承認・例外対応を行う |
| **File System** | Workflowの唯一のSource of Truth |

---

## Directory Structure

```
$WORKSPACE_ROOT/

├── agents/
│   ├── task-manager/       <- Taskのトリアージ・ルーティング担当
│   ├── system-operator/    <- システム運用タスク担当
│   ├── code-developer/     <- コード開発担当
│   ├── code-review-manager/<- レビュー管理担当
│   ├── code-reviewer/      <- レビュー実行担当
│   └── supervisor/         <- 監督担当
│
├── tasks/                  <- アクティブなTask
│   └── <task-id>/
│       ├── metadata.yaml
│       ├── task.md
│       ├── event/
│       ├── artifacts/
│       └── history/
│
├── archive/                <- 完了したTaskの退避先
│
├── events/
│   └── incoming/           <- イベントファイルの配置場所
│
└── runtime/
```

---

## Quick Start

### 環境変数

```bash
export WORKSPACE_ROOT=/path/to/workspace
```

### イベントを投入してTaskを生成

```bash
echo '{"alert": "disk full"}' > $WORKSPACE_ROOT/events/incoming/disk_alert.json

agentcrawl --dir $WORKSPACE_ROOT/events/incoming --pretty
```

### Taskを処理

```bash
agentrun --dir $WORKSPACE_ROOT --pretty
```

### 結果を確認

```bash
# タスク一覧
ls $WORKSPACE_ROOT/tasks/

# metadata確認
cat $WORKSPACE_ROOT/tasks/<task-id>/metadata.yaml

# 履歴確認
ls $WORKSPACE_ROOT/tasks/<task-id>/history/
```

---

## Dependencies

- Go 1.25.7
- `github.com/spf13/cobra` — CLI フレームワーク
- `gopkg.in/yaml.v3` — YAML パース
- `github.com/fsnotify/fsnotify` — inotify監視
- `log/slog` — 構造化ログ

---

## Development Phases

| Phase | 内容 | Status |
|-------|------|--------|
| Phase 1 | Core workflow (agentcrawl + agentrun, OpenCode, oneshot/daemon) | **実装完了** |
| Phase 2 | waiting状態、イベントソース抽象化 (GitHub/Jira) | 未着手 |
| Phase 3 | 複数Agent連携、Scheduler、Dashboard、Metrics | 未着手 |
| Phase 4 | Plugin化、複数LLM対応、Remote Workspace | 未着手 |
