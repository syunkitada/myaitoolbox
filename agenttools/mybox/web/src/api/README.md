# `web/src/api/`

バックエンドのREST APIに接続する型定義、リクエスト処理、エラー処理をまとめます。

## Index

| パス | 役割 |
| --- | --- |
| [`client.ts`](./client.ts) | タスクテンプレート取得、タスク、task triggerの作成・即時実行、ファイル実行ジョブ、Git、Herdr Agentの起動・操作、Codex会話履歴・新規会話・ターン取得・WebSocketイベント・全プロジェクトの稼働中および指定時刻以降の終了セッション取得、予約プロンプト、Stats、内部ブラウザの手動・安全操作自動承認モードと監査ログなどのAPI型と呼び出し。 |
| [`client.test.ts`](./client.test.ts) | APIクライアントのリクエストとエラー処理のテスト。 |
