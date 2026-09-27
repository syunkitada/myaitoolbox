# `scripts/`

リポジトリ全体から実行する運用補助スクリプトを置きます。myboxの再インストール、ポート別の起動・再起動、Tailscale経由の公開、myntfyのtopic確認を扱います。

myboxの起動スクリプト名の末尾は待ち受けポートを表します。通常は `1111` をプライマリとして使い、`1112` や `1113` は障害時にプライマリを復旧するためのセカンダリとして利用できます。セカンダリの起動は必須ではありません。

`show-myntfy-topic.sh`は、myboxでこのリポジトリを開いているときに実行し、保存済みのntfy topicを確認するためのスクリプトです。

## Index

| パス | 役割 |
| --- | --- |
| [`reinstall-mybox.sh`](./reinstall-mybox.sh) | `git pull`、Web UIビルド、mybox再インストール。 |
| [`start-or-restart-mybox1111.sh`](./start-or-restart-mybox1111.sh) | 1111番ポートのプライマリmyboxを起動・再起動。 |
| [`start-or-restart-mybox1112.sh`](./start-or-restart-mybox1112.sh) | 1112番ポートのセカンダリmyboxを起動・再起動。 |
| [`start-or-restart-mybox1113.sh`](./start-or-restart-mybox1113.sh) | 1113番ポートのセカンダリmyboxを起動・再起動。 |
| [`start-tailscale.sh`](./start-tailscale.sh) | Tailscale Serveで1111番ポートを公開。 |
| [`show-myntfy-topic.sh`](./show-myntfy-topic.sh) | 保存済みのmyntfy topicを表示。 |
