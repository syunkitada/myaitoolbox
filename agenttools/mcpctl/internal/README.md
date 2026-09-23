# `mcpctl/internal/`

`mcpctl`の内部実装です。アプリケーション層、ドメイン層、CLIのエントリポイント、MCPクライアント接続やプロファイル管理のインフラ層に分かれています。

## Index

| パス | 役割 |
| --- | --- |
| [`application/README.md`](./application/README.md) | ツール検索・情報取得・実行とプロファイル操作のユースケース。 |
| [`domain/README.md`](./domain/README.md) | ツール、プロファイル、各層間のインターフェース。 |
| [`entrypoint/README.md`](./entrypoint/README.md) | Cobra CLIのコマンドとシェル補完。 |
| [`infrastructure/README.md`](./infrastructure/README.md) | MCPクライアント、MCPサーバ、プロファイル永続化。 |
