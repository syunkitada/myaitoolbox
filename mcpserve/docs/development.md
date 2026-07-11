# 開発ガイド（サーバーの追加方法）

`mcpserve` への新しい MCP Server の追加は非常に簡単です。`Provider` インターフェースを実装した新しいパッケージを作成し、それを Registry に登録するだけです。レイヤードアーキテクチャの全般的な説明は [go_project_guide.md](go_project_guide.md) を参照してください。

## 1. パッケージの作成

`internal/providers/` 配下に新しいサーバー用のディレクトリ（例: `example`）を作成し、以下の構造を推奨します:

```
internal/providers/example/
    application/
    domain/
    infrastructure/
    provider.go
    init.go
```

## 2. Provider インターフェースの実装

作成したファイル内で、`domain.Provider` インターフェースを満たす構造体を実装します。
`NewServer()` メソッドでは、`infrastructure.NewMCServer()` を使用してサーバーインスタンスを生成し、必要な Tool を登録します。

```go
package example

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/infrastructure"
)

// init() を用いて自動で Registry に登録されるようにします。
func init() {
	domain.Register(New())
}

type exampleProvider struct{}

func New() domain.Provider {
	return &exampleProvider{}
}

func (p *exampleProvider) Name() string {
	return "example" // コマンドライン引数で指定される名前
}

func (p *exampleProvider) Description() string {
	return "Example integration for MCP." // -h オプションで表示される説明
}

func (p *exampleProvider) NewServer() domain.Server {
	s := infrastructure.NewMCServer(
		&mcp.Implementation{Name: "example", Version: "0.0.1"},
		&mcp.ServerOptions{},
	)

	// ここに Tool を追加します
	s.AddTool(&mcp.Tool{
		Name:        "example_tool",
		Description: "An example tool",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		// データを返します。metaはメタ情報（例: フィルタ条件、件数など）
		return "Result from example_tool", nil, nil
	})

	return s
}
```

## 3. エントリーポイントへの登録

作成したパッケージの `init()` 関数が実行されるよう、`internal/application/imports.go` の import に追加します（アンダースコア `_` を用いた blank import）。

```go
package application

import (
	// Register providers
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring"
	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/example" // <--- 追加
)
```

## 4. 動作確認

ビルドして正しく追加されているか確認します。

```bash
go run cmd/mcpserve/main.go -h
```

`Available servers:` の一覧に `example` が表示されれば追加成功です。

## 5. テスト

テストファイルを作成し、動作を確認します。

```bash
go test ./internal/providers/example/...
```

## 注意事項

- **レスポンスフォーマット**: 全てのツールは成功時に `structuredContent` を返すこと。形式は `{"structuredContent": {"meta": {...}, "data": {...}}}` です。
- **エラーハンドリング**: エラー時は `IsError: true` を設定し `structuredContent` は省略すること。
- **ヘルパー関数**: `newStructuredResult(text, meta, data)` を使用すると、レスポンスの構築が簡単になります。
- **ドキュメント更新**: 機能追加・変更時には、対応するREADME.md、docs/* 内のファイルを参照し、必要に応じて更新すること。
