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
| [`browser-worker/README.md`](./browser-worker/README.md) | PlaywrightでWebページを操作・観測する外部worker。 |
| [`tests/README.md`](./tests/README.md) | CLI E2E とインストール先ガードのテスト。 |
| [`web/README.md`](./web/README.md) | Web UI のソース、ビルド、テスト。 |
| [`node_modules/README.md`](./node_modules/README.md) | Git 管理下に残っている開発ツールの実行時生成物。編集対象外。 |
| [`.gitignore`](./.gitignore) | ローカル生成物・ビルド成果物の除外設定。 |
| [`.golangci.yml`](./.golangci.yml) | Go lint の設定。 |
| [`Makefile`](./Makefile) | ビルド、インストール、テスト、lint、Web UI・ブラウザworker開発用コマンド。 |
| [`go.mod`](./go.mod) | Go モジュール名と直接依存関係。 |
| [`go.sum`](./go.sum) | Go 依存関係のチェックサム。 |
| [`openapi.yaml`](./openapi.yaml) | HTTP API の仕様。生成コードの入力。 |
| [`README.md`](./README.md) | リポジトリ全体の使い方と構成。本文書。 |

`node_modules/` はGit管理下に残っている開発ツールの実行時ファイルです。内容はツールが生成するため、機能追加やドキュメント更新では編集しません。テスト実行結果は `.gitignore` で除外しています。

## 機能

- **タスク管理** — `_tasks/` 配下の Markdown ファイル（1 タスク＝1 ディレクトリ）で管理。タイトルはディレクトリ名（`YYYYMMDD_<slug>`）を使用し、ディレクトリ名全体をUnicode文字数で80文字以内に制限します。フロントマターにはステータス（todo / doing / blocked / review / done）・期限・保留情報・タグを保持。優先度・担当者・説明・Agent種別はTaskの正式メタデータとして扱わない。新規作成時は雛形や入力に残る旧キーを除去し、既存タスクの読み取り・更新時も `title` は使用・保存しない
  - CLI: 作成・一覧（アクティブタスクのみ。フィルタ / JSON 出力）・表示・編集・フィールド更新・アーカイブ
  - Web UI: GTD ボード（Todo / Doing / Blocked / Review / Done）とドラッグ＆ドロップ
  - 同じプロジェクト内では、アクティブ・アーカイブ済みを含めてタスクディレクトリ名を一意に管理
- **自動タスク実行** — `_task_triggers/<id>/trigger.yaml` と `task.md` で定義したcron・ファイル作成・manualトリガーから、`_tasks/` に実行タスクを生成。Trigger IDは生成されるタスクディレクトリ名が80文字以内に収まる長さに制限
  - CLI: `automation validate`、`list`、`run-once`、`daemon`、`runs`
  - Web UI: 新規タスクモーダルからtriggerを作成し、triggerディレクトリの右クリックから即時実行
- **ファイル管理** — プロジェクトルート起点で任意のファイル / ディレクトリを操作。実行権限のあるファイルはFilesタブから実行でき、標準出力と標準エラーを実行中から逐次確認できます
  - CLI: 一覧・表示・作成（ファイル / ディレクトリ）・編集・移動・コピー・リネーム・削除
  - Web UI: Files タブ（ツリー・ファイルタブ・Monaco エディタ・Markdown プレビュー・Mermaid・Vega-Lite・アウトライン・PDFのブラウザ標準ビューア表示（読み取り専用）・HTMLのsandbox付きプレビュー（JavaScriptを実行し、相対参照のプロジェクト内ファイルも表示。新しいタブでページ単体表示も可能）・テキストとして非対応の形式はプレビューせずダウンロード・DnD・コンテキストメニュー・メインとReferenceの2ペイン表示（メインを切り替えてもReferenceを維持し、同じファイルも表示可能）・ファイルアップロード（1回あたり合計1GiBまで）・実行可能ファイルの実行と標準出力 / 標準エラーの逐次表示（大きな出力は最新部分を保持）、サーバー管理ジョブによるブラウザ再読み込み後も継続する実行、WebSocket切断後の自動再接続、同時実行上限16件、モーダルを閉じても継続する実行状態の右サイドバー表示・再表示・停止・Dismiss・完了済み履歴は直近20件・git 状態表示・変更ファイルの本文とGit差分の左右比較・プロジェクト付きのお気に入りの追加 / 削除ができ、プロジェクトをまたいで開けるモーダル / 最近開いたファイルタブ・タスクのステータス変更 / アーカイブ）と Graph タブ（Markdown ファイルのリンク構造図）
- **Git** — ブランチ・ステージ / アンステージ・破棄・コミット（staged-only / amend 対応、コミットメッセージ入力中の `Ctrl+Enter` 確定）・checkout・pull / push・ログ・diff、remoteとの差分判定・fetch をディレクトリスコープ付きで操作。変更ファイルはdiffを見ながら編集・保存できる
- **Herdr エージェント連携** — `herdr` CLI と連携し、タスクディレクトリに AI エージェント（opencode 等）を割り当て。Filesでタスクディレクトリまたは配下のファイルを表示中は、ビューアのAgentボタンから未起動のエージェントをモーダルで起動し、対応するAgentパネルを開けます。プロジェクト共通のAgent右サイドバーには「Start agent」ボタンがあり、Agent kindとTab（初期値`main`、自由入力可）を指定して一般Agentを起動できます。指定TabがHerdr上に存在すれば同じTabを使い、なければ作成します。ボタンは文字を使わず、未起動、起動済み・パネル非表示、起動済み・パネル表示中の3状態を色と形状で示し、起動済みの場合はエージェント自身の `working` / `blocked` / `idle` / `done` などの状態を色付きドットとツールチップで確認できます。エージェントの停止・出力確認・操作はプロジェクト共通のAgent右サイドバーで行えます。端末出力は直近200行から表示し、「Load older output」で200行ずつ最大5,000行まで追加して遡れます。右サイドバーから、どのタブにいても `/new`・`/compact`・`/help`・`/resume`・`/plan`・`/status`、定型プロンプト「進めて」「次は何をするとよいですか？」「セルフレビューして」「振り返りをして」「Commitして」をボタンで送信可能。通常のSendに加えて、指定時刻の予約送信、予約一覧、キャンセルに対応し、予約はサーバー側へ永続化されるためブラウザを閉じてもmybox起動中なら送信されます。端末出力とプロンプト入力は縦方向に高さを調整できます
  - `task create --agent-kind opencode --prompt ...` でタスク作成と同時にエージェントを起動
  - Herdr タブでワークスペース / タブ / ペイン / エージェントを操作（プロンプト送信・出力参照・キー送信・フォーカス・分割・リサイズ）
- **Codex Diagnostics** — AgentタブでCodex CLIのバージョン、認証設定の有無、モデル・実行ポリシー、設定ファイル、適用されるAGENTS.md、Skills、MCP設定を読み取り専用で確認できます。Codex runtimeがwarning/errorの場合は、`codex doctor`の非OKチェックと安全な要約を表示します。Config・Instructionsは項目を選択して本文を確認でき、SkillsはSkill一覧からディレクトリツリーを展開して配下のファイルを選択できます。Skillsが標準の列挙上限を超える場合は切り詰め状態を表示します。Configの秘匿値はマスクされます。Config・Instructions・Skillや配下のファイルがシンボリックリンク経由の場合は明示し、標準候補として検出されたリンク先の本文を表示します。MCPサーバーは起動せず、疎通確認も行いません。セッション履歴は表示しません
- **ターミナル** — プロジェクトディレクトリを cwd とする PTY シェルを WebSocket 経由で起動（リロード後も継続する永続セッション対応）
- **Stats** — ホストの CPU / メモリ / ディスク / ネットワーク / プロセスをリアルタイム表示
- **HTTP API + Web UI** — `serve` コマンドで起動。OpenAPI スペック（`openapi.yaml`）から生成された REST API を提供
- **Internal browser** — プロジェクトごとに永続化されたPlaywright browser sessionを起動し、AIのアクセシビリティ観測・semantic reference操作と、ユーザーのライブ映像操作を同じセッションで切り替え可能。最新映像フレームはUI再接続時に再送し、AI操作後にも画面を更新する。承認はセッション単位で既定の手動確認と、UIからユーザーが有効化する「安全な操作を自動承認」から選択できる。自動モードはページ移動と判定上低リスクなリンク・メニュー操作に限り、送信・購入・削除などは引き続き承認対象。機密フィールドへの入力はユーザー専用。ページ上のラベル判定は完全ではないため、サイト側の予期しない副作用が起こる可能性があり、ユーザーはいつでも制御を引き取れる。初期版はpublic HTTPSのみ。workerが異常終了した場合はセッションを`error`状態で保持し、UIから明示的に再起動できる。再起動時もプロファイルは保持するが、承認モードは手動に戻る。

## インストール

**前提:** Go 1.25+、Node.js 20+とnpm（Web UI・ブラウザworkerのセットアップに必要）、ripgrep（`rg`、Files本文検索に必要）

```bash
git clone https://github.com/syunkitada/myaitoolbox
cd myaitoolbox/agenttools/mybox

make install            # Web UIをビルドし、myboxとブラウザworkerをインストール
```
> [!NOTE]
> インストール先はGoの`GOBIN`です。未設定の場合は最初の`GOPATH/bin`を使います。インストール先が`PATH`に含まれていることを確認してください。
> インストール先を変更する場合は、絶対パスで`make install GOBIN=/path/to/bin`を指定してください。
> `make install`はworkerの依存関係を`npm ci`で導入し、Playwright用Chromiumもインストールします。ブラウザの保存先を変える場合は、`PLAYWRIGHT_BROWSERS_PATH`を指定してください。
> ブラウザworkerを使わずCLI/Web UIだけをインストールする場合は、`make web-build`の後に`go install ./cmd/mybox`を実行できます。`go install`だけではWeb UIとworkerは配置されません。
> `go install github.com/syunkitada/myaitoolbox/mybox/cmd/mybox@latest` のようなリモートからの直接インストールはWeb UIとブラウザworkerが配置されないため非推奨です。

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
mybox task show --project proj 20260802_design-the-login-flow
mybox task set --project proj <task-id> --status review --tags web,ux
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

リポジトリからインストールする場合、workerの依存関係とChromiumは`make install`が導入します。開発・E2E用にリポジトリ内のworkerをセットアップする場合は、次を実行します。

```bash
make browser-worker-install
```

`mybox`は既定でプロジェクトごとに`browser-worker/src/index.mjs`をNode.js workerとして起動します。別のworkerを使う場合は`MYBOX_BROWSER_WORKER`でパスを指定できます。workerは127.0.0.1だけで待ち受け、セッションごとのChromiumプロファイルはユーザー設定ディレクトリ配下に保存されます。workerは同時に最大4個まで起動し、`MYBOX_BROWSER_MAX_WORKERS`で変更できます（`0`は無制限）。workerが切断すると、myboxはそのプロジェクトの関連セッションの操作を停止し、UIに再起動操作を表示します。他のプロジェクトのworkerは継続します。再起動は新しいセッションを作成しますが、既存のプロファイルを自動的に再接続・自動操作することはありません。`MYBOX_BROWSER_IDLE_TIMEOUT`に正のduration（例:`30m`）を指定すると、AI操作、ユーザー操作権、承認待ち、接続中のUIがないセッションを自動終了します。既定値は無効です。タイムアウト後もブラウザプロファイルは保持されます。セッション作成、再起動、終了、takeover、control返却、承認、機密フィールド拒否、worker障害、AIコマンドの結果は、既定でユーザー設定ディレクトリの`browser-audit.jsonl`にowner-only権限で追記されます。既定では10MiBに達するとローテーションし、退避ファイルを5世代保持します（現行を含む最大6ファイル）。Browser画面のAudit logから、現在のプロジェクトについて全保持世代を最新順に確認できます。保存先は`MYBOX_BROWSER_AUDIT_LOG`、容量は`MYBOX_BROWSER_AUDIT_MAX_BYTES`（バイト、既定10485760。`0`でローテーション無効）、退避世代数は`MYBOX_BROWSER_AUDIT_BACKUPS`（既定5、0〜100）で変更できます。退避世代数を`0`にするとローテーション時に旧ログを保持しません。設定値が不正な場合は既定値を使います。監査ログにはURL、クエリ、フォーム値、入力値、token、password、エラー本文を記録しません。myboxをloopback以外で公開してMCPからagent操作を許可する場合は、myboxとmymcpserveの両方に同じ`MYBOX_BROWSER_AGENT_TOKEN`を設定してください。ユーザー向けのブラウザAPIはloopback接続を前提とし、外部から使う場合は認証済みのリバースプロキシ配下で運用してください。

監査ログの保存先に`.lock`を付けたowner-onlyファイルを同じディレクトリに作成し、複数プロセス間の検索・追記・ローテーションを調停します。このロックファイルはログとともに残ります。

#### 内部ブラウザの認証・リバースプロキシ方針

mybox HTTP API全体には組み込みのユーザー認証はありません。既定の`--host 127.0.0.1`でローカル利用するか、外部公開時はTLSとユーザー認証を終端するリバースプロキシを同じホストに置き、myboxへloopbackで転送してください。`--host 0.0.0.0`だけでは認証になりません。

- ユーザー向けbrowser APIとライブ映像は、TCP接続元がloopbackの場合だけ許可します。`X-Forwarded-For`などの転送ヘッダーは信頼しません。
- リバースプロキシは認証済みリクエストだけを`127.0.0.1:8080`へ転送し、`Host`を保持し、WebSocketのUpgradeを転送してください。`Host`を置き換えるとOrigin検査に失敗します。
- ユーザー向け経路では、クライアントからの`X-Browser-Actor`と`X-Browser-Agent-Token`を削除してください。agent tokenはブラウザへ渡さず、MCPなどの信頼済みagent経路だけで使用します。
- loopback以外からMCP agentを接続する場合は、myboxとmymcpserveに同じ`MYBOX_BROWSER_AGENT_TOKEN`を設定します。tokenなしのremote agentは拒否され、tokenはユーザー認証の代替にはなりません。

Nginxの構成例（TLS証明書とBasic認証の設定は環境に合わせてください）:

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 443 ssl;
    server_name mybox.example;

    location /mybox/ {
        auth_basic "mybox";
        auth_basic_user_file /etc/nginx/.htpasswd;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header X-Browser-Actor "";
        proxy_set_header X-Browser-Agent-Token "";
    }
}
```

この例に対応してmyboxは`mybox serve --host 127.0.0.1 --port 8080 --base-path /mybox --no-browser`で起動します。MCPを同じホストで動かす場合は`MYBOX_URL=http://127.0.0.1:8080`を使い、別ホストから接続する場合は`MYBOX_URL=https://mybox.example`と、TLSで保護された認証済みのagent専用経路および`MYBOX_BROWSER_AGENT_TOKEN`を用意してください。MCPのbrowser toolは、session操作ごとに`project`と`session_id`を要求します。

オプション: `--host` `--port` `--no-browser` `--base-path`

#### Web UI の構成

- **グローバル（サイドバー）** — Projects（プロジェクト管理）/ Board（全プロジェクト横断のボード）/ Stats。各プロジェクトに git 状態・Herdr workspace 状態が表示されます
- **Files（プロジェクトタブ）** — エクスプローラー（パスフィルタ / git 状態 / 実行 / DnD / コンテキストメニュー）と、フロントマターと本文を分けて編集できるエディタ、Markdown プレビュー（Mermaid・Vega-Lite・アウトライン）、PDFのブラウザ標準ビューア、JavaScriptを実行するsandbox付きHTMLプレビューを備えたファイルビューア。HTML内の相対参照は同じプロジェクト内のファイルへ解決され、新しいタブでページ単体として開くこともできます。ビューア上部には最近開いたファイルのタブが並び、その左端の☆ボタンからプロジェクト名付きのお気に入りをモーダルで開いて、プロジェクトをまたいでファイルへ移動できます。ヘッダーの「New file」メニューからファイルまたはディレクトリを作成できます。`_tasks/<id>/task.md` を開くと、表示モードのステータス選択から即時にステータスを変更でき、ファイル操作メニューから確認付きでタスクをアーカイブできます。実行可能ファイルの実行中はモーダルを閉じても実行と出力収集を継続し、プロジェクト共通の右サイドバーから状態・結果を再表示、停止、Dismissできます。完了済み実行履歴は直近20件を保持します。タスクディレクトリ自身や配下のファイルを表示中は、ビューアのAgentボタンから未起動のタスクAgentをモーダルで起動して対応パネルを開けます。Agentボタンは文字を使わず、未起動・起動済み・パネル非表示・起動済み・パネル表示中を色と形状で状態表示し、起動済みAgentの現在ステータスを色付きドットとツールチップで確認できます。停止・出力確認・操作は右サイドバーで行えます。git 状態は折りたたまれたディレクトリにも配下の変更を集約して表示します。変更があるファイルではGitボタンから本文とGit差分を左右に並べて確認でき、各ペインを独立してスクロールできます。task_triggerの作成後・実行後は、生成されたファイルを再読込してエクスプローラーに表示します
- **Board** — タスクの GTD ボード。ドラッグ＆ドロップでステータスを変更（フロントマターに反映）し、紐づいたHerdr agentの稼働ステータスも表示
- **タスク作成** — 新規タスクモーダルから`task`または`task_trigger`を作成。YAMLヘッダーを除く`task.md`本文はテンプレートを初期表示した編集欄で入力でき、`task_trigger`ではcron式または監視ディレクトリを指定できます
- **Graph** — プロジェクト内の Markdown ファイル（タスク・ドキュメントなど）をノード、Markdown リンクをエッジとして描画するグラフ。ディレクトリをフレームとして表示し、エクスプローラーでスコープを絞れます
- **Git** — リポジトリ初期化 / ステージ / アンステージ / 破棄 / コミット（staged-only・amend） / ブランチ / fetch / pull / push / ログ / diff。remoteとの差分から同期済み・Push必要・Pull必要・分岐を表示し、ディレクトリスコープにも対応。選択した変更ファイルのdiffを確認しながら編集・保存可能
- **Herdr** — ワークスペース・タブ・ペインの一覧と操作（プロンプト送信・出力参照・キー送信・フォーカス・リネーム・分割・クローズ・リサイズ）。Agent一覧と操作はプロジェクト共通の右サイドバーで提供します。「Start agent」からkindとTabを指定した一般Agentの起動、`done`から`idle`への遷移、未確認完了状態、予約送信、タスクファイルへのリンクなどを利用できます。タスクのファイルからもエージェントを起動できます
- **ターミナル** — ヘッダーのターミナルボタンで開閉。プロジェクトごとの永続シェルセッションを提供し、タブの表示番号は現在開いているターミナル内で最小の未使用番号を再利用できます。タブはドラッグハンドルで並び替えられます

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
│       └── task.md          # フロントマターで status / due / pending_* / tags 等を保持。title はディレクトリ名から導出
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
- タスク名には日本語などのUnicode文字を使用できます。タスクAgentの内部名はHerdrの制約に合わせてASCII化し、日本語を含むタスク同士の衝突を避ける決定的な識別子を使います。
- Herdrのタブ名にはタスクディレクトリ名を表示します。タブ名を変更するとタスクディレクトリ（タスクID）とAgent内部名も変更されます。
- タスクのメタデータはフロントマター、Markdown ファイル同士の関連付けは通常の Markdown リンクで記述します。
- myboxが管理する領域であることを示すため、タスクは `_tasks/`、アーカイブは `_archives/tasks/` に保存されます。
- 完了後はアーカイブして `_archives/tasks/<id>/` へ移動できます（`task archive`）。

### テンプレート

タスク作成時の雛形はリポジトリの `templates/task/task.yaml` としてバイナリに組み込まれています。プロジェクトの `templates/task/task.md` に同名の雛形を置くと上書きできます。プロンプトはリポジトリの `templates/prompts/*.yaml` が組み込みの既定値で、プロジェクトの `prompts/*.md` または既定プロジェクトの同名ファイルが優先されます。

### 設定ファイル

- `$XDG_CONFIG_HOME/mybox/config.yaml`（デフォルト `~/.config/mybox/config.yaml`）にプロジェクト一覧とデフォルトプロジェクトを保存します。`MYBOX_CONFIG` 環境変数でパスを変更できます。
- お気に入り・最近開いたファイルなどの状態は同じディレクトリの `state.yaml` に保存され、最近開いたファイルはプロジェクト単位で分離されます。ファイルまたはディレクトリを削除すると、対応するお気に入りも自動的に削除されます。
- 自動化の実行台帳と排他ロックは同じ設定ディレクトリの `automation/` に保存され、プロジェクトには生成されません。

## 開発

```bash
make web-build    # Web UI をビルドして internal/webui/dist へコピー
make build        # ./mybox バイナリを生成（go install の代わりにローカル確認用）
make test         # インストール先ガードとGoのユニットテスト
make test-install # インストール先ガードのみ
make lint         # golangci-lint
make e2e          # CLI E2E + Playwright E2E
make web-dev      # Vite 開発サーバー（web/）
```

## テスト

- Go ユニットテスト: リポジトリ・ユースケース・HTTP API（httptest）
- インストール先ガード: `make test-install`（GOBIN、シンボリックリンク、既存ディレクトリの拒否を検証）
- Web テスト: Vitest（`cd web && npm test`）
- Playwright E2E: Files / Board / Graph / Git / Herdr / Terminal / 埋め込みブラウザ / プロジェクト管理ほか
- CLI E2E: `tests/cli_e2e.sh`（一時プロジェクトで一連のコマンドを検証）
