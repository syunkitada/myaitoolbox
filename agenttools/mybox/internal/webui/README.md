# `internal/webui/`

ビルド済みWeb UIをGoのembed機能でバイナリに同梱するためのパッケージです。`make web-build` が [`dist/`](./dist/) を更新してから、サーバーがこのパッケージを配信します。

## Index

| パス | 役割 |
| --- | --- |
| [`dist/`](./dist/) | `web/` のビルド成果物を置くディレクトリ。`.gitkeep` とこのREADMEのみを管理し、実体は生成物。 |
| [`embed.go`](./embed.go) | `dist` の静的ファイルをembedする定義。 |
