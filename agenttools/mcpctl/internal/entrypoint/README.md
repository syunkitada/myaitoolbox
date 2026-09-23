# `mcpctl/internal/entrypoint/`

Cobraを使ったCLIのコマンド、共通オプション、シェル補完を定義します。処理本体は [`../application/README.md`](../application/README.md)、接続実装は [`../infrastructure/README.md`](../infrastructure/README.md) にあります。

## Index

| パス | 役割 |
| --- | --- |
| [`call.go`](./call.go) | MCPツールを呼び出す`call`コマンド。 |
| [`completehelper.go`](./completehelper.go) | シェル補完用のツール一覧取得。 |
| [`completion.go`](./completion.go) | zsh補完スクリプトの生成。 |
| [`info.go`](./info.go) | ツール詳細を表示する`info`コマンド。 |
| [`list.go`](./list.go) | ツール一覧を表示する`list`コマンド。 |
| [`profiles.go`](./profiles.go) | プロファイルを管理する`profiles`コマンド。 |
| [`root.go`](./root.go) | ルートコマンドと共通`--profile`オプション。 |
| [`search.go`](./search.go) | ツールを検索する`search`コマンド。 |
