# `monitoring` server

`monitoring`は、Alertmanagerのアラート・サイレンス操作、Grafana経由のPrometheusメトリクス取得、Grafanaダッシュボード管理を提供するMCP Serverです。

## 起動

```bash
mcpserve monitoring --transport stdio
```

HTTPで起動する場合は次のようにします。

```bash
mcpserve monitoring --transport http --host localhost --port 8080
```

## 設定

| 環境変数 | 必須 | 説明 |
| --- | --- | --- |
| `ALERTMANAGER_URL` | いいえ | AlertmanagerのURL。省略時は`http://127.0.0.1:9093`。 |
| `GRAFANA_URL` | ダッシュボード・メトリクス利用時 | GrafanaのURL。 |
| `GRAFANA_API_TOKEN` | ダッシュボード・メトリクス利用時 | Grafana APIトークン。 |
| `GRAFANA_DATASOURCE_UID` | メトリクス利用時 | Grafana内のPrometheusデータソースUID。 |

11個のツールは常にMCPのツール一覧に登録されます。Alertmanagerは設定を省略すると既定のURLへ接続し、Grafanaのダッシュボード・メトリクスツールは必要な環境変数が不足していると利用できません。依存サービスに接続できない場合はエラーになります。

## ツール一覧

| ツール | 対象 | 必須パラメータ | 概要 |
| --- | --- | --- | --- |
| [`list_alerts`](#list_alerts) | Alertmanager | なし | アラートを取得する。 |
| [`create_silence`](#create_silence) | Alertmanager | `endat`, `matchers`, `comment`, `created_by` | サイレンスを作成する。 |
| [`list_silences`](#list_silences) | Alertmanager | なし | サイレンスを取得する。 |
| [`delete_silence`](#delete_silence) | Alertmanager | `id` | サイレンスを削除する。 |
| [`query_metric_summary`](#query_metric_summary) | Prometheus | `query` | PromQLの統計サマリーを取得する。 |
| [`query_metric_history`](#query_metric_history) | Prometheus | `query` | PromQLの時系列データを取得する。 |
| [`list_dashboards`](#list_dashboards) | Grafana | なし | ダッシュボードを検索する。 |
| [`get_dashboard`](#get_dashboard) | Grafana | `uid` | ダッシュボードを取得する。 |
| [`create_dashboard`](#create_dashboard) | Grafana | `title`または`dashboard.title` | ダッシュボードを作成または更新する。 |
| [`delete_dashboard`](#delete_dashboard) | Grafana | `uid` | ダッシュボードを削除する。 |
| [`list_folders`](#list_folders) | Grafana | なし | ダッシュボードフォルダを取得する。 |

成功時の応答は、共通形式の`structuredContent`（`meta`と`data`）で返ります。

## Alertmanagerツール

### `list_alerts`

Alertmanagerからアラートを取得します。`status`を指定すると、`active`、`silenced`、`inhibited`のいずれかで絞り込めます。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `status` | string | いいえ | `active`、`silenced`、`inhibited`。省略時はすべて。 |
| `alertname` | string | いいえ | アラート名で絞り込む。 |
| `host` | string | いいえ | ホストで絞り込む。 |
| `verbose` | boolean | いいえ | `true`で全ラベルを返す。デフォルトは`false`。 |

### `create_silence`

Alertmanagerにサイレンスを作成します。`matchers`はカンマ区切りの`key=value`形式です。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `startat` | string | いいえ | 開始時刻。RFC3339または相対時刻（例：`+1h`）。デフォルトは現在時刻。 |
| `endat` | string | はい | 終了時刻。RFC3339または相対時刻（例：`+2h`）。 |
| `matchers` | string | はい | 例：`alertname="HighCPUUsage",host="server1"`。 |
| `comment` | string | はい | サイレンスのコメント。 |
| `created_by` | string | はい | 作成者。 |

### `list_silences`

Alertmanagerからサイレンスを取得します。結果にはサイレンスの`status`と構造化された`matchers`が含まれます。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `alertname` | string | いいえ | アラート名で絞り込む。 |
| `host` | string | いいえ | ホストで絞り込む。 |
| `verbose` | boolean | いいえ | `true`でコメント、作成者、開始・終了時刻を含める。デフォルトは`false`。 |

### `delete_silence`

Alertmanagerのサイレンスを削除します。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `id` | string | はい | 削除するサイレンスID。 |

## Prometheusメトリクスツール

### `query_metric_summary`

PromQLを実行し、系列ごとのサンプル数、最小値、P50、P90、P99、最大値、最終値を取得します。結果にはGrafana Explore URLも含まれます。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `query` | string | はい | 実行するPromQL。 |
| `var` | array of string | いいえ | クエリ変数。例：`["host=server1", "env=prod"]`。 |
| `legend` | string | いいえ | ラベルを使った凡例テンプレート。例：`{{host}}`。 |
| `sort` | string | いいえ | `legend`、`samples`、`min`、`p50`、`p90`、`p99`、`max`、`last`。デフォルトは`p99`。 |
| `reverse` | boolean | いいえ | `true`で並び順を反転する。 |
| `limit` | integer | いいえ | 返す件数。デフォルトは`100`。 |
| `offset` | integer | いいえ | 先頭からスキップする件数。デフォルトは`0`。 |
| `time_from` | string | いいえ | RFC3339または相対時刻。例：`now-1h`、`1h`、`1d`。デフォルトは`now-1h`。 |
| `time_to` | string | いいえ | RFC3339または相対時刻。例：`now`、`5m`。デフォルトは`now`。 |

### `query_metric_history`

1つまたは複数のPromQLを実行し、時刻で整列したデータポイントを取得します。`query`には文字列の配列を指定します。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `query` | array of string | はい | 実行するPromQLまたはPromQL群。 |
| `var` | array of string | いいえ | クエリ変数。例：`["host=server1", "env=prod"]`。 |
| `legend` | string | いいえ | ラベルを使った凡例テンプレート。例：`{{host}}`。 |
| `time_from` | string | いいえ | RFC3339または相対時刻。デフォルトは`now-1h`。 |
| `time_to` | string | いいえ | RFC3339または相対時刻。デフォルトは`now`。 |

## Grafanaダッシュボードツール

### `list_dashboards`

Grafanaのダッシュボードをタイトル、フォルダ、タグで検索します。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `query` | string | いいえ | ダッシュボードタイトルに対する検索文字列。 |
| `folder_uid` | string | いいえ | 対象フォルダUID。 |
| `tag` | array of string | いいえ | 絞り込むタグ。 |
| `limit` | integer | いいえ | 最大件数。デフォルトは`50`。 |

### `get_dashboard`

GrafanaのダッシュボードをUIDで取得します。デフォルトでは概要を返し、`verbose`を有効にすると完全なダッシュボードモデルを返します。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `uid` | string | はい | 取得するダッシュボードUID。 |
| `verbose` | boolean | いいえ | 完全なモデルを返す。デフォルトは`false`。 |

### `create_dashboard`

Grafanaのダッシュボードを作成または更新します。`dashboard`を省略した場合は、`title`から最小限のモデルを作成します。`title`と`dashboard.title`の両方を指定した場合は、`title`が優先されます。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `title` | string | 条件付き | ダッシュボードタイトル。`dashboard.title`がない場合に必要。 |
| `dashboard` | object | 条件付き | 完全なダッシュボードモデルJSON。`title`またはモデル内の`title`が必要。 |
| `folder_uid` | string | いいえ | 保存先フォルダUID。省略時はGeneralフォルダ。 |
| `tags` | array of string | いいえ | 設定するタグ。 |
| `overwrite` | boolean | いいえ | 同じUIDの既存ダッシュボードを上書きする。デフォルトは`false`。 |
| `message` | string | いいえ | 新しいバージョンに保存する変更メッセージ。 |

### `delete_dashboard`

GrafanaのダッシュボードをUIDで削除します。

| パラメータ | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `uid` | string | はい | 削除するダッシュボードUID。 |

### `list_folders`

Grafanaのダッシュボードフォルダを一覧表示します。入力パラメータはありません。
