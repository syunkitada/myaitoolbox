# `internal/entrypoint/api/`

[`../../../openapi.yaml`](../../../openapi.yaml) を入力として生成されるHTTP APIの型とサーバーインターフェースです。API仕様を変更した場合は `make generate` で再生成します。生成ファイルを直接編集しないでください。

## Index

| パス | 役割 |
| --- | --- |
| [`server.gen.go`](./server.gen.go) | OpenAPI定義から生成されたEchoサーバーの型・ルーティング補助。 |
| [`types.gen.go`](./types.gen.go) | OpenAPI定義から生成されたリクエスト・レスポンス型。 |
