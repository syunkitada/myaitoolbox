# `cmd/myntfy/`

`myntfy`コマンドの起動処理とCobraサブコマンド定義を担当します。

## Index

| パス | 役割 |
| --- | --- |
| [`main.go`](./main.go) | `topic`、`send`、`subscribe`などのCLIを構成して起動するエントリポイント。 |
| [`main_test.go`](./main_test.go) | topic管理、送信、購読のCLIテスト。 |
