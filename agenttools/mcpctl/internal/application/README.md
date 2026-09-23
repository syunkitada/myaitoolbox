# `mcpctl/internal/application/`

ドメインの契約を組み合わせ、CLIとMCPサーバから利用するユースケースを実装します。

## Index

| パス | 役割 |
| --- | --- |
| [`call.go`](./call.go) | MCPツールの実行と出力形式の検証・整形。 |
| [`call_test.go`](./call_test.go) | ツール実行引数と出力形式のテスト。 |
| [`info.go`](./info.go) | ツール情報取得のユースケース。 |
| [`list.go`](./list.go) | 利用可能なツールの一覧取得。 |
| [`profiles.go`](./profiles.go) | プロファイル一覧、現在値表示、切り替え。 |
| [`search.go`](./search.go) | ツール検索のユースケース。 |
