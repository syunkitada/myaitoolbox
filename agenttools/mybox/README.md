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
| [`docs/README.md`](./docs/README.md) | 詳細な利用者向け・開発者向けドキュメント。 |
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

## 概要

`mybox`はローカルの複数プロジェクトを、タスク、ファイル、Git、AIエージェント、ターミナルで管理するワークスペースツールです。CLIとWeb UIを備え、タスクや設定をファイルとして保存します。

- [利用ガイド](./docs/user-guide.md) — 主な機能、CLI/Web UIの使い方、設定とデータ構造。
- [ドキュメント一覧](./docs/README.md) — Markdown、アーキテクチャ、内部ブラウザ、MCPの仕様。

## インストール

**前提:** Go 1.25+、Node.js 20+とnpm、ripgrep（`rg`）。

```bash
git clone https://github.com/syunkitada/myaitoolbox
cd myaitoolbox/agenttools/mybox
make install
```

`make install`はWeb UIをビルドし、mybox、ブラウザworkerとPlaywright用Chromiumをインストールします。インストール先は`GOBIN`、未設定時は`GOPATH/bin`です。Herdr連携には`herdr` CLIが必要です。詳細は[利用ガイド](./docs/user-guide.md)を参照してください。

## クイックスタート

```bash
mybox project add ~/workspace/myproject
mybox task create --name "最初のタスク"
mybox task list
mybox serve
```

## 開発

```bash
make web-build
make build
make test
make lint
make e2e
```

コマンドの一覧とテスト範囲は[利用ガイド](./docs/user-guide.md)および[`Makefile`](./Makefile)を参照してください。
