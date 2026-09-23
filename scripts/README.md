# `scripts/`

リポジトリ全体から実行する運用補助スクリプトを置きます。myboxの再インストール、ポート別の起動・再起動、Tailscale経由の公開を扱います。

## Index

| パス | 役割 |
| --- | --- |
| [`reinstall-mybox.sh`](./reinstall-mybox.sh) | `git pull`、Web UIビルド、mybox再インストール。 |
| [`start-or-restart-mybox1111.sh`](./start-or-restart-mybox1111.sh) | 1111番ポートのmyboxを起動・再起動。 |
| [`start-or-restart-mybox1112.sh`](./start-or-restart-mybox1112.sh) | 1112番ポートのmyboxを起動・再起動。 |
| [`start-or-restart-mybox1113.sh`](./start-or-restart-mybox1113.sh) | 1113番ポートのmyboxを起動・再起動。 |
| [`start-tailscale.sh`](./start-tailscale.sh) | Tailscale Serveで1111番ポートを公開。 |
