# Configuration

`mcpctl` は `~/.config/mcpctl/` ディレクトリ配下に設定ファイルやプロファイルを格納します。

## ディレクトリ構成

```text
~/.config/mcpctl/
├── config.yaml          # 全体設定
├── profiles/            # 個別のプロファイル設定
│   ├── dev.yaml
│   ├── stg.yaml
│   └── prod.yaml
└── cache/               # キャッシュデータ (予約)
    ├── dev.json
    └── prod.json
```

## config.yaml

全体的なデフォルト挙動を設定します。

```yaml
# デフォルトで使用するプロファイル名
default_profile: dev

cache:
  enabled: true
  ttl: 10m

output:
  format: table
```

## profiles/<profile_name>.yaml

各環境（プロファイル）ごとの MCP サーバー定義を記述します。

```yaml
name: dev

servers:
  # サーバの名前（任意の識別子）
  github:
    transport: stdio
    command: github-mcp # 実行するコマンド

  slack:
    transport: stdio
    command: slack-mcp

  aws:
    transport: streamable-http
    url: https://aws.example.com/mcp
```

### Transport Types

- `stdio`: ローカルのコマンドを起動し、標準入出力経由で通信します（`command` 必須）
- `streamable-http`: HTTP 経由でのストリーミング通信を利用します（`url` 必須）
- `sse`: Server-Sent Events を用いた通信を利用します（`url` 必須）

## プロファイルの解決順序

実行時に使用するプロファイルは以下の優先順位で決定されます：

1. CLIの `--profile` (`-p`) フラグ、またはMCPリクエスト引数としての指定
2. `config.yaml` の `default_profile` に設定されたプロファイル
