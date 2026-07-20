# Development Guide

## Overview

Phase 1 の実装完了に伴い、システムの拡張方法を説明する。

---

## Adding a New LLM Adapter

将来的に OpenCode 以外のLLM (Claude Code, Codex CLI, Gemini CLI) に切り替える場合の手順。

### Step 1: アダプタの作成

`internal/infrastructure/` に新しいアダプタファイルを作成する。

```go
// claude_adapter.go
package infrastructure

import "github.com/syunkitada/myaitoolbox/agentrun/internal/domain"

type ClaudeAdapter struct {
	workspaceRoot string
}

func NewClaudeAdapter(workspaceRoot string) *ClaudeAdapter {
	return &ClaudeAdapter{workspaceRoot: workspaceRoot}
}

func (a *ClaudeAdapter) BuildCommand(taskInfo *domain.TaskInfo) (string, []string) {
	agentClass := extractAgentClass(taskInfo.Metadata.CurrentAssignee)
	return "claude", []string{
		"run",
		"--dir", filepath.Join("agents", agentClass),
		"--file", filepath.Join("tasks", taskInfo.TaskID, "task.md"),
	}
}
```

### Step 2: ProcessSpawner の修正

`internal/infrastructure/process_spawner.go` の `Spawn` メソッドで、使用するアダプタを切り替える。

```go
// 現在: OpenCode を使用
adapter := NewOpenCodeAdapter(s.workspaceRoot)

// 切り替え: Claude を使用
adapter := NewClaudeAdapter(s.workspaceRoot)
```

### Step 3: CLI フラグの追加 (オプション)

`--llm` フラグでアダプタを選択可能にする。

```go
flagLLM string
rootCmd.Flags().StringVar(&flagLLM, "llm", "opencode", "LLM provider (opencode, claude, codex)")
```

---

## Port Interfaces

`internal/domain/port.go` で定義されているインターフェース一覧。

| Port | メソッド | 用途 |
|------|----------|------|
| **TaskStore** | `ListTaskDirs()`, `ListTaskDirsByStatus()`, `ReadMetadata()`, `WriteMetadata()`, `MoveToArchive()`, `EnsureWorkspace()` | Task の CRUD |
| **LockManager** | `Acquire()`, `Release()`, `IsLocked()`, `GetLockInfo()`, `IsStale()`, `ForceRelease()`, `CleanupOrphan()` | アトミックロック |
| **HistoryRecorder** | `Record()`, `NextSequence()` | 履歴記録 |
| **AgentSpawner** | `Spawn()` | Agent サブプロセス起動 |
| **Watcher** | `Watch()`, `Close()` | ファイルシステム監視 |

新しいインフラストラクチャを追加する場合は、まず domain にPort を定義し、次に infrastructure に実装を置く。

---

## Adding a New Agent

### Step 1: ディレクトリ作成

```bash
mkdir -p $WORKSPACE_ROOT/agents/my-new-agent
```

### Step 2: AGENTS.md 定義

```markdown
# my-new-agent

## Role
<役割の説明>

## Supported Task Types
- <対応可能なTaskの種類>

## Rules
- <ルール>
```

### Step 3: knowledge/ と skills/ の配置 (オプション)

```bash
mkdir -p $WORKSPACE_ROOT/agents/my-new-agent/{knowledge,skills}
# ドキュメントやスクリプトを配置
```

### Step 4: Task をアサイン

`handoff.yaml` で `next_assignee: my-new-agent*` を指定する。

---

## Project Structure

```
agentcrawl/                        agentrun/
├── cmd/agentcrawl/                ├── cmd/agentrun/
│   └── main.go                    │   └── main.go
└── internal/                      └── internal/
    ├── domain/                        ├── domain/
    │   ├── event.go                    │   ├── task.go
    │   ├── task.go                     │   ├── metadata.go
    │   └── util.go                     │   ├── handoff.go
    ├── application/                   │   ├── lock.go
    │   └── crawler.go                  │   └── port.go
    └── infrastructure/                ├── application/
        ├── fs_event_reader.go          │   ├── runner.go
        └── fs_task_writer.go          │   ├── detector.go
                                        │   └── handoff_processor.go
                                        └── infrastructure/
                                            ├── fs_task_store.go
                                            ├── fs_lock.go
                                            ├── fs_history.go
                                            ├── fsnotify_watcher.go
                                            ├── process_spawner.go
                                            └── opencode_adapter.go
```

---

## Testing

### ユニットテスト

```bash
# agentcrawl
cd agentcrawl && go test ./...

# agentrun
cd agentrun && go test ./...
```

### 結合テスト

```bash
# テストワークスペース作成
export WORKSPACE_ROOT=/tmp/test-workspace
mkdir -p $WORKSPACE_ROOT/events/incoming

# テストイベント配置
echo '{"alert": "test"}' > $WORKSPACE_ROOT/events/incoming/test.json

# agentcrawl で Task 生成
cd agentcrawl && go run ./cmd/agentcrawl --dir $WORKSPACE_ROOT/events/incoming --pretty

# agentrun で Task 処理
cd agentrun && go run ./cmd/agentrun --dir $WORKSPACE_ROOT --pretty

# 結果確認
cat $WORKSPACE_ROOT/tasks/*/metadata.yaml
ls $WORKSPACE_ROOT/tasks/*/history/
```

### vet チェック

```bash
cd agentcrawl && go vet ./...
cd agentrun && go vet ./...
```

---

## Technology Stack

| Category | Library | Reason |
|----------|---------|--------|
| CLI | `github.com/spf13/cobra` | 標準的な Go CLI フレームワーク |
| Logging | `log/slog` | 標準ライブラリ、構造化ログ |
| Configuration | `gopkg.in/yaml.v3` | YAML パース |
| File Watch | `github.com/fsnotify/fsnotify` | inotify / fsevents 対応 |
| Formatting | `gofmt` + `goimports` | 標準 Go スタイル |
| Linting | `github.com/golangci/golangci-lint` | マルチリンターアグリゲータ |

---

## Phase 2+ 計画

| Phase | 内容 |
|-------|------|
| Phase 2 | waiting 状態、イベントソース抽象化 (GitHub/Jira) |
| Phase 3 | 複数 Agent 連携、Scheduler、Dashboard、Metrics |
| Phase 4 | Plugin 化、複数 LLM 対応、Remote Workspace |
