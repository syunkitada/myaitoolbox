# `mcpserve/internal/modules/monitoring/`

Alertmanagerのアラート・サイレンス操作と、Prometheus/Grafanaのメトリクス取得、Grafanaのダッシュボード管理をMCPツールとして公開するモジュールです。

利用者向けの起動方法、設定、提供ツールの一覧と入力パラメータは、[`docs/servers/monitoring.md`](../../../docs/servers/monitoring.md)を参照してください。

`list_alerts` の `status` はAlertmanagerの状態をそのまま文字列比較せず、`active`、`silenced`、`inhibited` のいずれかで絞り込みます。抑制アラートでは `silenced_by` と `inhibited_by` に根拠となるIDを返します。`list_silences` はサイレンスの `status` と、演算子を保持した構造化 `matchers` を返します。

ダッシュボード系ツール（`list_dashboards`、`get_dashboard`、`create_dashboard`、`delete_dashboard`、`list_folders`）は登録されていますが、利用には `GRAFANA_URL` と `GRAFANA_API_TOKEN` が必要です。ダッシュボード操作に `GRAFANA_DATASOURCE_UID` は不要です。`get_dashboard` の `verbose` は取得結果にダッシュボードの完全なモデルを含めるかどうかを切り替えます。

## Index

| パス | 役割 |
| --- | --- |
| [`application/README.md`](./application/README.md) | 監視データのユースケースとMCP向けのラップ処理。 |
| [`domain/README.md`](./domain/README.md) | アラート、メトリクス、リポジトリの契約。 |
| [`infrastructure/README.md`](./infrastructure/README.md) | Alertmanager、Prometheus、Grafanaのクライアント実装。 |
| [`provider.go`](./provider.go) | 監視ツールを登録するProvider実装。 |
| [`provider_test.go`](./provider_test.go) | Providerの登録と構成のテスト。 |
