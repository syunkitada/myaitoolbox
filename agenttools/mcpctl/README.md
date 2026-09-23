# mcpctl

`mcpctl` は、人間と AI の両方が利用できる MCP (Model Context Protocol) ツールの管理・実行CLIです。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | `mcpctl`固有の開発手順と完了時の検証コマンド。 |
| [`cmd/`](./cmd/) | `mcpctl` CLIのエントリーポイント。 |
| [`docs/README.md`](./docs/README.md) | 設定、仕様、AIエージェント向けガイド。 |
| [`internal/README.md`](./internal/README.md) | CLIのアプリケーション、ドメイン、インフラ実装。 |
| [`.gitignore`](./.gitignore) | Goのビルド成果物やローカル設定の除外設定。 |
| [`.golangci.yml`](./.golangci.yml) | `golangci-lint`の静的解析設定。 |
| [`Makefile`](./Makefile) | ビルド、テスト、lint用コマンド。 |
| [`go.mod`](./go.mod) | Goモジュールと依存関係の定義。 |
| [`go.sum`](./go.sum) | Go依存関係のチェックサム。 |
| [`README.md`](./README.md) | `mcpctl`の概要と使い方。本文書。 |

## 特徴

- **CLI Mode**: 人間向けにわかりやすいインターフェースを提供。
- **MCP Client**: 複数のバックエンドMCPサーバーに接続し、ツールを統合的に操作。
- **Unified Conceptual API**: List, Search, Info, Call といった操作を共通の概念モデルで提供。

## アーキテクチャ概要

### CLI Mode
人間やシェルスクリプト、または CLI を操作する AI エージェントが直接叩くモードです。

```text
Human / AI
    ↓
  mcpctl
    ↓
 MCP Servers
```

## インストール

```bash
go install ./cmd/mcpctl
```

## ドキュメント

より詳細な使い方は以下のドキュメントを参照してください。

- [AIエージェント向けガイド (AGENTS.md)](./docs/AGENTS.md)
- [設定ファイルとプロファイル管理 (CONFIGURATION.md)](./docs/CONFIGURATION.md)
- [詳細仕様書 (SPECIFICATION.md)](./docs/SPECIFICATION.md)

## 基本的なワークフロー

1. **検索 (Search)**
   ツール名がわからない場合は `mcpctl search <query>` を実行します。
2. **一覧表示 (List)**
   ツール一覧を確認する場合は `mcpctl list` を実行します。
3. **情報確認 (Info)**
   実行前にパラメータなどを確認するために `mcpctl info <server>/<tool>` を実行します。
4. **実行 (Call)**
   パラメータを理解した上で `mcpctl call <server>/<tool> [flags]` を実行します。

### Human Shortcut
人間向けの探索ショートカット機能も提供しています。

```bash
# サーバ一覧
mcpctl call -h

# サーバ内のツール一覧
mcpctl call github -h

# ツール情報表示
mcpctl call github/create_issue -h
```

## シェル補完 (zsh)

```bash
# 現在のシェルに読み込む場合
source <(mcpctl completion zsh)

# 永続的に設定する場合
mcpctl completion zsh > ~/.zsh/completions/_mcpctl
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -Uz compinit && compinit' >> ~/.zshrc
```

`call` コマンドでは、ツール名の補完に続けて `--パラメータ名` の補完が効きます。`list` ではサーバ名の補完が可能です。
