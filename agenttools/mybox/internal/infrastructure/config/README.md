# `internal/infrastructure/config/`

ユーザー設定とWeb UI状態をXDG設定ディレクトリ配下のYAMLファイルへ保存します。`MYBOX_CONFIG` を設定するとプロジェクト設定の保存先を変更できます。

## Index

| パス | 役割 |
| --- | --- |
| [`state.go`](./state.go) | お気に入り・最近開いたファイルを `state.yaml` に保存する実装。 |
| [`store.go`](./store.go) | プロジェクト一覧と既定プロジェクトを `config.yaml` に保存する実装。 |
| [`store_test.go`](./store_test.go) | 設定の読み書きと親ディレクトリ作成のテスト。 |

