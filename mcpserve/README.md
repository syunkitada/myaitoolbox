# mcpserve

`mcpserve` は、複数の MCP Server 実装を単一のバイナリに内包し、指定されたサーバを簡単に起動できる Go 製ランタイムです。

## 特徴

- 複数の MCP Server 実装を単一バイナリで提供
- MCP SDK (`github.com/modelcontextprotocol/go-sdk`) に準拠した共通インターフェース (`Provider`) の採用
- シンプルなコマンドラインインターフェース
- 実行時に任意のサーバーを指定して標準入出力を介した MCP 通信を開始

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

# 特定のサーバーのヘルプを表示
mcpserve github -h

# GitHub MCP Server の起動
mcpserve github
```

## ドキュメント

詳細な設計や開発方法については、[docs/](./docs/) ディレクトリを参照してください。

- [アーキテクチャ・設計方針](docs/architecture.md)
- [開発ガイド（サーバーの追加方法）](docs/development.md)
- [Go プロジェクトガイド（汎用）](docs/go_project_guide.md)

## ディレクトリ構成

```
cmd/mcpserve/main.go              # エントリーポイント
internal/
    domain/provider.go            # コアドメインインターフェース
    application/                  # アプリケーション層（ファサード、imports）
    infrastructure/               # インフラストラクチャ層（Server実装、レジストリ）
    providers/                    # MCPプロバイダー実装
        monitoring/
            provider.go           # Provider インターフェース実装
            init.go               # init() による自動登録
            domain/               # プロバイダー固有の型・インターフェース
            application/          # UseCase 実装
            infrastructure/       # 外部サービスクライアント
```
