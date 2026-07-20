# mcpserve アーキテクチャ

汎用のレイヤードアーキテクチャ、命名規則、エラーハンドリング、テスト戦略等の詳細は [golangアーキテクチャ](../../../docs/golang/golang_architecture.md) を参照してください。本文では mcpserve 固有の設計を記述します。

## 概要

`mcpserve` は、複数の MCP (Model Context Protocol) Server 実装を単一の Go バイナリに内包するランタイムです。各サーバーは `Provider` インターフェースを実装し、CLI コマンドで起動します。

```
mcpserve monitoring --transport stdio
mcpserve monitoring --transport http --port 8080
```

## ディレクトリ構成

```
mcpserve/
├── cmd/mcpserve/main.go              # エントリーポイント（CLI）
├── internal/
│   ├── domain/provider.go            # コアドメインインターフェース
│   ├── application/
│   │   ├── registry.go               # プロバイダーファサード
│   │   └── imports.go                # blank import による登録トリガー
│   ├── infrastructure/
│   │   ├── server.go                 # Server インターフェース実装
│   │   └── registry.go               # グローバルプロバイダーレジストリ
│   └── providers/
│       └── monitoring/               # monitoring プロバイダー
│           ├── provider.go           # Provider インターフェース実装
│           ├── init.go               # init() による自動登録
│           ├── domain/               # プロバイダー固有のドメイン型
│           │   ├── alert.go          # Alert, Silence, Matcher 型 + Repository インターフェース
│           │   └── metric.go         # MetricSummary 型 + MetricRepository インターフェース
│           ├── application/          # UseCase 実装
│           │   ├── app.go            # ListAlerts, CreateSilence, QueryMetricSummary 等
│           │   ├── wrap.go           # ツールハンドラのログラッパー
│           │   └── alert_utils.go    # 時間パース、マッチャー解析、ラベルフォーマット
│           └── infrastructure/       # 外部サービスクライアント
│               ├── alertmanager.go   # Alertmanager HTTP クライアント
│               ├── prometheus.go     # Grafana datasource proxy クライアント
│               └── metric_utils.go   # PromQL ユーティリティ
├── docs/                             # ドキュメント
├── go.mod
└── README.md
```

## コアインターフェース

### Provider (`internal/domain/provider.go`)

全ての MCP Server はこのインターフェースを実装します。

```go
type Provider interface {
    Name() string          // CLI で指定するサーバー名
    Description() string   // -h で表示される説明
    NewServer() Server     // サーバーインスタンスの生成
}
```

### Server (`internal/domain/provider.go`)

MCP サーバーの標準化されたラッパーです。

```go
type Server interface {
    AddTool(tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error))
    Run(ctx context.Context, transport mcp.Transport) error
    MCP() *mcp.Server
}
```

- `AddTool`: ツールハンドラを登録。ハンドラは `(data, meta, err)` を返し、`StructuredContent` への変換は自動で行われる
- `Run`: 指定されたトランスポートでサーバーを起動
- `MCP`: HTTP トランスポートで `mcp.NewSSEHandler` に渡すための内部サーバーを返す

## サーバー起動フロー

```
main.go
  │
  ├─ godotenv.Load()           # .env ファイル読み込み
  ├─ initLogger()              # slog の初期化
  └─ rootCmd.Execute()         # cobra による CLI 解析
       │
       └─ runServer()
            ├─ application.Get(name)  → Provider を取得
            ├─ provider.NewServer()   → Server を生成（Tool 登録含む）
            │
            ├─ [stdio] srv.Run(ctx, &mcp.StdioTransport{})
            └─ [http]  mcp.NewSSEHandler(...) + http.ListenAndServe
```

### トランスポート

| トランスポート | 説明 | 起動方法 |
|--------------|------|---------|
| `stdio` | 標準入出力経由（デフォルト） | `srv.Run(ctx, &mcp.StdioTransport{})` |
| `http` | SSE (Server-Sent Events) 経由 | `mcp.NewSSEHandler` + `http.ListenAndServe` |

## レスポンスフォーマット

全てのツールは成功時に以下の `StructuredContent` を返します。

```json
{
  "structuredContent": {
    "meta": { "count": 10, "query": "...", "from": "...", "to": "..." },
    "data": [ { ... }, { ... } ]
  }
}
```

- `meta`: リクエストパラメータ、件数、フィルタ条件などのメタ情報
- `data`: ツールの実行結果本体（オブジェクトまたは配列）

エラー時は `IsError: true` を設定し `StructuredContent` は省略します。

### 変換処理 (`internal/infrastructure/server.go`)

`serverImpl.AddTool` がハンドラの `(data, meta, err)` を自動で変換します:

```go
// 成功時
&mcp.CallToolResult{
    Content: []mcp.Content{&mcp.TextContent{Text: jsonData}},
    StructuredContent: map[string]interface{}{
        "meta": meta,
        "data": data,
    },
}

// エラー時
&mcp.CallToolResult{
    IsError: true,
    Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
}
```

## Monitoring プロバイダー

既存の唯一のプロバイダーで、Alertmanager と Prometheus (Grafana経由) との連携を提供します。

### プロバイダーアーキテクチャ

```
providers/monitoring/
  ├── domain/         ← プロバイダー固有の型とリポジトリインターフェース
  ├── application/    ← UseCase 実装
  ├── infrastructure/ ← 外部サービスクライアント
  ├── provider.go     ← Provider インターフェース実装
  └── init.go         ← init() による登録
```

### ドメイン (`providers/monitoring/domain/`)

| ファイル | 定義 |
|---------|------|
| `alert.go` | `Alert`, `Silence`, `Matcher` 型、`AlertRepository` / `SilenceRepository` インターフェース |
| `metric.go` | `MetricSummary`, `OrderedMap` 型、`MetricRepository` インターフェース |

### UseCase (`providers/monitoring/application/app.go`)

| ツール名 | UseCase | 概要 |
|---------|---------|------|
| `list_alerts` | `ListAlerts` | Alertmanager からアラートを取得 |
| `create_silence` | `CreateSilence` | アラートのサイレンスを作成 |
| `list_silences` | `ListSilences` | サイレンス一覧を取得 |
| `delete_silence` | `DeleteSilence` | サイレンスを削除 |
| `query_metric_summary` | `QueryMetricSummary` | PromQL 統計サマリー (min, max, p50/p90/p99) |
| `query_metric_history` | `QueryMetricHistory` | PromQL 時系列データポイント |

### Infrastructure (`providers/monitoring/infrastructure/`)

| クライアント | 対象サービス | 実装するリポジトリ |
|------------|-------------|-------------------|
| `alertmanagerClient` | Alertmanager v2 API | `AlertRepository` + `SilenceRepository` |
| `grafanaClient` | Grafana datasource proxy → Prometheus | `MetricRepository` |

### ツールラッパー (`WrapTool`)

`providers/monitoring/application/wrap.go` がツールハンドラにログ機能を追加します:

```
ツール呼び出し → slog.Info(tool, params) → ハンドラ実行 → slog.Info/Error(tool)
```

## 設定

### 環境変数 (.env)

```env
ALERTMANAGER_URL=http://127.0.0.1:9093
GRAFANA_URL=http://localhost:3000
GRAFANA_API_TOKEN=your-token
GRAFANA_DATASOURCE_UID=your-uid
```

### CLI フラグ

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--transport` | `stdio` | `stdio` または `http` |
| `--host` | `localhost` | HTTP トランスポート時のリッスンアドレス |
| `--port` | `8080` | HTTP トランスポート時のポート |
| `--log-level` | `info` | `debug` / `info` / `warn` / `error` |

## 依存関係

| パッケージ | 用途 |
|-----------|------|
| `github.com/modelcontextprotocol/go-sdk` | MCP SDK（サーバー、トランスポート、型定義） |
| `github.com/spf13/cobra` | CLI フレームワーク |
| `github.com/joho/godotenv` | .env ファイル読み込み |
