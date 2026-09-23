# `internal/entrypoint/`

外部との境界を実装するパッケージです。CLIとHTTPサーバーを組み立て、アプリケーション層のユースケースをGit、PTY、Herdr、ホスト統計と接続します。

## Index

| パス | 役割 |
| --- | --- |
| [`api/README.md`](./api/README.md) | `openapi.yaml` から生成されたAPI型・サーバーコード。 |
| [`bootstrap.go`](./bootstrap.go) | 設定・リポジトリ・ユースケースを組み立てる初期化処理。 |
| [`cli.go`](./cli.go) | CobraベースのCLIコマンドとオプション。 |
| [`cli_test.go`](./cli_test.go) | CLIコマンドのテスト。 |
| [`git.go`](./git.go) | Git状態、remote同期差分、fetch、差分、ブランチ、ステージ、コミットの処理。 |
| [`git_test.go`](./git_test.go) | Git操作のHTTPテスト。 |
| [`herdr.go`](./herdr.go) | Herdrワークスペース、タブ、ペイン、エージェントの連携。 |
| [`herdr_test.go`](./herdr_test.go) | Herdr連携のテスト。 |
| [`osc.go`](./osc.go) | PTY出力に含まれるOSCシーケンスの処理。 |
| [`osc_test.go`](./osc_test.go) | OSC処理のテスト。 |
| [`server.go`](./server.go) | Echo HTTPサーバー、ファイルAPI、WebSocketの登録。 |
| [`server_test.go`](./server_test.go) | HTTP APIとファイル操作のテスト。 |
| [`stats.go`](./stats.go) | CPU、メモリ、ディスク、ネットワーク、プロセス統計の取得。 |
| [`stats_test.go`](./stats_test.go) | Stats処理のテスト。 |
| [`terminal.go`](./terminal.go) | PTYシェルとWebSocketターミナルの管理。 |
| [`terminal_test.go`](./terminal_test.go) | ターミナル処理のテスト。 |
| [`vtmode.go`](./vtmode.go) | VT端末モードの追跡と再接続時の再適用。 |
| [`vtmode_test.go`](./vtmode_test.go) | VTモード処理のテスト。 |
