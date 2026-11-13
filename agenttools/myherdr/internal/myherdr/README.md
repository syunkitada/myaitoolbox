# `internal/myherdr/`

CLI、プロジェクトと session 名の解決、Herdr CLI の呼び出し、SSH 設定検証、端末 stream と localhost Web アプリを実装します。

## Index

| パス | 役割 |
| --- | --- |
| [`README.md`](./README.md) | 実装 package の入口。 |
| [`command.go`](./command.go) | Cobra command と `start`、`session-name`、`serve`、`url` の dispatch。 |
| [`herdr.go`](./herdr.go) | Herdr session の pane 列挙と Herdr 環境の構築。 |
| [`project.go`](./project.go) | Git root、project 名、既定 session 名の解決。 |
| [`project_test.go`](./project_test.go) | project root、session 名、URL の unit test。 |
| [`remote.go`](./remote.go) | TOML session 設定、許可 SSH host、非対話 SSH 経由の Herdr 呼び出し。 |
| [`remote_test_unix_test.go`](./remote_test_unix_test.go) | fake SSH を使う remote host allowlist と pane 列挙の test。 |
| [`serve.go`](./serve.go) | loopback HTTP/WebSocket server、runtime 状態、host/origin 検査。 |
| [`serve_lock_other.go`](./serve_lock_other.go) | JS/WASM、Plan 9、WASI で serve 非対応を返す。 |
| [`serve_lock_fcntl.go`](./serve_lock_fcntl.go) | AIX/Solaris の fcntl record lock とプロセス内 mutex による単一 serve 制御。 |
| [`serve_lock_unix.go`](./serve_lock_unix.go) | Unix flock による単一 serve instance 制御。 |
| [`serve_lock_windows.go`](./serve_lock_windows.go) | Windows file lock による単一 serve instance 制御。 |
| [`serve_lock_test.go`](./serve_lock_test.go) | owner process 終了後も serve lock を再取得できることを確認。 |
| [`serve_lock_other_test.go`](./serve_lock_other_test.go) | lock 非対応環境で lock file を作らず serve 非対応を返すことを確認。 |
| [`serve_test_unix_test.go`](./serve_test_unix_test.go) | fake Herdr を使う HTTP、WebSocket、端末 bridge の test。 |
| [`session.go`](./session.go) | session/pane ID 検証と URL 生成。 |
| [`terminal_bridge.go`](./terminal_bridge.go) | Herdr `control` / `observe` stream と WebSocket の中継。 |
| [`command_test_unix_test.go`](./command_test_unix_test.go) | fake Herdr を使う CLI 起動引数と nested launch 防止の test。 |
| [`test_helpers_unix_test.go`](./test_helpers_unix_test.go) | fake executable を使う Unix test helper。 |
| [`web/`](./web/) | Go binary に埋め込むブラウザー端末。詳細は [`web/README.md`](./web/README.md) を参照。 |
