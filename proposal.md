# AI Agent Workflow System
## Implementation Specification

Version: 1.0

---

# 1. Overview

本プロジェクトは、LLM(OpenCode等)を利用したマルチエージェントワークフローシステムを実装する。

本システムでは、Workflow EngineやDatabaseを使用しない。

代わりに、

- File System
- Markdown
- YAML
- Shell Command

のみを利用してWorkflowを構築する。

Agentは単なるCLIとして動作し、
Taskはディレクトリとして管理される。

Workflowの状態はすべてファイルシステムに保存される。

---

# 2. Goals

本システムの目的は以下である。

- Agentを容易に追加できる
- Agent同士を疎結合にする
- 人間をWorkflowへ自然に参加させる
- Databaseを不要にする
- Gitで履歴管理できる
- すべての成果物をMarkdownで残す
- OpenCode以外のAgentへ容易に置き換えられる

---

# 3. Design Principles

以下を必ず守ること。

## 3.1 AgentはCLI

AgentはHTTP Serverではない。

Agentは以下のようなCLIで実行される。

```bash
agentrun --watch agents/operator
```

AgentはTaskを入力として受け取り、
成果物を生成する。

---

## 3.2 RuntimeがWorkflowを管理する

AgentはWorkflowを管理しない。

Workflow管理はRuntimeのみが行う。

Agentは禁止事項。

- metadata.yamlを書き換える
- Taskを移動する
- Queueを管理する
- Lockを管理する

---

## 3.3 File Systemが唯一のSource of Truth

Databaseは禁止。

Workflowの状態はすべてファイルシステムに保存する。

---

## 3.4 Taskは不変

Task Directoryは移動しない。

担当者だけ変更する。

---

## 3.5 人間もAgent

Supervisorは特別扱いしない。

Human Agentとして扱う。

---

# 4. Architecture

```
+----------------+
| Event Crawlers |
+-------+--------+
        |
        v
+----------------+
| Runtime        |
+-------+--------+
        |
        v
tasks/<task-id>/
        |
        +---- metadata.yaml
        +---- task.md
        +---- event/
        +---- artifacts/
        +---- history/
        +---- handoff.md
        +---- escalation.md
```

RuntimeはTaskを監視し、
担当Agentへ実行を依頼する。

---

# 5. Directory Layout

```
workspace/

├── agents/
│   ├── operator/
│   │   ├── AGENTS.md
│   │   ├── knowledge/
│   │   └── skills/
│   │
│   ├── developer/
│   ├── reviewer/
│   └── supervisor/
│
├── tasks/
│
├── crawlers/
│
└── runtime/
```

---

# 6. Task Directory

Taskは以下の構成を持つ。

```
tasks/

└── 20260718_103000/

    metadata.yaml

    task.md

    event/

    artifacts/

    history/
```

Task IDは一意であること。

Task Directoryは削除・移動しない。

---

# 7. metadata.yaml

Runtimeが管理する。

```yaml
id: 20260718_103000

title: Investigate Alert

status: inbox

current_assignee: operator1

priority: normal

retry_count: 0

created_at: 2026-07-18T10:30:00Z

updated_at: 2026-07-18T10:30:00Z

source: alert
```

Agentはこのファイルを書き換えてはならない。

---

# 8. task.md

Agentへの指示を書く。

Markdownで管理する。

例

```markdown
# Task

event.jsonを解析してください。

調査結果を

artifacts/report.md

へ保存してください。

必要であればhandoff.mdを書いてください。
```

---

# 9. event/

イベントの原本を保存する。

例

```
event/

alert.json

proposal.md

jira.json

github_issue.json
```

変更禁止。

---

# 10. artifacts/

Agentが生成する成果物。

例

```
report.md

implementation_plan.md

review.md

patch.diff

benchmark.md
```

自由に追加できる。

---

# 11. history/

Runtimeのみが更新する。

例

```
0001-created.md

0002-assigned.md

0003-started.md

0004-finished.md

0005-handoff.md
```

Taskのすべての状態遷移を記録する。

---

# 12. Runtime

RuntimeはWorkflow Engineである。

責務

- Task監視
- Lock取得
- metadata更新
- OpenCode起動
- handoff処理
- escalation処理
- history更新

---

## Runtime Flow

```
Task検知

↓

Lock取得

↓

status=inprogress

↓

OpenCode実行

↓

成果物保存

↓

handoff確認

↓

escalation確認

↓

history追加

↓

status=done

↓

Unlock
```

---

# 13. Agent

Agentは以下のみ実施する。

- task.mdを読む
- knowledgeを読む
- skillsを読む
- eventを読む
- artifactsを書く
- handoff.mdを書く
- escalation.mdを書く

Agentは禁止事項

- metadata.yaml更新
- history更新
- Task移動
- Lock取得

---

# 14. Handoff

AgentはTaskを移動してはいけない。

代わりに

```
handoff.md
```

を書く。

例

```markdown
next_assignee: developer1

reason:

実装が必要
```

Runtimeが担当者を変更する。

---

# 15. Escalation

Agentは

```
escalation.md
```

を書く。

例

```markdown
target: supervisor

reason:

判断が必要
```

RuntimeがRoutingを決定する。

---

# 16. Event Crawlers

CrawlerはTask生成のみ担当する。

例

```
Alert

↓

Task生成

↓

metadata.yaml

↓

task.md
```

CrawlerはTaskを実行しない。

---

# 17. Agent Execution

RuntimeはAgentを以下のように起動する。

例

```bash
opencode \
    run \
    --dir agents/operator \
    --file tasks/<task-id>/task.md \
    --thinking \
    --format json
```

RuntimeはOpenCodeの実装に依存しないよう抽象化すること。

将来的に

- Claude Code
- Codex
- Gemini CLI

へ置き換え可能にする。

---

# 18. Lock

同一Taskを複数Runtimeが実行しないこと。

最低限、

```
.lock
```

ファイルによる排他制御を実装する。

---

# 19. Retry

Agent失敗時は

retry_count

を増加させる。

最大回数を超えたら

SupervisorへEscalationする。

---

# 20. Logging

Runtimeは以下を記録する。

- Agent起動
- Agent終了
- Exit Code
- 実行時間
- Retry回数
- Error

---

# 21. Extensibility

以下を容易に追加できる設計とする。

- 新しいAgent
- 新しいCrawler
- 新しいイベント
- 新しいArtifact
- 新しいLLM
- 新しいRouting Rule

既存コードの修正を最小限にすること。

---

# 22. Non Functional Requirements

- Database不要
- Queue不要
- API不要
- Linuxで動作
- Git管理可能
- Markdown中心
- YAML中心
- Shell Scriptから利用可能
- OpenCode依存を抽象化

---

# 23. Future Scope

将来的には以下を追加予定。

- Dashboard
- Web UI
- Metrics
- Priority Queue
- Parallel Execution
- Multi-Agent Collaboration
- SLA Monitoring
- Notification
- Approval Workflow
- Workspace Graph Specification (WGS) Integration
- Knowledge Repository
- Skill Repository
- MCP Integration

---

# 24. Acceptance Criteria

以下を満たせば実装完了とする。

- EventからTaskが生成される
- RuntimeがTaskを検知する
- AgentがTaskを処理できる
- Artifactsが生成される
- Handoffが動作する
- Escalationが動作する
- Historyが記録される
- Lockが動作する
- Retryが動作する
- SupervisorがWorkflowへ参加できる
- Databaseを使用しない
- Workflow状態がすべてファイルシステムに保存される
- OpenCode以外へ置き換え可能な構造になっている

---

# 25. Philosophy

本システムは、LLMそのものを賢くすることを目的としない。

目的は、LLMを協調動作させるための**シンプルで堅牢なワークフロー基盤**を提供することである。

設計思想は以下の一文に集約される。

> **"Workflow lives in the File System. Agents only think."**
