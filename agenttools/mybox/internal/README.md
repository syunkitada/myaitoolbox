# `internal/`

外部パッケージから直接利用させないアプリケーション本体です。ドメイン、ユースケース、入出力のエントリポイント、ファイルベースの実装、Web UI の埋め込みを分離しています。

## Index

| パス | 役割 |
| --- | --- |
| [`application/README.md`](./application/README.md) | タスク、プロジェクト、ファイル、状態を扱うユースケース。 |
| [`domain/README.md`](./domain/README.md) | エンティティ、エラー、リポジトリのインターフェース。 |
| [`entrypoint/README.md`](./entrypoint/README.md) | CLI、HTTP API、Git、PTY、Herdr、Stats の入出力処理。 |
| [`infrastructure/README.md`](./infrastructure/README.md) | YAML、Markdown、ファイルシステムによる実装。 |
| [`webui/README.md`](./webui/README.md) | Web UI の静的ファイルをGoバイナリへ埋め込む処理。 |

