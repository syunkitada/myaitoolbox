# mybox

Markdownを唯一のデータソースとする個人ワークスペースツールです。
タスク（GTD）とナレッジをファイルとして管理し、CLIとWeb UIの両方から操作できます。データベースは不要です。

## 機能

- **タスク管理** — `tasks/` 配下のMarkdownファイル（1タスク＝1ディレクトリ）で管理
  - CLI: 作成・一覧・検索・表示・編集・フィールド更新・アーカイブ
  - Web UI: GTDボード（Todo / Doing / Blocked / Review / Done）とドラッグ＆ドロップ
- **ナレッジ管理** — `knowledge/` 配下のMarkdownファイル
  - CLI: 作成・一覧・検索・表示・編集・移動・リネーム
  - Web UI: エクスプローラー・WikiLink・Backlinks・Graph View・Mermaid・アウトライン・全文検索・タグ・お気に入り・最近開いたファイル
- **横断検索** — タスクとナレッジをまとめて検索
- **HTTP API + Web UI** — `serve` コマンドで起動

## インストール

**前提:** Go 1.25+、Node.js（Web UI のビルドに必要）

```bash
git clone https://github.com/syunkitada/myaitoolbox
cd myaitoolbox/agenttools/mybox

make web-build          # Web UI をビルドして internal/webui/dist へコピー
go install ./cmd/mybox  # $GOPATH/bin/mybox にインストール
```

> [!NOTE]
> `make web-build` を先に実行しないと、Web UI が空のバイナリがインストールされます。
> `go install github.com/syunkitada/myaitoolbox/mybox/cmd/mybox@latest` のようなリモートからの直接インストールは Web UI が含まれないため非推奨です。

### zsh 補完

```zsh
# ~/.zshrc に追加
source <(mybox completion zsh)
```

または補完ファイルを生成して読み込む場合:

```zsh
mybox completion zsh > "${fpath[1]}/_mybox"
compinit
```

## 使い方

### プロジェクト登録

```bash
mybox project add ~/workspace/myproject
mybox project list
```

### タスク

```bash
mybox task create --project proj --name "Design the login flow"
mybox task list --project proj
mybox task list --project proj --status doing --tag web
mybox task show --project proj 20260802_1200_design-the-login-flow
mybox task set --project proj <task-id> --status review --priority high
mybox task edit --project proj <task-id>   # $EDITOR で編集
mybox task archive --project proj <task-id>
```

### ナレッジ

```bash
mybox knowledge create --project proj notes/architecture
mybox knowledge list --project proj
mybox knowledge show --project proj notes/architecture
mybox knowledge edit --project proj notes/architecture
mybox knowledge move --project proj notes/architecture docs/architecture
mybox knowledge rename --project proj docs/architecture design
```

### 検索

```bash
mybox search oauth --project proj
mybox search oauth --type knowledge
mybox search login --type task --json
```

### Web UI

```bash
mybox serve --project proj
# http://127.0.0.1:8080 （ブラウザ自動起動）
```

オプション: `--host` `--port` `--no-browser` `--read-only`

## データ構造

```
myproject/
├── tasks/
│   └── 20260802_1200_design-the-login-flow/
│       └── task.md          # --- フロントマターで status / priority 等を保持
└── knowledge/
    ├── index.md
    └── notes/architecture.md
```

タスクはフロントマター、ナレッジは `[[WikiLink]]` 記法で相互にリンクできます。

## 開発

```bash
make web-build # Web UI をビルドして internal/webui/dist へコピー
make build     # ./mybox バイナリを生成（go install の代わりにローカル確認用）
make test      # Go のユニットテスト
make lint      # golangci-lint
make e2e       # CLI E2E + Playwright E2E
```

## テスト

- Go ユニットテスト: リポジトリ・ユースケース・HTTP API（httptest）
- Playwright E2E: Dashboard / タスク / ナレッジ（編集・Mermaid・アウトライン）/ 検索 / Graph / ボード DnD
- CLI E2E: `tests/cli_e2e.sh`（一時プロジェクトで一連のコマンドを検証）
