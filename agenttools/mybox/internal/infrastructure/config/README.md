# `internal/infrastructure/config/`

ユーザー設定、Web UI状態、自動化の実行状態をXDG設定ディレクトリ配下のYAMLファイルへ保存します。`MYBOX_CONFIG` を設定するとプロジェクト設定と自動化状態の保存先を変更できます。

## Index

| パス | 役割 |
| --- | --- |
| [`state.go`](./state.go) | お気に入り・最近開いたファイルを `state.yaml` に保存する実装。 |
| [`state_test.go`](./state_test.go) | 旧形式のお気に入りstateの読み込みとプロジェクト付き保存のテスト。 |
| [`scheduled_prompts.go`](./scheduled_prompts.go) | Herdr予約プロンプトを `scheduled-prompts.yaml` に永続化する実装。 |
| [`scheduled_prompts_test.go`](./scheduled_prompts_test.go) | 予約プロンプトの永続化、プロジェクト絞り込み、削除のテスト。 |
| [`store.go`](./store.go) | プロジェクト一覧と既定プロジェクトを `config.yaml` に保存し、自動化状態ディレクトリを解決する実装。 |
| [`store_test.go`](./store_test.go) | 設定の読み書きと親ディレクトリ作成のテスト。 |
