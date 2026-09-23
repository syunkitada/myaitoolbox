# `internal/infrastructure/markdown/`

プロジェクトをMarkdownファイルとして扱うリポジトリ実装です。パス検証、タスクの新旧レイアウト、YAMLフロントマター、テンプレートのフォールバックをここで処理します。

## Index

| パス | 役割 |
| --- | --- |
| [`file_repository.go`](./file_repository.go) | プロジェクト内ファイルの一覧、内容取得、変更、コピー、実行。 |
| [`file_repository_test.go`](./file_repository_test.go) | ファイル操作、パス境界、タグ、実行のテスト。 |
| [`frontmatter.go`](./frontmatter.go) | Markdownフロントマターの分離、YAML解析、再構成。 |
| [`frontmatter_test.go`](./frontmatter_test.go) | フロントマターとタスク雛形のテスト。 |
| [`prompt_repository.go`](./prompt_repository.go) | プロジェクト・既定プロジェクト・組み込みプロンプトの探索と変数展開。 |
| [`prompt_repository_test.go`](./prompt_repository_test.go) | プロンプトの上書き、フォールバック、変数展開のテスト。 |
| [`task_repository.go`](./task_repository.go) | `tasks/<id>/task.md` と旧レイアウトのタスク保存、検索、アーカイブ。 |
| [`task_repository_test.go`](./task_repository_test.go) | タスクのCRUD、アーカイブ、新旧レイアウトのテスト。 |
| [`template.go`](./template.go) | タスク雛形の探索、Go template描画、YAML値のクォート。 |

