# mcpserve

`mcpserve` は、複数の MCP Server 実装を単一のバイナリに内包し、指定されたサーバを簡単に起動できる Go 製ランタイムです。

## Index

| パス | 役割 |
| --- | --- |
| [`cmd/`](./cmd/) | 実行可能ファイルのエントリポイント。詳細は `cmd/README.md` を参照。 |
| [`docs/`](./docs/) | アーキテクチャと開発手順。詳細は `docs/README.md` を参照。 |
| [`internal/`](./internal/) | MCPサーバのドメイン、起動処理、実装、モジュール。詳細は `internal/README.md` を参照。 |
| [`AGENTS.md`](./AGENTS.md) | AIエージェント向けの設計・実装ガイドライン。 |
| [`.gitignore`](./.gitignore) | ビルド成果物やローカル設定の除外設定。 |
| [`.golangci.yml`](./.golangci.yml) | Go lint の設定。 |
| [`go.mod`](./go.mod) | Goモジュールと依存関係の定義。 |
| [`go.sum`](./go.sum) | Go依存関係のチェックサム。 |
| [`README.md`](./README.md) | `mcpserve`の概要と使い方。本文書。 |

## 特徴

- 複数の MCP Server 実装を単一バイナリで提供
- MCP SDK (`github.com/modelcontextprotocol/go-sdk`) に準拠した共通インターフェース (`Provider`) の採用
- シンプルなコマンドラインインターフェース
- 実行時に任意のサーバーを指定してstdioまたはHTTP経由のMCP通信を開始

## インストール

```bash
go install ./cmd/mcpserve
```

## 使い方

メインコマンドに起動したいサーバー名を指定するだけです。

### 基本構文

```bash
mcpserve <server-name>
```

### コマンド例

```bash
# サーバー一覧とヘルプの表示
mcpserve -h

# monitoringサーバーをstdioで起動
mcpserve monitoring --transport stdio

# monitoring MCP Server をHTTPで起動
mcpserve monitoring --transport http --host localhost --port 8080
```

## ドキュメント

詳細な設計や開発方法については、[docs/](./docs/) ディレクトリを参照してください。

- [アーキテクチャ・設計方針](docs/architecture.md)
- [開発ガイド（サーバーの追加方法）](docs/development.md)
- [Go プロジェクトガイド（汎用）](../../docs/golang/golang_architecture.md)

## ディレクトリ構成

```
cmd/mcpserve/main.go              # エントリーポイント
internal/
    domain/provider.go            # コアドメインインターフェース
    entrypoint/                   # Provider登録とサーバー起動
    infrastructure/               # MCP Server実装
    modules/                      # MCPプロバイダー実装
        monitoring/
            provider.go           # Provider実装とツール登録
            domain/               # プロバイダー固有の型・インターフェース
            application/          # UseCase 実装
            infrastructure/       # 外部サービスクライアント
```
