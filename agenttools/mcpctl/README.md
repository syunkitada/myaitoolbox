# mcpctl

`mcpctl` は、人間と AI の両方が利用できる MCP (Model Context Protocol) ツールの管理・実行CLI兼サーバです。

## 特徴

- **CLI Mode**: 人間向けにわかりやすいインターフェースを提供。
- **MCP Server Mode**: AI エージェントから、他のMCPサーバへアクセスするための統合エンドポイントとして動作。
- **Unified Conceptual API**: CLI と MCP で List, Search, Info, Call といった同じ概念モデルを共有しています。

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

### MCP Mode (`mcpctl serve`)
AI エージェントなどが、MCP 経由で mcpctl に接続するモードです。複数のバックエンド MCP サーバーを一つの MCP サーバとして束ねて公開します。

```text
AI Agent
    ↓ MCP
 mcpctl serve
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

