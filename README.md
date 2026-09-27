# myaitoolbox

AIエージェント向けのツール、Go製のMCP実装、個人ワークスペース管理ツールをまとめたリポジトリです。

## このリポジトリについて

`agenttools/` に、MCPサーバやAIエージェントと連携するためのGo製ツールをまとめています。主なツールは次のとおりです。

- `mcpctl`: MCPツールの検索・情報表示・実行を行うCLI
- `mcpserve`: 複数のMCPサーバ実装を束ねて起動するランタイム
- `mybox`: ローカルプロジェクトのタスク・ファイル・Git・AIエージェントなどを一元管理するワークスペースツール
- `myntfy`: 保存済みtopicでntfy通知を送受信するCLI

設計・構成の参考資料は [`docs/`](./docs/) に、リポジトリ全体の運用スクリプトは [`scripts/`](./scripts/) にあります。

## mybox

このリポジトリで個人ワークスペースを管理するときは、[`agenttools/mybox/`](./agenttools/mybox/) を使います。複数のローカルプロジェクトを対象に、プロジェクト・ファイル・タスクの管理、Git操作、AIエージェント連携、ターミナル、Web UIをまとめて提供します。タスクはMarkdownファイルとして保存し、プロジェクト内の任意のファイルを扱います。

プライベートネットワーク内での利用を想定しており、認証機能はありません。スマートフォンなどから接続する場合は、TailscaleなどのVPN越しに利用してください。

### 初回セットアップ

サーバーの `tmux` セッション上で、更新・インストールとmyboxの起動を順に実行します。

```bash
# tmux上で実行
./scripts/reinstall-mybox.sh
./scripts/start-or-restart-mybox1111.sh
```

起動スクリプト名の末尾は待ち受けポートを表します。通常は `1111` をプライマリとして使い、`1112` や `1113` は障害時にプライマリを復旧するためのセカンダリとして用意できます。セカンダリを常時起動しておく必要はありません。

### 起動後の更新・再起動

起動後の更新・再インストールは、myboxでこのリポジトリをプロジェクトとして開き、Web UIのターミナルから同じ `reinstall-mybox.sh` を実行できます。ブラウザだけで `git pull`、Web UIのビルド、myboxの再インストールまで行えます。

起動用スクリプトはmyboxを常駐させるため、初回起動時と同様に `tmux` 上で実行します。herdr内では起動しないでください。

起動後にWeb UIのターミナルから `./scripts/start-or-restart-mybox1111.sh` を実行すると、稼働中のmyboxを停止して、`tmux` 上の監視ループによる自動リスタートを行えます。更新したバイナリを反映するときも、この手順を使います。

## myboxの実行環境

myboxを運用するには、次の実行環境・ツールを用意します。

- `tmux`: インフラアプリケーションの実行基盤。
- `herdr`: AIエージェントの実行基盤。myboxのエージェント連携で利用。
- AIエージェント: `opencode`、`codex`、`antigravity`など。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | リポジトリ全体のREADME整備と作業に関するガイドライン。 |
| [`agenttools/`](./agenttools/) | `mcpctl`、`mcpserve`、`mybox`などのエージェント関連ツール。 |
| [`docs/`](./docs/) | Goプロジェクトで共有する設計・構成・技術スタックの参考資料。 |
| [`inbox/`](./inbox/) | 作業中の検証環境、提案、メモ。READMEは必須ではない領域。 |
| [`scripts/`](./scripts/) | mybox、Tailscale、myntfyの運用補助スクリプト。 |
| [`.gitignore`](./.gitignore) | リポジトリ共通の除外設定。 |
| [`README.md`](./README.md) | リポジトリ全体の入口。本文書。 |
