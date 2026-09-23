# `cmd/mybox/`

`mybox` コマンドの起動処理だけを担当します。CLI の各サブコマンドやHTTPサーバーの実装は [`../../internal/entrypoint/README.md`](../../internal/entrypoint/README.md) を参照してください。

## Index

| パス | 役割 |
| --- | --- |
| [`main.go`](./main.go) | `entrypoint.Execute` を呼び出してCLIを起動する `main` パッケージ。 |
