# `myherdr`

Codex が Herdr の名前付きセッションをプロジェクトごとに使い、必要なときに同じ terminal pane をブラウザーへ引き継ぐ Go 製 CLI と Codex skill です。セッションと pane の作成・管理は Herdr に任せ、ブラウザー端末は Herdr の terminal stream を利用します。

## インストール

リポジトリのルートから CLI をインストールします。

```bash
cd agenttools/myherdr
go install ./cmd/myherdr
```

Herdr CLI が別途 PATH 上に必要です。myherdr は Herdr がない場合に案内を表示します。

Codex skill を利用するには、Codex が読む skills ディレクトリに `skills/myherdr/` を配置します。詳細は [`skills/myherdr/README.md`](./skills/myherdr/README.md) を参照してください。

## 基本操作

プロジェクトの Git ルートを基準に名前付き Herdr セッションを起動します。Git 管理外では現在のディレクトリを使います。

```bash
myherdr start
myherdr session-name
```

既定のセッション名は `codex-<project-name>` を ASCII slug にしたもので、Herdr の上限に合わせて64文字以内になります。利用者は `myherdr session-name` が表示した `herdr --session ...` を使って同じセッションへ接続できます。既に Herdr pane 内で `myherdr start` を実行した場合、入れ子の Herdr は起動しません。

ユーザーへ terminal 操作を引き継ぐときは、Herdr の専用 pane で Web サーバーを起動したままにします。

```bash
myherdr serve
myherdr url codex-my-project
```

`serve` は `127.0.0.1` にだけ bind し、既定では空きポートを使います。Linux、macOS、Android、BSD、illumos、AIX、Solaris、Windows では OS file lock で単一起動を制御します。JS/WASM、Plan 9、WASI では `serve` は未対応エラーを返します。`myherdr url` は起動中の serve インスタンスを確認してから URL を表示します。Codex pane が一つのときは自動接続し、複数あるときはブラウザーで選べます。`--pane <pane-id>` を指定するとその既存 pane を直接開きます。読み取り専用接続も選択できます。Web アプリは pane を作成・削除せず、既存の制御接続を自動 takeover しません。

SSH を使う前に、Codex skill は利用プロジェクトの `.myherdr/ssh-hosts.md` を確認します。ひな形は [`templates/ssh-hosts.md`](./templates/ssh-hosts.md) です。この一覧は Codex の運用方針であり、SSH 自体の技術的な遮断ではありません。SSH の鍵、パスワード、MFA 情報は保存しません。

リモート Herdr pane をブラウザーで開く場合は、[`templates/sessions.toml`](./templates/sessions.toml) のように Web URL 名、SSH host、リモートの Herdr session を設定し、host を同じプロジェクトの `.myherdr/ssh-hosts.md` にも記載します。リモート接続には SSH agent または鍵による非対話認証が必要です。

## 開発

Web 端末資材は [`internal/myherdr/web/`](./internal/myherdr/web/) に同梱し、Go バイナリへ埋め込みます。ビルド時に Node.js は必要ありません。xterm.js と fit addon の出所・ライセンスは `internal/myherdr/web/LICENSES.md` に記録しています。

```bash
make build
go install ./cmd/myherdr
```

## Index

| パス | 役割 |
| --- | --- |
| [`.gitignore`](./.gitignore) | ローカルビルドの myherdr バイナリを除外。 |
| [`Makefile`](./Makefile) | CLI の build、test、lint、clean ターゲット。 |
| [`README.md`](./README.md) | インストール、使い方、構成の入口。 |
| [`cmd/`](./cmd/) | CLI entrypoint。詳細は [`cmd/README.md`](./cmd/README.md) を参照。 |
| [`docs/`](./docs/) | 設計と運用の説明。詳細は [`docs/README.md`](./docs/README.md) を参照。 |
| [`go.mod`](./go.mod) | Go module と依存関係。 |
| [`go.sum`](./go.sum) | Go 依存関係のチェックサム。 |
| [`internal/`](./internal/) | CLI、Herdr bridge、localhost サーバー。詳細は [`internal/README.md`](./internal/README.md) を参照。 |
| [`skills/`](./skills/) | Codex 向け skill。詳細は [`skills/README.md`](./skills/README.md) を参照。 |
| [`templates/`](./templates/) | 利用プロジェクトへ配置する SSH policy とリモート session 設定の例。詳細は [`templates/README.md`](./templates/README.md) を参照。 |
