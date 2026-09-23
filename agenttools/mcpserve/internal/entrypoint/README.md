# `mcpserve/internal/entrypoint/`

利用可能なプロバイダーをレジストリへ登録し、指定されたプロバイダーとトランスポートでサーバーを起動します。

## Index

| パス | 役割 |
| --- | --- |
| [`bootstrap.go`](./bootstrap.go) | プロバイダー登録とstdio/HTTPサーバーの起動。 |
| [`registry.go`](./registry.go) | Providerの登録、検索、一覧取得。 |
