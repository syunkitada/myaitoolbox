# `mcpserve/internal/infrastructure/`

MCP SDKのサーバーをドメインの`Server`インターフェースとして提供し、ツールの成功・エラー応答を共通形式へ変換します。

## Index

| パス | 役割 |
| --- | --- |
| [`server.go`](./server.go) | MCPサーバの生成、ツール登録、`meta`・`data`形式の応答整形。 |
| [`server_test.go`](./server_test.go) | サーバー生成とツール呼び出しのテスト。 |
