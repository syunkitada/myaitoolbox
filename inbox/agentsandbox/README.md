# agentsandbox

## Index

| パス | 役割 |
| --- | --- |
| [`dockerfiles/`](./dockerfiles/) | Agent Sandbox用のDockerイメージ定義。 |
| [`nginx/`](./nginx/) | Sandbox内のリバースプロキシ設定。 |
| [`oauth2-proxy/`](./oauth2-proxy/) | OAuth2 Proxyの設定例。 |
| [`.env.example`](./.env.example) | Sandbox起動時の環境変数の例。 |
| [`.gitignore`](./.gitignore) | ローカル環境ファイルの除外設定。 |
| [`docker-compose.yml`](./docker-compose.yml) | Sandboxコンテナの構成。 |
| [`README.md`](./README.md) | Sandboxの起動手順。本文書。 |

```
$ sudo docker compose up -d
$ sudo docker exec -it agent01 bash

$ opencode web --hostname 0.0.0.0 --port 8000
```
