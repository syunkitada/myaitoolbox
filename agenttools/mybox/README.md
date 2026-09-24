# mybox

Markdown を唯一のデータソースとする個人ワークスペースツールです。
タスク（GTD）とプロジェクト内のファイルをプロジェクト単位で管理し、CLI と Web UI の両方から操作できます。データベースは不要で、すべてがファイルとして保存されます。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | AIエージェント向けのリポジトリ作業ガイドライン。 |
| [`cmd/README.md`](./cmd/README.md) | CLI のエントリポイント。 |
| [`internal/README.md`](./internal/README.md) | アプリケーション本体の各層。 |
| [`templates/README.md`](./templates/README.md) | バイナリに埋め込む組み込みテンプレート。 |
| [`tests/README.md`](./tests/README.md) | CLI E2E テスト。 |
| [`web/README.md`](./web/README.md) | Web UI のソース、ビルド、テスト。 |
| [`node_modules/README.md`](./node_modules/README.md) | Git 管理下に残っている開発ツールの実行時生成物。編集対象外。 |
| [`test-results/README.md`](./test-results/README.md) | Git 管理下に残っているテスト実行結果。編集対象外。 |
| [`.gitignore`](./.gitignore) | ローカル生成物・ビルド成果物の除外設定。 |
| [`.golangci.yml`](./.golangci.yml) | Go lint の設定。 |
| [`Makefile`](./Makefile) | ビルド、テスト、lint、Web UI 開発用コマンド。 |
| [`go.mod`](./go.mod) | Go モジュール名と直接依存関係。 |
| [`go.sum`](./go.sum) | Go 依存関係のチェックサム。 |
| [`openapi.yaml`](./openapi.yaml) | HTTP API の仕様。生成コードの入力。 |
| [`README.md`](./README.md) | リポジトリ全体の使い方と構成。本文書。 |

`node_modules/`、`test-results/`、および `web/test-results/` は現状Git管理下にある実行時ファイルです。内容はツールが生成するため、機能追加やドキュメント更新では編集しません。

## 機能

- **タスク管理** — `tasks/` 配下の Markdown ファイル（1 タスク＝1 ディレクトリ）で管理。ステータス（todo / doing / blocked / review / done）・優先度（low / medium / high / urgent）・担当者・期限・タグ・エージェント種別（`agent_kind`）をフロントマターで保持
  - CLI: 作成・一覧（フィルタ / JSON 出力）・表示・編集・フィールド更新・アーカイブ
  - Web UI: GTD ボード（Todo / Doing / Blocked / Review / Done）とドラッグ＆ドロップ
- **ファイル管理** — プロジェクトルート起点で任意のファイル / ディレクトリを操作
  - CLI: 一覧・表示・作成（ファイル / ディレクトリ）・編集・移動・コピー・リネーム・削除
  - Web UI: Files タブ（ツリー・ファイルタブ・Monaco エディタ・Markdown プレビュー・Mermaid・アウトライン・DnD・コンテキストメニュー・ファイルアップロード（1回あたり合計1GiBまで）・実行可能ファイルの実行・git 状態表示・お気に入り / 最近開いたファイル）と Graph タブ（Markdown ファイルのリンク構造図）
- **Git** — ブランチ・ステージ / アンステージ・破棄・コミット（staged-only / amend 対応）・checkout・pull / push・ログ・diff、remoteとの差分判定・fetch をディレクトリスコープ付きで操作。変更ファイルはdiffを見ながら編集・保存できる
- **Herdr エージェント連携** — `herdr` CLI と連携し、タスクディレクトリに AI エージェント（opencode 等）を割り当て
  - `task create --agent-kind opencode --prompt ...` でタスク作成と同時にエージェントを起動
  - Herdr タブでワークスペース / タブ / ペイン / エージェントを操作（プロンプト送信・出力参照・キー送信・分割・リサイズ）
- **ターミナル** — プロジェクトディレクトリを cwd とする PTY シェルを WebSocket 経由で起動（リロード後も継続する永続セッション対応）
- **Stats** — ホストの CPU / メモリ / ディスク / ネットワーク / プロセスをリアルタイム表示
- **HTTP API + Web UI** — `serve` コマンドで起動。OpenAPI スペック（`openapi.yaml`）から生成された REST API を提供

## インストール

**前提:** Go 1.25+、Node.js（Web UI のビルドに必要）

```bash
git clone https://github.com/syunkitada/myaitoolbox
cd myaitoolbox/agenttools/mybox

make web-build          # Web UI をビルドして internal/webui/dist へコピー
go install ./cmd/mybox  # $GOPATH/bin/mybox にインストール
```
> [!NOTE]
> `make web-build` を先に実行しないと、Web UI が含まれない空のバイナリがインストールされます。
> `go install github.com/syunkitada/myaitoolbox/mybox/cmd/mybox@latest` のようなリモートからの直接インストールは Web UI が含まれないため非推奨です。

Herdr タブ・AI エージェント連携を使う場合は `herdr` CLI が PATH 上にある必要があります（オプション）。

## 使い方

### プロジェクト登録

```bash
mybox project add ~/workspace/myproject   # ディレクトリをプロジェクトとして登録
mybox project list
mybox project remove myproject
```

初回に登録したプロジェクトがデフォルトになります。`--project <name>` フラグは全コマンドの共通オプションです。

### タスク

```bash
mybox task create --project proj --name "Design the login flow"
mybox task list --project proj
mybox task list --project proj --status doing --tag web
mybox task list --project proj --all          # アーカイブ済みも含める
mybox task show --project proj 20260802_design-the-login-flow
mybox task set --project proj <task-id> --status review --priority high --tags web,ux
mybox task edit --project proj <task-id>      # $EDITOR で編集
mybox task archive --project proj <task-id>
```

AI エージェントを起動してタスクを開始する場合:

```bash
# タスク作成と同時に opencode エージェントを起動し、プロンプトテンプレートを送信
mybox task create --name "Plan the release" --agent-kind opencode --prompt plan-the-task
# エージェントを起動しない
mybox task create --name "Quick note" --no-agent
```

`--prompt` はプロジェクトの `prompts/<name>.md`、または組み込みの `templates/prompts/<name>.yaml` を参照するテンプレート名（例: `plan-the-task`, `do-the-task`）か、インラインのプロンプト文字列です。テンプレート内の `$task_file_path` がタスクのファイルパスに展開されます。

### ファイル

```bash
mybox files list                   # プロジェクトルート起点でファイル一覧
mybox files list docs --hidden     # 指定ディレクトリ配下のみ（隠しファイル含む）
mybox files show notes/arch.md
mybox files create notes/new.md
mybox files mkdir assets
mybox files edit notes/arch.md     # $EDITOR で編集
mybox files move old.md new.md
mybox files copy old.md copy.md
mybox files rename old.md renamed.md
mybox files delete notes/old.md
```

### Web UI

```bash
mybox serve --project proj
# http://127.0.0.1:8080 （ブラウザ自動起動）
```

オプション: `--host` `--port` `--no-browser` `--base-path`

#### Web UI の構成

- **グローバル（サイドバー）** — Workspaces（プロジェクト管理）/ Board（全プロジェクト横断のボード）/ Stats。各プロジェクトに git 状態・herdr ワークスペース状態が表示されます
- **Files（プロジェクトタブ）** — エクスプローラー（パスフィルタ / お気に入り / 最近開いたファイル / git 状態 / 実行 / DnD / コンテキストメニュー）と、フロントマターと本文を分けて編集できるエディタ、Markdown プレビュー（Mermaid・アウトライン）を備えたファイルビューア
- **Board** — タスクの GTD ボード。ドラッグ＆ドロップでステータスを変更（フロントマターに反映）
- **Graph** — プロジェクト内の Markdown ファイル（タスク・ドキュメントなど）をノード、Markdown リンクをエッジとして描画するグラフ。ディレクトリをフレームとして表示し、エクスプローラーでスコープを絞れます
- **Git** — リポジトリ初期化 / ステージ / アンステージ / 破棄 / コミット（staged-only・amend） / ブランチ / fetch / pull / push / ログ / diff。remoteとの差分から同期済み・Push必要・Pull必要・分岐を表示し、ディレクトリスコープにも対応。選択した変更ファイルのdiffを確認しながら編集・保存可能
- **Herdr** — ワークスペース・タブ・ペイン・エージェントの一覧と操作（プロンプト送信・出力参照・キー送信・リネーム・分割・クローズ・リサイズ）。タスクのファイルからエージェントを起動できます
- **ターミナル** — ヘッダーのターミナルボタンで開閉。プロジェクトごとの永続シェルセッションを提供

### ベースパス配下で公開する場合

リバースプロキシ配下などで `/mybox/` のような特定パス配下から Web UI を配信できます。

```bash
mybox serve --project proj --base-path /mybox
# http://127.0.0.1:8080/mybox/ 配下で表示（ブラウザ自動起動）
```

- アセットは相対パスで生成されるため、ビルド後の配置場所に依存しません。
- API は `{base-path}/api/...` にマウントされ、Web UI 側も自動で `{base-path}` を検出して呼び出します。
- プロジェクトは `/projects/{project}/` 配下のパスベースルーティングで提供されます（例: `/projects/proj/dashboard/files/notes/architecture.md`）。`--base-path` 指定時はその配下に自動的に前置されます。

## データ構造

```
myproject/
├── tasks/
│   └── 20260802_design-the-login-flow/
│       └── task.md          # フロントマターで status / priority / assignee / due / tags / agent_kind 等を保持
├── notes/
│   ├── index.md
│   └── architecture.md
├── prompts/                 # 任意: プロンプトテンプレートの上書き
│   └── plan-the-task.md
└── templates/               # 任意: タスク雛形の上書き
    └── task/task.md
```

- タスク ID は `YYYYMMDD_<slug>` 形式（例: `20260802_design-the-login-flow`）で、常に `tasks/<id>/task.md` に保存されます。
- タスクのメタデータはフロントマター、Markdown ファイル同士の関連付けは通常の Markdown リンクで記述します。
- 従来の `tasks/adhoc/<id>.md`（アーカイブは `archives/tasks/adhoc/`）というレガシーレイアウトもそのまま読み込み・更新・アーカイブできます。
- 完了後はアーカイブして `archives/tasks/<id>/` へ移動できます（`task archive`）。

### テンプレート

タスク作成時の雛形はリポジトリの `templates/task/task.yaml` としてバイナリに組み込まれています。プロジェクトの `templates/task/task.md` に同名の雛形を置くと上書きできます。プロンプトはリポジトリの `templates/prompts/*.yaml` が組み込みの既定値で、プロジェクトの `prompts/*.md` または既定プロジェクトの同名ファイルが優先されます。

### 設定ファイル

- `$XDG_CONFIG_HOME/mybox/config.yaml`（デフォルト `~/.config/mybox/config.yaml`）にプロジェクト一覧とデフォルトプロジェクトを保存します。`MYBOX_CONFIG` 環境変数でパスを変更できます。
- お気に入り・最近開いたファイルなどの状態は同じディレクトリの `state.yaml` に保存されます。

## 開発

```bash
make web-build # Web UI をビルドして internal/webui/dist へコピー
make build     # ./mybox バイナリを生成（go install の代わりにローカル確認用）
make test      # Go のユニットテスト
make lint      # golangci-lint
make e2e       # CLI E2E + Playwright E2E
make web-dev   # Vite 開発サーバー（web/）
```

## テスト

- Go ユニットテスト: リポジトリ・ユースケース・HTTP API（httptest）
- Web テスト: Vitest（`cd web && npm test`）
- Playwright E2E: Files / Board / Graph / Git / Herdr / Terminal / プロジェクト管理ほか
- CLI E2E: `tests/cli_e2e.sh`（一時プロジェクトで一連のコマンドを検証）
