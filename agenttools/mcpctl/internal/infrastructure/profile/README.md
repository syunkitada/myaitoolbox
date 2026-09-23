# `mcpctl/internal/infrastructure/profile/`

`~/.config/mcpctl/`の設定とプロファイルを扱い、CLIの指定・MCPリクエスト・既定値から利用プロファイルを解決します。

## Index

| パス | 役割 |
| --- | --- |
| [`loader.go`](./loader.go) | 設定とプロファイルの読み書き、設定ディレクトリの解決。 |
| [`resolver.go`](./resolver.go) | CLI指定、MCP指定、既定値からプロファイルを解決。 |
| [`validator.go`](./validator.go) | プロファイルと接続設定の検証。 |
| [`validator_test.go`](./validator_test.go) | 設定読み込みと検証のテスト。 |
