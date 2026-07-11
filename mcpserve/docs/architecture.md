# アーキテクチャ・設計方針

## レイヤードアーキテクチャ

```
Application
      │
      ▼
   Domain
      ▲
      │
Infrastructure
```

- **Domain**: ビジネスルールのみを持つ。他のレイヤを参照してはいけない。
- **Application**: UseCase を実装し、Domain のみを利用する。
- **Infrastructure**: Domain の Interface を実装する。
- **Providers**: 各MCPプロバイダーの実装。

## ディレクトリ構成

```
internal/
    application/    # アプリケーション層（UseCase）
    domain/         # ドメイン層（ビジネスルール、インターフェース）
    infrastructure/ # インフラストラクチャ層（実装）
    providers/      # MCPプロバイダー実装
        <provider>/
            application/
            domain/
            infrastructure/
```

### 例

```
internal/
    domain/
        provider.go       # Providerインターフェース
        registry.go       # プロバイダーレジストリ
    infrastructure/
        server.go         # Server実装
    providers/
        monitoring/
            application/
                usecase1.go
                ...
            domain/
                database1_repository.go
                externalservice1_client.go
                ...
            infrastructure/
                database1/
                    repository.go
                externalservice1/
                    client.go
                ...
```

## プロバイダーインターフェース

全てのプロバイダーは `provider.Provider` インターフェースを実装する必要がある:

```go
type Provider interface {
    Name() string
    Description() string
    NewServer() Server
}
```

## サーバーインターフェース

`provider.Server` インターフェースは、MCPサーバーの標準化されたレスポンスフォーマットを提供する:

```go
type Server interface {
    AddTool(tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error))
    Run(ctx context.Context, transport mcp.Transport) error
    MCP() *mcp.Server
}
```

## レスポンスフォーマット

全てのツールは成功時に `structuredContent` を返すこと。形式は以下の通り:

```json
{
  "structuredContent": {
    "meta": { /* クエリパラメータ、件数、メタ情報 */ },
    "data": { /* または配列 */ }
  }
}
```

- `meta`: リクエストパラメータ、件数、フィルタ条件などのメタ情報
- `data`: ツールの実行結果本体（オブジェクトまたは配列）

エラー時は `IsError: true` を設定し `structuredContent` は省略すること。

ヘルパー: `newStructuredResult(text, meta, data)` を使用すること。

## レジストリ

プロバイダーは `registry.Register()` を用いて登録される。起動時に `registry.Get()` でプロバイダーを取得し、サーバーを起動する。
