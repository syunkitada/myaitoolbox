# `internal/domain/`

アプリケーションが扱うデータモデルと、外部実装へ渡す契約を定義します。具体的なYAMLやMarkdownの読み書きは [`../infrastructure/markdown/README.md`](../infrastructure/markdown/README.md) にあります。

## Index

| パス | 役割 |
| --- | --- |
| [`config_store.go`](./config_store.go) | プロジェクト設定を保存・読み込みする契約。 |
| [`errors.go`](./errors.go) | ドメイン共通エラー。 |
| [`file_repository.go`](./file_repository.go) | プロジェクトファイルを操作する契約。 |
| [`project.go`](./project.go) | プロジェクト、設定のモデル。 |
| [`prompt_repository.go`](./prompt_repository.go) | プロンプトテンプレートを描画する契約。 |
| [`state_store.go`](./state_store.go) | UI状態を保存・読み込みする契約。 |
| [`task.go`](./task.go) | タスク、ステータス、優先度、テンプレートデータのモデル。 |
| [`task_repository.go`](./task_repository.go) | タスクを保存・検索・アーカイブする契約。 |
| [`template_renderer.go`](./template_renderer.go) | タスク雛形を描画する契約。 |

