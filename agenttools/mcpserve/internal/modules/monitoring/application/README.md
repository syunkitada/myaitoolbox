# `mcpserve/internal/modules/monitoring/application/`

監視モジュールのユースケースを実装し、外部サービスの戻り値をMCPのツール応答へ変換します。

## Index

| パス | 役割 |
| --- | --- |
| [`alert_utils.go`](./alert_utils.go) | アラートのフィルタ・整形などの補助処理。 |
| [`alert_utils_test.go`](./alert_utils_test.go) | アラート補助処理のテスト。 |
| [`app.go`](./app.go) | Alertmanager、メトリクス取得のアプリケーション処理。 |
| [`app_test.go`](./app_test.go) | アプリケーション処理のテスト。 |
| [`dashboard.go`](./dashboard.go) | ダッシュボード・フォルダ管理のアプリケーション処理。 |
| [`dashboard_test.go`](./dashboard_test.go) | ダッシュボード管理処理のテスト。 |
| [`wrap.go`](./wrap.go) | MCPリクエストからユースケースを呼び出すラッパー。 |
| [`wrap_test.go`](./wrap_test.go) | MCPラッパーのテスト。 |
