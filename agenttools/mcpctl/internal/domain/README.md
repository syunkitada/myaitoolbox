# `mcpctl/internal/domain/`

`mcpctl`が扱うツール、サーバープロファイル、設定と、アプリケーション・インフラ間の契約を定義します。

## Index

| パス | 役割 |
| --- | --- |
| [`discovery.go`](./discovery.go) | ツール検索・実行・プロファイル解決のインターフェースとツール名解析。 |
| [`discovery_test.go`](./discovery_test.go) | ツール名解析のテスト。 |
| [`profile.go`](./profile.go) | 設定、プロファイル、MCPサーバー接続設定の型。 |
