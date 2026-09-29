# `web/src/api/`

バックエンドのREST APIに接続する型定義、リクエスト処理、エラー処理をまとめます。

## Index

| パス | 役割 |
| --- | --- |
| [`client.ts`](./client.ts) | タスクテンプレート取得、タスク、task triggerの作成・即時実行、ファイル、Git、Herdr、予約プロンプト、StatsなどのAPI型と呼び出し。 |
| [`client.test.ts`](./client.test.ts) | APIクライアントのリクエストとエラー処理のテスト。 |
