# `browser-worker/src/`

Playwrightを起動するNode.js worker本体です。Go側からloopback WebSocketでコマンドを受け、semantic observation、参照IDによる操作、ユーザー入力、CDP JPEG screencastを提供します。セッション開始時とAI操作完了後にもJPEGを明示送信し、静止画面やUI再接続時にも最新状態を表示できるようにします。セッション承認モードは手動確認が既定で、ユーザーが`auto_safe`へ切り替えた場合のみAIのページ移動と低リスクと判定したリンク・メニュー操作を自動実行します。送信等の高リスク操作と機密フィールド入力は引き続き自動化しません。

## Index

| パス | 役割 |
| --- | --- |
| [`index.mjs`](./index.mjs) | workerのHTTP health endpoint、認証付きWebSocket、Playwright session、操作と映像配信。 |
| [`README.md`](./README.md) | 本ディレクトリの説明。 |
