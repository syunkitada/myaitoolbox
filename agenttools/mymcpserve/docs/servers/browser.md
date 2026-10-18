# `browser` server

`browser`は、起動中のmyboxへHTTPで接続し、プロジェクト単位のPlaywright browser sessionをMCP toolとして公開します。myboxのWeb UIと同じsessionを共有でき、ユーザーがいつでもTake overできます。

## 起動

```bash
MYBOX_URL=http://127.0.0.1:8080 mymcpserve browser --transport stdio
```

`MYBOX_URL`を省略すると`http://127.0.0.1:8080`へ接続します。mybox側でbrowser workerを利用するには、`agenttools/mybox/browser-worker`で`npm install`と`npx playwright install chromium`を先に実行してください。mybox HTTP APIには組み込みのユーザー認証がないため、外部UIはTLS・ユーザー認証を終端する同一ホスト上のリバースプロキシから、`Host`を保持してloopbackへ転送してください。プロキシはWebSocket Upgradeを転送し、ユーザー向け経路の`X-Browser-Actor`と`X-Browser-Agent-Token`を削除します。myboxをloopback以外からagent接続する場合は、`MYBOX_URL`にHTTPS URLを指定し、myboxとmymcpserveの両方に同じ`MYBOX_BROWSER_AGENT_TOKEN`を設定してください。remote HTTP URL、認証情報・query・fragmentを含む`MYBOX_URL`は拒否されます。agent tokenはユーザー認証の代替ではありません。

## ツール

| ツール | 必須パラメータ | 概要 |
| --- | --- | --- |
| `browser_start_session` | `project` | 空ページでプロジェクトのbrowser sessionを開始または再利用する。ページ移動は`browser_navigate`でユーザー承認を求める。 |
| `browser_attach_session` | `project`, `session_id` | 指定sessionがプロジェクトの唯一のsessionであることを確認する。 |
| `browser_observe` | `project`, `session_id` | アクセシビリティスナップショットとsemantic reference idを取得する。 |
| `browser_navigate` | `project`, `session_id`, `url` | public HTTPSへの移動を要求する。通常は`approval_required`になり、ユーザーがmybox UIでセッションの安全操作自動承認を有効にした場合は移動する。 |
| `browser_click` | `project`, `session_id`, `observation_id`, `ref` | semantic referenceをクリックする。通常は承認対象だが、自動承認モードでは低リスクと判定されたリンク・メニュー・開閉操作に限り承認なしで実行する。 |
| `browser_fill` | `project`, `session_id`, `observation_id`, `ref`, `value` | 非機密フィールドへ入力する。password、OTP、カード情報、token等の機密性が高いフィールドはユーザー操作が必要。 |
| `browser_press` | `project`, `session_id`, `observation_id`, `ref`, `key` | semantic referenceへキーを送る。 |
| `browser_scroll` | `project`, `session_id`, `delta_y` | ページをスクロールする。 |
| `browser_wait` | `project`, `session_id` | ページが落ち着くまで短時間待つ。 |
| `browser_screenshot` | `project`, `session_id` | 現在ページのJPEGを取得する。 |
| `browser_close_session` | `project`, `session_id` | sessionを閉じる。永続profileは保持する。 |

すべての操作toolは`session_id`を明示的に要求します。観測後にページが変わるとreferenceは無効になり、再度`browser_observe`が必要です。AIは座標操作や任意JavaScriptを使用しません。

workerが異常終了すると、MCP toolはエラーを返し、sessionは`error`状態になります。ユーザーがmybox Web UIのRestart browserを実行すると、新しいworkerとsessionが起動します。既存のprofileは保持されますが、ページや操作の自動復元は行いません。

## 安全境界

- v1のトップレベル遷移とリソースはpublic HTTPSのみです。HTTP、localhost、プライベート・link-local・metadata networkはworker側で拒否します。Service Workerは無効化し、WebSocket接続も同じ検査を通します。
- ダウンロード・アップロードは初期版では無効です。
- パスワード、OTP、カード情報、token等の機密性が高いフィールドはAIから入力できず、ユーザー操作が必要です。
- 既定ではAI主体の遷移とリンク・ボタン操作をmyboxが保留し、ユーザーがWeb UIでApproveした1回だけ実行します。ユーザーがWeb UIからセッション単位の`auto_safe`を有効にすると、AIのページ移動と低リスクと判定されたリンク・メニュー・開閉操作は自動実行されます。送信・保存・削除・購入など高リスクと判定された操作は引き続き承認が必要で、機密フィールドへの入力はユーザー専用です。ページラベルの分類は完全ではなく、Webサイトが予期しない副作用を起こす可能性があります。MCP側から承認モードを変更することはできず、ユーザーはいつでもmybox UIで制御を引き取れます。
- browser workerは127.0.0.1だけで待ち受け、myboxがランダムBearer tokenで接続します。
