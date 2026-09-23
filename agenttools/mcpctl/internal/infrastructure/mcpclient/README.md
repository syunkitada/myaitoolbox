# `mcpctl/internal/infrastructure/mcpclient/`

設定されたMCPサーバへ接続し、ツールを発見・実行して結果をCLI向けに整形します。

## Index

| パス | 役割 |
| --- | --- |
| [`client.go`](./client.go) | 接続設定に応じたMCPクライアントセッションの生成。 |
| [`discovery.go`](./discovery.go) | 複数サーバーからのツール一覧・詳細・検索。 |
| [`executor.go`](./executor.go) | 指定したサーバーのツール呼び出し。 |
| [`output.go`](./output.go) | JSON値の順序保持と表形式・TSV出力。 |
| [`output_test.go`](./output_test.go) | JSONデコードと出力処理のテスト。 |
