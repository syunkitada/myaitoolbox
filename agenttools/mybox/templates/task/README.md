# `templates/task/`

新規タスクの本文とフロントマターを生成する組み込み雛形です。プロジェクト側の `templates/task/task.md` があればそちらが優先されます。新規作成時には、雛形に残る `priority`、`agent_kind`、`assignee`、`description` も除去されます。

## Index

| パス | 役割 |
| --- | --- |
| [`task.yaml`](./task.yaml) | `title`、ステータス、概要などを含む既定タスク雛形。 |

