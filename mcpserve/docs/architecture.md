# アーキテクチャ・設計方針

## 概要

`mcpserve` の主な責務は、MCP Server 実装の「ホスティング」と「起動」に特化したシンプルなランタイムを提供することです。
複雑な管理機能は持たせず、単一バイナリで様々なプロバイダ（Monitoringなど）の MCP Server を起動できることを目的としています。

## アーキテクチャ

各 MCP Server のプロバイダは、共通の `Provider` インターフェースを実装します。

```go
package provider

import "github.com/mark3labs/mcp-go/server"

type Provider interface {
	Name() string
	Description() string
	NewServer() *server.MCPServer
}
```

これらの実装は、アプリケーション起動時に内部の Registry に登録され、引数として渡されたサーバ名をもとに Registry から `Provider` を取得し、`NewServer()` を通して `*server.MCPServer` を起動します。通信プロトコルには標準入出力（stdio）を使用します。

## 起動フロー

```text
mcpserve github
        │
        ▼
 Registry Lookup (指定されたサーバを探索)
        │
        ▼
 Provider.NewServer() (MCP Server インスタンスを生成)
        │
        ▼
 server.ServeStdio() (標準入出力を介して MCP Server の起動・通信開始)
```

## ディレクトリ構成

```text
mcpserve/
├── cmd/
│   └── mcpserve/
│       └── main.go           # アプリケーションのエントリーポイント
├── internal/
│   ├── registry/
│   │   └── registry.go       # Provider を管理するレジストリ実装
│   ├── provider/
│   │   └── provider.go       # Provider インターフェースの定義
│   └── servers/
│       ├── monitoring/       # Monitoring プロバイダ実装
├── docs/                     # ドキュメントディレクトリ
├── go.mod                    # 依存関係定義
└── README.md                 # メイン README
```

## 設計原則

- **MCP Server ランタイムとして設計する**: あくまで基盤（ランタイム）であり、内部ロジック自体は各 `servers/` 配下でカプセル化する。
- **シンプルな CLI**: メインコマンドは `mcpserve <server-name>` のみとする。
- **わかりやすいヘルプ**: `-h` オプションで一覧を表示し、`mcpserve <server-name> -h` で固有情報を見れるようにする。
- **保守性と拡張性**: 新しいサーバーは `Provider` インターフェースを実装し、`init()` 関数内で `registry.Register()` を呼び出すことで容易に追加可能とする。
