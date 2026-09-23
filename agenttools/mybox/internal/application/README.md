# `internal/application/`

ドメインのインターフェースを組み合わせて、CLIとHTTP APIから共通利用するユースケースを実装します。永続化形式や通信方式には依存しません。

## Index

| パス | 役割 |
| --- | --- |
| [`file.go`](./file.go) | ファイル・ディレクトリの一覧、読み書き、移動、コピー、削除、実行。 |
| [`helpers.go`](./helpers.go) | アプリケーション層で共有する補助処理。 |
| [`project.go`](./project.go) | プロジェクトの登録、一覧、既定値、削除。 |
| [`project_test.go`](./project_test.go) | プロジェクトユースケースのテスト。 |
| [`state.go`](./state.go) | お気に入り・最近使ったファイルなどの状態管理。 |
| [`task.go`](./task.go) | タスクの作成、一覧、表示、更新、アーカイブ、エージェント起動。 |
| [`task_test.go`](./task_test.go) | タスクユースケースのテスト。 |

