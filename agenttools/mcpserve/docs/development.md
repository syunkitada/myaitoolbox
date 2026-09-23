# 開発ガイド（サーバーの追加方法）

`mcpserve` への新しい MCP Server の追加は非常に簡単です。`Provider` インターフェースを実装した新しいパッケージを作成し、それを Registry に登録するだけです。レイヤードアーキテクチャの全般的な説明は [golangアーキテクチャ](../../../docs/golang/golang_architecture.md) を参照してください。

## 1. パッケージの作成

`internal/modules/` 配下に新しいサーバー用のディレクトリ（例: `example`）を作成し、以下の構造を推奨します:

```
internal/modules/example/
    application/
    domain/
    infrastructure/
    provider.go
```

## 2. Provider インターフェースの実装

作成したファイル内で、`domain.Provider` インターフェースを満たす構造体を実装します。
`RegisterTools(server)` メソッドで、受け取ったサーバーに必要なToolを登録します。サーバーの生成と起動は `internal/entrypoint` が担当します。

```go
package example

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
)

type exampleProvider struct{}

func New() domain.Provider {
	return &exampleProvider{}
}

func (p *exampleProvider) Name() string {
	return "example" // コマンドライン引数で指定される名前
}

func (p *exampleProvider) Description() string {
    return "Example integration for MCP."
}

func (p *exampleProvider) RegisterTools(s domain.Server) {
    s.AddTool(&mcp.Tool{
		Name:        "example_tool",
		Description: "An example tool",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		// データを返します。metaはメタ情報（例: フィルタ条件、件数など）
		return "Result from example_tool", nil, nil
	})

}
```

## 3. エントリーポイントへの登録

作成したパッケージを `internal/entrypoint/bootstrap.go` からimportし、`NewRegistryWithProviders` 内で明示的に登録します。

```go
import (
    "github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring"
    "github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/example"
)

func NewRegistryWithProviders() *Registry {
    registry := NewRegistry()
    registry.Register(monitoring.New())
    registry.Register(example.New())
    return registry
}
```

## 4. 動作確認

ビルドして正しく追加されているか確認します。

```bash
go run ./cmd/mcpserve -h
```

MCPクライアントから `mcpserve example` に接続し、ツール一覧に `example_tool` が表示されれば追加成功です。

## 5. テスト

テストファイルを作成し、動作を確認します。

```bash
go test ./internal/modules/example/...
```

## サーバードキュメント

サーバーを追加・変更した場合は、利用者向けドキュメントも更新します。

- `docs/servers/<server-name>.md` にサーバーの起動方法、環境変数、提供ツール一覧、各ツールの入力パラメータを記載する
- `docs/servers/README.md` のIndexにサーバードキュメントを追加する
- `RegisterTools`で登録しているツール名・説明・入力スキーマとドキュメントの一覧を一致させる

## 6. 品質チェック

実装後は、テストと静的解析を実行して確認します。

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
go install ./cmd/mcpserve
```

`golangci-lint` の設定は [`.golangci.yml`](../.golangci.yml) で管理し、バージョンを固定して導入しています。未インストールまたはバージョン更新時は、次のコマンドでインストールしてください。

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

## 注意事項

- **レスポンスフォーマット**: 全てのツールは成功時に `structuredContent` を返すこと。形式は `{"structuredContent": {"meta": {...}, "data": {...}}}` です。
- **エラーハンドリング**: エラー時は `IsError: true` を設定し `structuredContent` は省略すること。
- **ドキュメント更新**: 機能追加・変更時には、対応するREADME.md、docs/* 内のファイルを参照し、必要に応じて更新すること。
