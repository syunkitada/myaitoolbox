# mybox

ローカルの複数プロジェクトを、タスク（GTD）・ファイル・Git・AIエージェント・ターミナルなどの機能で一元管理する個人ワークスペースツールです。
CLI と Web UI の両方から操作でき、タスクは Markdown ファイルとして保存し、プロジェクト内の任意のファイルを扱います。データベースは不要で、プロジェクト設定や UI 状態を含むデータはファイルとして保存されます。

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

- **タスク管理** — `_tasks/` 配下の Markdown ファイル（1 タスク＝1 ディレクトリ）で管理。ステータス（todo / doing / blocked / review / done）・優先度（low / medium / high / urgent）・担当者・期限・タグ・エージェント種別（`agent_kind`）をフロントマターで保持
  - CLI: 作成・一覧（フィルタ / JSON 出力）・表示・編集・フィールド更新・アーカイブ
  - Web UI: GTD ボード（Todo / Doing / Blocked / Review / Done）とドラッグ＆ドロップ
- **自動タスク実行** — `_task_triggers/<id>/trigger.yaml` と `task.md` で定義したcron・ファイル作成・manualトリガーから、`_tasks/` に実行タスクを生成
  - CLI: `automation validate`、`list`、`run-once`、`daemon`、`runs`
  - Web UI: 新規タスクモーダルからtriggerを作成し、triggerディレクトリの右クリックから即時実行
- **ファイル管理** — プロジェクトルート起点で任意のファイル / ディレクトリを操作。実行権限のあるファイルはFilesタブから実行でき、標準出力と標準エラーを実行中から逐次確認できます
  - CLI: 一覧・表示・作成（ファイル / ディレクトリ）・編集・移動・コピー・リネーム・削除
  - Web UI: Files タブ（ツリー・ファイルタブ・Monaco エディタ・Markdown プレビュー・Mermaid・Vega-Lite・アウトライン・DnD・コンテキストメニュー・メインとReferenceの2ペイン表示（メインを切り替えてもReferenceを維持し、同じファイルも表示可能）・ファイルアップロード（1回あたり合計1GiBまで）・実行可能ファイルの実行と標準出力 / 標準エラーの逐次表示・git 状態表示・変更ファイルの本文とGit差分の左右比較・お気に入り / 最近開いたファイル・タスクのステータス変更 / アーカイブ）と Graph タブ（Markdown ファイルのリンク構造図）
- **Git** — ブランチ・ステージ / アンステージ・破棄・コミット（staged-only / amend 対応、コミットメッセージ入力中の `Ctrl+Enter` 確定）・checkout・pull / push・ログ・diff、remoteとの差分判定・fetch をディレクトリスコープ付きで操作。変更ファイルはdiffを見ながら編集・保存できる
- **Herdr エージェント連携** — `herdr` CLI と連携し、タスクディレクトリに AI エージェント（opencode 等）を割り当て。Web UI の agent パネルから `/new`・`/compact`・`/help`・`/resume`・`/plan`・`/status`、定型プロンプト「進めて」「次は何をするとよいですか？」「セルフレビューして」「Commitして」をボタンで送信可能。通常のSendに加えて、指定時刻の予約送信、予約一覧、キャンセルに対応し、予約はサーバー側へ永続化されるためブラウザを閉じてもmybox起動中なら送信されます。端末出力とプロンプト入力は縦方向に高さを調整できます
  - `task create --agent-kind opencode --prompt ...` でタスク作成と同時にエージェントを起動
  - Herdr タブでワークスペース / タブ / ペイン / エージェントを操作（プロンプト送信・出力参照・キー送信・フォーカス・分割・リサイズ）
- **ターミナル** — プロジェクトディレクトリを cwd とする PTY シェルを WebSocket 経由で起動（リロード後も継続する永続セッション対応）
- **Stats** — ホストの CPU / メモリ / ディスク / ネットワーク / プロセスをリアルタイム表示
- **HTTP API + Web UI** — `serve` コマンドで起動。OpenAPI スペック（`openapi.yaml`）から生成された REST API を提供

## インストール

**前提:** Go 1.25+、Node.js（Web UI のビルドに必要）、ripgrep（`rg`、Files本文検索に必要）

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

自動タスク実行を設定する場合は、プロジェクト直下にトリガー定義を作成します。

```text
_task_triggers/
└── daily_report/
    ├── trigger.yaml
    └── task.md
```

`trigger.yaml` の例:

```yaml
version: 1
enabled: true
trigger:
  type: cron
  cron: "0 9 * * 1-5"
  timezone: Asia/Tokyo
task:
  agent_kind: opencode
  prompt: do-the-task
```

手動実行だけのtriggerは、`trigger` を次のように定義します。cronやファイル監視では自動発火せず、CLIまたはWeb UIから実行したときだけタスクを生成します。

```yaml
trigger:
  type: manual
```

作成時にPromptを指定しない場合は、次のデフォルトプロンプトが`trigger.yaml`に保存されます。`$task_file_path`は実行時に生成されたタスクのパスへ展開されます。

```yaml
task:
  prompt: "'$task_file_path' を実施してください。"
```

確認と手動実行は次のコマンドで行います。

```bash
mybox automation validate
mybox automation list
mybox automation run-once
mybox automation run-once --id manual_report
mybox automation daemon
mybox automation runs
```

ファイル作成トリガーでは、`trigger.yaml` の `type` を `file_created`、`path` を `incoming` に設定し、`_task_triggers/<id>/incoming/` に対象ファイルを配置します。生成されたタスクは既存の `_tasks/` に保存されます。

Web UIでは `_task_triggers/<id>` ディレクトリを右クリックして `Run trigger` を選択すると、trigger typeに関係なく1回実行できます。

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

- **グローバル（サイドバー）** — Projects（プロジェクト管理）/ Board（全プロジェクト横断のボード）/ Stats。各プロジェクトに git 状態・Herdr workspace 状態が表示されます
- **Files（プロジェクトタブ）** — エクスプローラー（パスフィルタ / お気に入り / 最近開いたファイル / git 状態 / 実行 / DnD / コンテキストメニュー）と、フロントマターと本文を分けて編集できるエディタ、Markdown プレビュー（Mermaid・Vega-Lite・アウトライン）を備えたファイルビューア。ヘッダーの「New file」メニューからファイルまたはディレクトリを作成できます。`_tasks/<id>/task.md` を開くと、表示モードのステータス選択から即時にステータスを変更でき、ファイル操作メニューから確認付きでタスクをアーカイブできます。git 状態は折りたたまれたディレクトリにも配下の変更を集約して表示します。変更があるファイルではGitボタンから本文とGit差分を左右に並べて確認でき、各ペインを独立してスクロールできます。task_triggerの作成後・実行後は、生成されたファイルを再読込してエクスプローラーに表示します
- **Board** — タスクの GTD ボード。ドラッグ＆ドロップでステータスを変更（フロントマターに反映）し、紐づいたHerdr agentの稼働ステータスも表示
- **タスク作成** — 新規タスクモーダルから`task`または`task_trigger`を作成。YAMLヘッダーを除く`task.md`本文はテンプレートを初期表示した編集欄で入力でき、`task_trigger`ではcron式または監視ディレクトリを指定できます
- **Graph** — プロジェクト内の Markdown ファイル（タスク・ドキュメントなど）をノード、Markdown リンクをエッジとして描画するグラフ。ディレクトリをフレームとして表示し、エクスプローラーでスコープを絞れます
- **Git** — リポジトリ初期化 / ステージ / アンステージ / 破棄 / コミット（staged-only・amend） / ブランチ / fetch / pull / push / ログ / diff。remoteとの差分から同期済み・Push必要・Pull必要・分岐を表示し、ディレクトリスコープにも対応。選択した変更ファイルのdiffを確認しながら編集・保存可能
- **Herdr** — ワークスペース・タブ・ペイン・エージェントの一覧と操作（プロンプト送信・出力参照・キー送信・フォーカス・リネーム・分割・クローズ・リサイズ）。agentパネルを開くとHerdr側のフォーカスも更新され、`done`から`idle`へ遷移できます。Web UI側ではHerdrの生ステータスと独立した`mybox focused`状態を管理し、見ていないagentの完了（`done`）はHerdr側で`idle`になってもmyboxでパネルを開くまで表示を保持します。パネルを閉じる・別agentへ切り替える・別ページへ移動する操作でmybox側のフォーカスを更新します。agentパネルの開閉状態はプロジェクトごとに保存され、別プロジェクトから戻ったときに復元されます。Stop操作ではagentへ割り込みを送った後にペインを閉じ、同じタブに他のペインがなければタブも閉じます。agentパネルから`/status`を含むクイックコマンドを送信でき、通常のSendとは別にサーバー永続化された指定時刻の予約送信（一覧・キャンセル・再読み込み復元）も利用でき、ブラウザを閉じてもmybox起動中なら送信されます。端末出力とプロンプト入力は縦方向に高さを調整できます。タスクのファイルからエージェントを起動できます
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
├── _archives/
│   └── tasks/
├── _tasks/
│   └── 20260802_design-the-login-flow/
│       └── task.md          # フロントマターで status / priority / assignee / due / tags / agent_kind 等を保持
├── _task_triggers/
│   └── daily_report/
│       ├── trigger.yaml     # 自動タスクの発動条件と実行設定
│       └── task.md          # 実行タスクのテンプレート
├── notes/
│   ├── index.md
│   └── architecture.md
├── prompts/                 # 任意: プロンプトテンプレートの上書き
│   └── plan-the-task.md
└── templates/               # 任意: タスク雛形の上書き
    └── task/task.md
```

- タスク ID は `YYYYMMDD_<slug>` 形式（例: `20260802_design-the-login-flow`）で、常に `_tasks/<id>/task.md` に保存されます。
- タスクのメタデータはフロントマター、Markdown ファイル同士の関連付けは通常の Markdown リンクで記述します。
- myboxが管理する領域であることを示すため、タスクは `_tasks/`、アーカイブは `_archives/tasks/` に保存されます。
- 完了後はアーカイブして `_archives/tasks/<id>/` へ移動できます（`task archive`）。

### テンプレート

タスク作成時の雛形はリポジトリの `templates/task/task.yaml` としてバイナリに組み込まれています。プロジェクトの `templates/task/task.md` に同名の雛形を置くと上書きできます。プロンプトはリポジトリの `templates/prompts/*.yaml` が組み込みの既定値で、プロジェクトの `prompts/*.md` または既定プロジェクトの同名ファイルが優先されます。

### 設定ファイル

- `$XDG_CONFIG_HOME/mybox/config.yaml`（デフォルト `~/.config/mybox/config.yaml`）にプロジェクト一覧とデフォルトプロジェクトを保存します。`MYBOX_CONFIG` 環境変数でパスを変更できます。
- お気に入り・最近開いたファイルなどの状態は同じディレクトリの `state.yaml` に保存されます。
- 自動化の実行台帳と排他ロックは同じ設定ディレクトリの `automation/` に保存され、プロジェクトには生成されません。

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
