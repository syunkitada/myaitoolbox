# `web/tests/e2e/`

Web UIの主要操作をブラウザ上で検証します。テストは開発用サーバーを起動し、Files、Board、Graph、Git、Herdr、Terminalなどの画面を確認します。

## Index

| パス | 役割 |
| --- | --- |
| [`app.spec.ts`](./app.spec.ts) | アプリケーションのPlaywright E2Eシナリオ。Files画面のHTMLプレビュー、相対アセットの読み込み、sandbox内のスクリプト実行も確認する。 |
| [`browser-fixture.html`](./browser-fixture.html) | 外部ネットワークに依存しないブラウザE2E用の固定HTML。 |
| [`browser.spec.ts`](./browser.spec.ts) | 埋め込みPlaywrightブラウザの開始、観測、MCP stdio経由の操作、手動・安全操作自動承認と高リスク操作の承認維持、再接続、Take over、worker異常終了からの再起動、タイムアウト時のセッション分離、ネットワーク制限、監査ログの表示・セッション/結果フィルタのE2Eシナリオ。 |
| [`server.sh`](./server.sh) | E2E用バックエンドサーバーの起動補助。 |

`server.sh` はタイムアウト分離のE2Eを高速に実行するため、`MYBOX_BROWSER_COMMAND_TIMEOUT=5s`を設定します。本番の既定値は30秒です。

MCPシナリオでは、Playwrightテストがリポジトリ隣接の`mymcpserve`を`go run`でstdio起動し、固定ポートのmyboxへ接続します。そのためMCPシナリオの実行にはGo環境も必要です。

