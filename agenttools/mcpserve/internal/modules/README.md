# `mcpserve/internal/modules/`

個別のMCP機能をモジュールとして配置します。各モジュールはドメイン、アプリケーション、インフラストラクチャのレイヤーに分かれ、共通の [`../domain/README.md`](../domain/README.md) 契約を利用します。

## Index

| パス | 役割 |
| --- | --- |
| [`monitoring/README.md`](./monitoring/README.md) | Alertmanager、Prometheus、Grafanaを扱う監視モジュール。 |
