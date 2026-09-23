# `internal/mygit/`

`mygit`のドメインモデルと処理本体です。YAMLの読み書き、Workspace探索、clone先検証、Gitのrevision解決、`sync`・`update`・`status`を実装します。

テストは一時ディレクトリとローカルbare repositoryを使い、外部remoteへの書き込みを行いません。

## Index

| パス | 役割 |
| --- | --- |
| [`discovery.go`](./discovery.go) | Workspaceの再帰探索とclone先のprune。 |
| [`git.go`](./git.go) | Gitコマンド実行とrevision解決。 |
| [`model.go`](./model.go) | manifest、lockfile、clone先のモデルと検証。 |
| [`service.go`](./service.go) | `sync`、`update`、`status`、lockfile・`.gitignore`更新。 |
| [`discovery_test.go`](./discovery_test.go) | Workspace探索のテスト。 |
| [`model_test.go`](./model_test.go) | manifest・lockfile・clone先検証のテスト。 |
| [`service_test.go`](./service_test.go) | ローカルGit repositoryを使う同期統合テスト。 |
