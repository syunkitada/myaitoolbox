# `mymcpserve/internal/modules/browser/`

myboxのHTTP APIを呼び出し、Playwright browser workerをMCP toolとして公開するproviderです。各toolは`project`と`session_id`を明示的に受け取り、mybox側でもプロジェクトとsessionの一致を検証します。既定ではAIのページ移動やクリックに承認が必要です。ユーザーがmybox UIでセッションの`auto_safe`モードを有効にすると、ページ移動と低リスクと判定されたリンク・メニュー操作は自動実行されます。送信・保存・削除など高リスクの操作は承認対象のままで、モード変更をMCP agentから行うことはできません。

myboxをloopback以外で公開する場合は、`MYBOX_URL`にHTTPS URLを指定し、myboxとmymcpserveの両方に同じ`MYBOX_BROWSER_AGENT_TOKEN`を設定します。remote HTTP URL、認証情報・query・fragmentを含む`MYBOX_URL`は拒否します。未設定時のagent APIはloopback接続に限定されます。

myboxのworkerが異常終了した場合、MCPの操作は失敗し、既存のsessionは`error`状態になります。ユーザーがmybox Web UIのRestart browserを実行して新しいsessionを開始するまで、MCPの`browser_start_session`は再利用せず競合を返します。再起動では既存のプロファイルを保持しますが、ページや操作を自動復元しません。

## Index

| パス | 役割 |
| --- | --- |
| [`provider.go`](./provider.go) | mybox browser APIを呼び出すMCP providerとtool登録。 |
| [`README.md`](./README.md) | 本モジュールの説明。 |
