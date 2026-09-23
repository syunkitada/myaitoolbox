# `mcpserve/internal/modules/monitoring/infrastructure/`

監視モジュールのドメイン契約を、Alertmanager、Prometheus、GrafanaのHTTP APIへ接続する実装です。

## Index

| パス | 役割 |
| --- | --- |
| [`alertmanager.go`](./alertmanager.go) | Alertmanagerのアラート・サイレンス操作。 |
| [`alertmanager_test.go`](./alertmanager_test.go) | Alertmanagerクライアントのテスト。 |
| [`grafana_dashboard.go`](./grafana_dashboard.go) | Grafanaのダッシュボード・フォルダ管理クライアント。 |
| [`grafana_dashboard_test.go`](./grafana_dashboard_test.go) | ダッシュボード管理クライアントのテスト。 |
| [`metric_utils.go`](./metric_utils.go) | メトリクスレスポンスの解析・共通処理。 |
| [`metric_utils_test.go`](./metric_utils_test.go) | メトリクス補助処理のテスト。 |
| [`prometheus.go`](./prometheus.go) | Prometheus/Grafanaのメトリクス取得クライアント。 |
| [`prometheus_test.go`](./prometheus_test.go) | メトリクスクライアントのテスト。 |
