# `templates/`

アプリケーションへ埋め込む組み込みテンプレートです。実行時はプロジェクト側の上書きファイルが優先され、ここは既定値として使われます。

タスク雛形は `templates/task/task.yaml`、プロンプトは `templates/prompts/*.yaml` です。プロジェクト側ではタスク雛形を `templates/task/task.md`、プロンプトを `prompts/<name>.md` に置きます。

## Index

| パス | 役割 |
| --- | --- |
| [`prompts/README.md`](./prompts/README.md) | 組み込みプロンプトテンプレート。 |
| [`task/README.md`](./task/README.md) | 組み込みタスク雛形。 |
| [`embed.go`](./embed.go) | `task` と `prompts` をembedする定義。 |

