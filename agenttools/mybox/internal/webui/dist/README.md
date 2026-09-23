# `internal/webui/dist/`

`web/`の本番ビルド成果物を置くディレクトリです。`make web-build`で生成され、Goの`embed`機能によってバイナリに同梱されます。成果物本体はGit管理対象外で、ディレクトリ維持用の`.gitkeep`と本文書だけを管理します。

## Index

| パス | 役割 |
| --- | --- |
| [`.gitkeep`](./.gitkeep) | 生成物がない状態でもディレクトリを保持するファイル。 |
