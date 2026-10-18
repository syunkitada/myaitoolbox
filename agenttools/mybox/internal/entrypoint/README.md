# `internal/entrypoint/`

外部との境界を実装するパッケージです。CLIとHTTPサーバーを組み立て、アプリケーション層のユースケースをGit、PTY、Herdr、ホスト統計と接続します。

## Index

| パス | 役割 |
| --- | --- |
| [`api/README.md`](./api/README.md) | `openapi.yaml` から生成されたAPI型・サーバーコード。 |
| [`automation.go`](./automation.go) | 自動タスクトリガーのCLI、manual実行、デーモン、Herdrディスパッチ接続。 |
| [`browser_audit.go`](./browser_audit.go) | 内部ブラウザの秘匿値を含まない監査ログのプロジェクトスコープ付き検索API。 |
| [`browser.go`](./browser.go) | Playwright browser workerの起動、プロジェクト単位セッション、worker同時数上限、アイドルタイムアウト、制御権、ユーザー専用のセッション承認モード、秘匿値を除いた監査イベント、プロファイル、最新映像フレームの保持とWebSocket再接続時の再送、worker異常終了時のエラー保持と明示的再起動。 |
| [`bootstrap.go`](./bootstrap.go) | 設定・リポジトリ・ユースケースを組み立てる初期化処理。 |
| [`cli.go`](./cli.go) | CobraベースのCLIコマンドとオプション。 |
| [`cli_test.go`](./cli_test.go) | CLIコマンドのテスト。 |
| [`codex.go`](./codex.go) | Codex CLIの読み取り専用診断、設定メタデータ、Config・AGENTS.md・Skills本文、Skillディレクトリ一覧、MCP設定の収集。本文取得時のパス検証、Config・AGENTS.md・Skillのシンボリックリンク対応、上限、Config秘匿値マスクも担当する。 |
| [`codex_test.go`](./codex_test.go) | Codex診断のCLI出力解析、本文取得、Skillディレクトリ一覧、パス検証、秘匿値マスク、部分エラーのHTTPテスト。 |
| [`file_execute.go`](./file_execute.go) | Filesタブからの実行可能ファイル実行とWebSocket出力ストリーム。 |
| [`file_execute_test.go`](./file_execute_test.go) | Filesタブの実行ストリームの統合テスト。 |
| [`file_execution.go`](./file_execution.go) | ブラウザ切断後も継続するサーバー管理のファイル実行ジョブ、履歴、再接続、停止API。 |
| [`git.go`](./git.go) | Git状態、remote同期差分、fetch、差分、ブランチ、ステージ、コミットの処理。 |
| [`git_test.go`](./git_test.go) | Git操作のHTTPテスト。 |
| [`herdr.go`](./herdr.go) | Herdrワークスペース、タブ、ペイン、エージェントの連携。 |
| [`herdr_test.go`](./herdr_test.go) | Herdr連携のテスト。 |
| [`prompt_scheduler.go`](./prompt_scheduler.go) | 永続化されたHerdr予約プロンプトを指定時刻に送信するサーバーワーカー。 |
| [`prompt_scheduler_test.go`](./prompt_scheduler_test.go) | 予約ワーカーの送信、未到来、リトライのテスト。 |
| [`osc.go`](./osc.go) | PTY出力に含まれるOSCシーケンスの処理。 |
| [`osc_test.go`](./osc_test.go) | OSC処理のテスト。 |
| [`server.go`](./server.go) | Echo HTTPサーバー、ファイルAPI、taskテンプレート取得、task trigger作成・実行API、WebSocketの登録。 |
| [`server_test.go`](./server_test.go) | HTTP APIとファイル操作のテスト。 |
| [`stats.go`](./stats.go) | CPU、メモリ、ディスク、ネットワーク、プロセス統計の取得。 |
| [`stats_test.go`](./stats_test.go) | Stats処理のテスト。 |
| [`terminal.go`](./terminal.go) | PTYシェルとWebSocketターミナルの管理。 |
| [`terminal_test.go`](./terminal_test.go) | ターミナル処理のテスト。 |
| [`vtmode.go`](./vtmode.go) | VT端末モードの追跡と再接続時の再適用。 |
| [`vtmode_test.go`](./vtmode_test.go) | VTモード処理のテスト。 |
