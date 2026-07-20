# AI Agent Guidelines - Agent Workflow System

## 概要

本ドキュメントは、AI Agent が agentcrawl / agentrun を使用する際の操作ガイドとルールを定義する。

---

## agentcrawl 操作

### oneshot 実行

```bash
agentcrawl --dir $WORKSPACE_ROOT/events/incoming --pretty
```

### 結果確認

```bash
# 生成されたタスク一覧
ls $WORKSPACE_ROOT/tasks/

# metadata 確認
cat $WORKSPACE_ROOT/tasks/<task-id>/metadata.yaml
```

---

## agentrun 操作

### oneshot 実行

```bash
agentrun --dir $WORKSPACE_ROOT --pretty
```

### daemon 実行

```bash
agentrun --watch --dir $WORKSPACE_ROOT --pretty
```

### タスク状態確認

```bash
# 全タスク
find $WORKSPACE_ROOT/tasks -name metadata.yaml

# inbox タスクのみ
grep -l "status: inbox" $WORKSPACE_ROOT/tasks/*/metadata.yaml

# 特定タスクの履歴
ls $WORKSPACE_ROOT/tasks/<task-id>/history/
```

---

## Agent 禁止事項

以下の操作は Agent が行ってはならない。

- **metadata.yaml を直接更新しない** — Runtime のみが更新する
- **history/ を直接更新しない** — Runtime のみが記録する
- **.lock/ を操作しない** — Runtime のみが管理する
- **Task ディレクトリを削除しない** — Task は永続オブジェクト

---

## handoff.yaml 出力ルール

handoff.yaml を生成する際は以下のルールに従う。

### 必須事項

- `next_assignee` は必須
- **純粋なYAML形式のみ出力する**
- **Markdown コードブロック (` ```yaml ... ``` ) を含めない**

### 出力形式

```yaml
next_assignee: code-developer*
reason: ソースコードの実装が必要であると判断したため
```

### next_assignee のパターン

| パターン | 例 | 意味 |
|----------|-----|------|
| `クラス名*` | `code-developer*` | 空きインスタンスすべてから選択 |
| `インスタンス名` | `system-operator1` | 特定のインスタンスを指名 |
| `human` | `human` | 人間に判断を仰ぐ |

### 例

```yaml
# 役割を指定して委譲
next_assignee: code-developer*
reason: ソースコードの実装が必要であると判断したため

# 特定のインスタンスを指名
next_assignee: system-operator1
reason: 以前の対応の続きであるため

# 人間に判断を仰ぐ
next_assignee: human
reason: 承認が必要なため
```

---

## 成果物ルール

### artifacts/

- 成果物は `artifacts/` 配下に配置する
- ファイル名に制限はない
- **空ファイルを作成しない** (agentrun がバリデーションで検出する)

### 中間状態ファイル

- Agent が現在どの状態にあるかを記憶する必要がある場合は
- `artifacts/.manager_state.json` のような独自のステートファイルを `artifacts/` 配下に生成する
- このステートファイルは **Agent専用** であり、Runtime が参照・解釈しない

---

## 注意事項

### Lock との関係

Agent が長時間実行される場合、Lock の TTL (40分) に注意する。
Agent がハングアップした場合、Runtime が Stale Lock と判定してタスクをリカバリする可能性がある。

### タイムアウト

Agent サブプロセスのデフォルトタイムアウトは10分。
長時間のタスクは、handoff で別の Agent に委譲し、分割して実行する。

### Retry

Agent がエラー終了した場合、Runtime が自動でリトライする (最大3回)。
エラーの原因を取り除いてから終了すること。
