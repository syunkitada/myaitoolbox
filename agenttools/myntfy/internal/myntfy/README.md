# `internal/myntfy/`

保存済みtopicを管理し、ntfyの送信API・JSON購読APIへ接続する内部パッケージです。

## Index

| パス | 役割 |
| --- | --- |
| [`client.go`](./client.go) | ntfyサーバーURLの解決、通知送信、JSONストリーム購読。 |
| [`client_test.go`](./client_test.go) | HTTPクライアントのテスト。外部サービスには接続しない。 |
| [`topic.go`](./topic.go) | 必須prefix付きtopicの生成、prefix置換、形式検証、0600での原子的保存。 |
| [`topic_test.go`](./topic_test.go) | topicストアと形式検証のテスト。 |
