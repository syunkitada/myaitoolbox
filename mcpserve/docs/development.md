# 開発ガイド（サーバーの追加方法）

`mcpserve` への新しい MCP Server の追加は非常に簡単です。`Provider` インターフェースを実装した新しいパッケージを作成し、それを Registry に登録するだけです。

## 1. パッケージの作成

`internal/servers/` 配下に新しいサーバー用のディレクトリ（例: `example`）を作成し、ファイル（例: `example.go`）を追加します。

## 2. Provider インターフェースの実装

作成したファイル内で、`provider.Provider` インターフェースを満たす構造体を実装します。
`NewServer()` メソッド内で `*server.MCPServer` インスタンスを生成し、必要な Tool や Resource を登録します。

```go
package example

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/provider"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/registry"
)

// init() を用いて自動で Registry に登録されるようにします。
func init() {
	registry.Register(New())
}

type exampleProvider struct{}

func New() provider.Provider {
	return &exampleProvider{}
}

func (p *exampleProvider) Name() string {
	return "example" // コマンドライン引数で指定される名前
}

func (p *exampleProvider) Description() string {
	return "Example integration for MCP." // -h オプションで表示される説明
}

func (p *exampleProvider) NewServer() *server.MCPServer {
	s := server.NewMCPServer("example", "0.0.1")

	// ここに Tool などを追加します
	s.AddTool(mcp.Tool{
		Name:        "example_tool",
		Description: "An example tool",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Result from example_tool"), nil
	})

	return s
}
```

## 3. エントリーポイントへの登録

作成したパッケージの `init()` 関数が実行されるよう、`cmd/mcpserve/main.go` の import に追加します（アンダースコア `_` を用いた blank import）。

```go
package main

import (
	// ...既存のインポート...
	
	// Register providers
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/aws"
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/github"
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/jira"
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/example" // <--- 追加
)
// ...
```

## 4. 動作確認

ビルドして正しく追加されているか確認します。

```bash
go run cmd/mcpserve/main.go -h
```

`Available servers:` の一覧に `example` が表示されれば追加成功です。
