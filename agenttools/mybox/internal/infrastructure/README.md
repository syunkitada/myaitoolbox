# `internal/infrastructure/`

ドメインのインターフェースを、ローカルファイルと標準ライブラリで実装する層です。

## Index

| パス | 役割 |
| --- | --- |
| [`automation/`](./automation/) | `_task_triggers` の定義読み込みと自動化実行台帳。 |
| [`config/README.md`](./config/README.md) | `config.yaml`、`state.yaml`、`scheduled-prompts.yaml` の永続化。 |
| [`fsutil/README.md`](./fsutil/README.md) | アトミックなファイル書き込みなどの低レベル補助処理。 |
| [`markdown/README.md`](./markdown/README.md) | Markdown、フロントマター、タスク、テンプレートのリポジトリ実装。 |
