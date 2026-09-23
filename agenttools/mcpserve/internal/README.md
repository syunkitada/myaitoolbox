# `mcpserve/internal/`

`mcpserve`の外部公開しない実装です。MCPの契約、起動とプロバイダー登録、SDKを包むサーバー実装、機能モジュールに分かれています。

## Index

| パス | 役割 |
| --- | --- |
| [`domain/README.md`](./domain/README.md) | ProviderとServerのコアインターフェース。 |
| [`entrypoint/README.md`](./entrypoint/README.md) | プロバイダー登録とサーバー起動の組み立て。 |
| [`infrastructure/README.md`](./infrastructure/README.md) | MCP SDKを使った共通サーバー実装。 |
| [`modules/README.md`](./modules/README.md) | 個別MCP機能モジュール。 |
