# `cmd/mygit/`

`mygit` CLIのエントリポイントです。Cobraでサブコマンドを構成し、実装本体は`internal/mygit`にあります。

利用方法は[`../../docs/usage.md`](../../docs/usage.md)を参照してください。

## Index

| パス | 役割 |
| --- | --- |
| [`main.go`](./main.go) | `sync`、`update`、`status`を受け付けるCLIエントリポイント。 |
| [`main_test.go`](./main_test.go) | CLI引数検証のテスト。 |

```bash
go run ./cmd/mygit --help
go run ./cmd/mygit sync
go run ./cmd/mygit update
go run ./cmd/mygit status
```
