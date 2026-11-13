# myherdr 設計

## CLI と session

`myherdr start` は Git root の directory 名から Herdr の64文字上限内で `codex-<slug>` を作り、`herdr --session <name>` を直接起動します。Git repository 以外では指定 directory または現在の directory を使います。`HERDR_ENV=1` の pane 内では Herdr を入れ子で起動しません。Herdr が pane/session のライフサイクルを所有します。

## ローカル Web terminal

`myherdr serve` は `127.0.0.1` と空き port で起動し、user runtime directory に PID/port の一時情報を `0600` で保存します。単一 serve lock を取得し、終了時に state file を削除します。flock、fcntl、Windows file lock が使える OS では異常終了後も lock が解放されます。JS/WASM、Plan 9、WASI では stale lock を残さず、serve を未対応エラーで終了します。`myherdr url` は loopback health endpoint に接続してから URL を表示します。

ブラウザーは pane 一覧 API から既存 pane を選び、Herdr の `terminal session control` または読み取り専用 `observe` CLI stream に接続します。frame の base64 ANSI bytes を xterm.js で描画し、terminal input と resize を JSON lines で Herdr へ返します。controller ごとに 1 接続だけをローカルで許可し、Herdr 側の制御競合も takeover せずにエラーとして扱います。pane や session の自動作成・削除は行いません。

## リモート session

`.myherdr/sessions.toml` の URL alias から host と Herdr session を解決します。host は `.myherdr/ssh-hosts.md` の明示的な一覧に存在しなければなりません。SSH は batch mode で起動し、対話認証が必要な場合にブラウザー接続を続けません。ローカルの Herdr CLI は実行ファイルと引数を分けて起動します。SSH の remote command に使う host、session、pane ID は形式を検証し、shell のメタ文字を含む値は拒否します。SSH 接続情報は保存しません。

## 信頼境界

- HTTP listener は IPv4 loopback に固定し、Host と WebSocket/HTTP Origin を serve origin と照合します。CORS は有効にしません。
- session 名と pane ID を形式検証し、Herdr binary と argument を分けて起動します。
- Herdr terminal frame は untrusted terminal data として xterm.js に渡し、DOM HTML として描画しません。
- request URL、端末内容、入力内容をログへ書きません。HTTP server の access/error log を無効化しています。
- URL を知る同一マシン上の利用者は terminal input 権限を得られるため、単一ユーザーのローカル開発環境を対象にします。
