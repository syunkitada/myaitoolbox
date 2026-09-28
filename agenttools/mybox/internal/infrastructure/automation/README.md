# `internal/infrastructure/automation/`

プロジェクト直下の `_task_triggers/` を読み込み、cron・ファイル作成イベント・manual実行をmyboxのタスク実行へ渡すファイルベース実装です。実行台帳とプロジェクトロックは、プロジェクト外のmybox設定ディレクトリに保存します。

## Index

| パス | 役割 |
| --- | --- |
| [`definitions.go`](./definitions.go) | `trigger.yaml` の読み書き、パス検証、設定の既定値適用。 |
| [`definitions_test.go`](./definitions_test.go) | トリガー定義の読み書きと入力検証のテスト。 |
| [`scheduler.go`](./scheduler.go) | cron時刻計算、ファイルスキャン、fsnotifyによるファイル作成監視。 |
| [`scheduler_test.go`](./scheduler_test.go) | cronとファイルイベントのテスト。 |
| [`state.go`](./state.go) | 自動化実行台帳とプロジェクト単位の排他ロック。 |
| [`state_test.go`](./state_test.go) | 実行台帳の読み書きテスト。 |
