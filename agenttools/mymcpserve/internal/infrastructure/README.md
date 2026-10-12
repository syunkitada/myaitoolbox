# `mymcpserve/internal/infrastructure/`

MCP SDKのサーバーをドメインの`Server`インターフェースとして提供し、ツールの成功・エラー応答を共通形式へ変換します。

## Index

| パス | 役割 |
| --- | --- |
| [`server.go`](./server.go) | MCPサーバの生成、ツール登録、共通レスポンスへの変換。 |
| [`response.go`](./response.go) | `{data, meta, error}`エンベロープ、出力スキーマ、エラー情報の整形。 |
| [`server_test.go`](./server_test.go) | サーバー生成とツール呼び出しのテスト。 |
