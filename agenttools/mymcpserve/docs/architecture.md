# mymcpserve アーキテクチャ

汎用のレイヤードアーキテクチャ、命名規則、エラーハンドリング、テスト戦略等の詳細は [golangアーキテクチャ](../../../docs/golang/golang_architecture.md) を参照してください。本文では mymcpserve 固有の設計を記述します。

## 概要

`mymcpserve` は、複数の MCP (Model Context Protocol) Server 実装を単一の Go バイナリに内包するランタイムです。各サーバーは `Provider` インターフェースを実装し、CLI コマンドで起動します。

```
mymcpserve monitoring --transport stdio
mymcpserve monitoring --transport http --port 8080
```

## ディレクトリ構成

```
mymcpserve/
├── cmd/mymcpserve/main.go            # エントリーポイント（CLI）
├── internal/
│   ├── domain/provider.go            # コアドメインインターフェース
│   ├── entrypoint/
│   │   ├── registry.go               # プロバイダーレジストリ
│   │   └── bootstrap.go              # DIとサーバー起動
│   ├── infrastructure/
│   │   ├── server.go                 # Server インターフェース実装
│   │   └── response.go               # 共通レスポンスエンベロープ
│   └── modules/
│       └── monitoring/               # monitoring モジュール
│           ├── provider.go           # Provider インターフェース実装
│           ├── domain/               # プロバイダー固有のドメイン型
│           │   ├── alert.go          # Alert, Silence, Matcher 型 + Repository インターフェース
│           │   ├── dashboard.go      # Dashboard, Folder 型 + DashboardRepository インターフェース
│           │   └── metric.go         # MetricSummary 型 + MetricRepository インターフェース
│           ├── application/          # UseCase 実装
│           │   ├── app.go            # ListAlerts, CreateSilence, QueryMetricSummary 等
│           │   ├── dashboard.go      # ListDashboards, CreateDashboard 等
│           │   ├── wrap.go           # ツールハンドラのログラッパー
│           │   └── alert_utils.go    # 時間パース、マッチャー解析、ラベルフォーマット
│           └── infrastructure/       # 外部サービスクライアント
│               ├── alertmanager.go   # Alertmanager HTTP クライアント
│               ├── grafana_dashboard.go # Grafana ダッシュボード管理 HTTP クライアント
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
    Description() string   // プロバイダーの説明
    RegisterTools(server Server) // ツールをサーバーへ登録
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
  └─ rootCmd.Execute()         # cobra による CLI 解析とログ初期化
       │
       └─ runServer()
            ├─ entrypoint.Get(name)  → Provider を取得
            ├─ NewMCServer() + provider.RegisterTools(server)
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

全てのツールは、成功・エラーにかかわらず、以下の共通エンベロープを`StructuredContent`で返します。MCPの`Content`は、成功時は空配列、エラー時は人間向けの短いエラーテキストです。

```json
{
  "data": [ { "...": "..." } ],
  "meta": {
    "schema_version": "1",
    "tool": "list_items",
    "requested_at": "2026-10-12T09:00:00Z",
    "data_status": "unknown",
    "execution_status": "success",
    "warnings": [],
    "count": 10
  },
  "error": null
}
```

- `data`: ツールの実行結果本体（オブジェクトまたは配列）。エラー時は`null`
- `meta`: 標準メタ情報とツール固有のメタ情報。`input`を含める場合は秘密情報を除いた正規化済みの値だけを入れる
- `data_status`: データの鮮度・性質。`live`、`historical`、`reference`、`degraded`、`unknown`などを使用する
- `execution_status`: 処理結果。`success`または`error`
- `error`: 成功時は`null`、エラー時は`code`、`message`、`retryable`を持つオブジェクト

エラー時は`IsError: true`、短いテキスト`Content`、共通の`StructuredContent`を返します。クライアントは成功とエラーで異なるJSON形状を判定する必要がありません。

### 変換処理 (`internal/infrastructure/server.go`)

`serverImpl.AddTool`がハンドラの`(data, meta, err)`を自動で変換します。各モジュールはMCP SDKの`CallToolResult`を直接生成しません。

成功時は`data`と`meta`を包み、`error: null`を設定します。ハンドラが返すメタ情報がオブジェクトでない場合は`meta.details`へ格納します。エラー時は`data: null`とエラーオブジェクトを設定します。標準以外のエラーコードと再試行可否が必要な場合は、`Code() string`と`Retryable() bool`を実装したエラーを返します。

詳細なフィールド定義と利用例は[レスポンスフォーマット](./response-format.md)を参照してください。

## Monitoring プロバイダー

既存の唯一のプロバイダーで、Alertmanager と Prometheus (Grafana経由)、Grafana ダッシュボード管理との連携を提供します。

利用者向けの起動方法、設定、ツール一覧は[monitoringサーバー利用ガイド](./servers/monitoring.md)を参照してください。

### プロバイダーアーキテクチャ

```
modules/monitoring/
  ├── domain/         ← プロバイダー固有の型とリポジトリインターフェース
  ├── application/    ← UseCase 実装
  ├── infrastructure/ ← 外部サービスクライアント
  └── provider.go     ← Provider実装とツール登録
```

### ドメイン (`modules/monitoring/domain/`)

| ファイル | 定義 |
|---------|------|
| `alert.go` | `Alert`, `Silence`, `Matcher` 型、`AlertRepository` / `SilenceRepository` インターフェース |
| `dashboard.go` | `DashboardSummary`, `Dashboard`, `Folder` 型、`DashboardRepository` インターフェース |
| `metric.go` | `MetricSummary`, `OrderedMap` 型、`MetricRepository` インターフェース |

### UseCase (`modules/monitoring/application/app.go`)

| ツール名 | UseCase | 概要 |
|---------|---------|------|
| `list_alerts` | `ListAlerts` | Alertmanager からアラートを取得 |
| `create_silence` | `CreateSilence` | アラートのサイレンスを作成 |
| `list_silences` | `ListSilences` | サイレンス一覧を取得 |
| `delete_silence` | `DeleteSilence` | サイレンスを削除 |
| `query_metric_summary` | `QueryMetricSummary` | PromQL 統計サマリー (min, max, p50/p90/p99) |
| `query_metric_history` | `QueryMetricHistory` | PromQL 時系列データポイント |
| `list_dashboards` | `ListDashboards` | Grafana ダッシュボードの検索 |
| `get_dashboard` | `GetDashboard` | Grafana ダッシュボードの取得（`verbose` でモデル全体を含む） |
| `create_dashboard` | `CreateDashboard` | Grafana ダッシュボードの作成・更新 |
| `delete_dashboard` | `DeleteDashboard` | Grafana ダッシュボードの削除 |
| `list_folders` | `ListFolders` | Grafana フォルダ一覧の取得 |

### Infrastructure (`modules/monitoring/infrastructure/`)

| クライアント | 対象サービス | 実装するリポジトリ |
|------------|-------------|-------------------|
| `alertmanagerClient` | Alertmanager v2 API | `AlertRepository` + `SilenceRepository` |
| `grafanaClient` | Grafana datasource proxy → Prometheus | `MetricRepository` |
| `grafanaDashboardClient` | Grafana dashboard API | `DashboardRepository` |

### ツールラッパー (`WrapTool`)

`modules/monitoring/application/wrap.go` がツールハンドラにログ機能を追加します:

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

- メトリクス取得ツールは `GRAFANA_URL` / `GRAFANA_API_TOKEN` / `GRAFANA_DATASOURCE_UID` の3つが設定されている場合に有効になります。
- ダッシュボード管理ツール（`list_dashboards` 等）は `GRAFANA_URL` と `GRAFANA_API_TOKEN` のみで有効になり、`GRAFANA_DATASOURCE_UID` は不要です。

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
