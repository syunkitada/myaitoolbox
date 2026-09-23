# mcpctl 仕様書

## 概要

`mcpctl` は MCP (Model Context Protocol) サーバー群を統合的に操作する CLI です。

## ディレクトリ構成

```
~/.config/mcpctl/
├── config.yaml              # 全体設定
├── profiles/                # プロファイル定義
│   ├── dev.yaml
│   └── prod.yaml
└── cache/                   # キャッシュ（予約、現時点では未使用）
```

## コマンド一覧

| コマンド | 引数 | 説明 |
|----------|------|------|
| `list` | `[server]` | ツール一覧表示（サーバー指定でフィルタ） |
| `search` | `<query>` | ツールをキーワード検索 |
| `info` | `<server/tool>` | ツールの詳細情報（パラメータスキーマ）を表示 |
| `call` | `<server/tool> [flags]` | ツールを実行 |
| `profiles` | | プロファイル一覧表示 |
| `profiles current` | | 現在のデフォルトプロファイル表示 |
| `profiles use` | `<name>` | デフォルトプロファイルを変更 |
| `completion` | `zsh` | zsh補完スクリプト生成 |

### 隠しコマンド（シェル補完用）

| コマンド | 説明 |
|----------|------|
| `__list_tools` | 全ツール一覧を `server/tool:description` 形式で出力 |
| `__list_params <server/tool>` | ツールのパラメータ一覧を `--param:desc (required)` 形式で出力 |
| `__list_param_values <server/tool> <param>` | enum パラメータの候補値を出力 |

## フラグ

### グローバルフラグ

| フラグ | 短縮 | 型 | デフォルト | 説明 |
|--------|------|----|-----------|------|
| `--profile` | `-p` | string | `""` | 使用するプロファイル名 |

### call コマンドのフラグ

`call` は `DisableFlagParsing: true` のため、フラグは手動パースされる。

| フラグ | 短縮 | 型 | デフォルト | 説明 |
|--------|------|----|-----------|------|
| `--profile` | `-p` | string | `""` | プロファイル名 |
| `--output` | `-o` | string | `config.yaml` の `output.format`（未指定時 `table`） | 出力形式（raw / tsv / table） |
| `--params` | | string | | パラメータJSON（インライン`{...}`またはファイルパス） |
| `--<paramName>` | `-<s>` | varies | | ツールパラメータ（値なしで boolean true） |

**Human Shortcut:** 最後の引数に `-l` または `-h` を指定すると、実行モードではなく一覧/情報表示モードになる。

## 出力形式（call -o）

| 形式 | 説明 |
|------|------|
| `raw` | レスポンス全体を `json.MarshalIndent` で整形して出力 |
| `tsv` | JSONをパースしタブ区切りで出力 |
| `table` | アラインメントされたテーブル形式で出力 |

### 出力処理フロー

```
res (*mcp.CallToolResult)
├── raw → res 全体を JSON 出力
├── tsv/table + StructuredContent あり
│   ├── StructuredContent を printTSV/printTable で出力（stdout）
│   └── meta.outputs に対応するキーを出力（stderr）
└── 上記以外
    └── Content[] をループ
        ├── TextContent → JSONパース → printTSV/printTable（JSONでなければそのまま出力）
        ├── ImageContent → "[Image <mime>]" と出力
        └── その他 → JSON 出力
```

### StructuredContent のメタ出力

`StructuredContent` に `meta.outputs` 配列が含まれる場合、TSV/テーブル出力の後に標準エラーに空行を挟んで以下を出力する:

```
key: <JSON value>
```

`outputs` に指定されたキーが `meta` に存在しない場合は警告を出力する:

```
Warning: key "foo" specified in outputs not found in meta
```

## 設定

### config.yaml

```yaml
default_profile: dev

cache:
  enabled: true
  ttl: 10m

output:
  format: table
```

### profiles/<name>.yaml

```yaml
name: dev

servers:
  github:
    transport: stdio
    command: npx
    args: ["@anthropic/github-mcp-server"]
    env: ["GITHUB_TOKEN=..."]
  weather:
    transport: streamable-http
    url: https://api.example.com/mcp
  logs:
    transport: sse
    url: https://logs.example.com/mcp/sse
```

### Transport 種類

| Transport | 必須フィールド | 説明 |
|-----------|---------------|------|
| `stdio` | `command` | ローカルコマンドの標準入出力で通信 |
| `streamable-http` | `url` | HTTP ストリーミング通信 |
| `sse` | `url` | Server-Sent Events 通信 |

### プロファイル解決順序

1. CLI の `--profile` / `-p` フラグ
2. `config.yaml` の `default_profile`

## アーキテクチャ

### パッケージ構成

```
cmd/mcpctl/main.go          → エントリーポイント（シグナルとCLI実行）
internal/
├── application/            → ユースケース、call引数、出力処理
├── domain/                 → ツール、プロファイル、インターフェース
├── entrypoint/             → Cobraコマンドとzsh補完
└── infrastructure/
    ├── mcpclient/          → MCPクライアント、発見、実行、出力整形
    └── profile/            → 設定・プロファイル読み込み、検証、保存
```

### データフロー（CLI Mode）

```
User → entrypoint/root.go → entrypoint/call.go
                              ├── infrastructure/profile → プロファイル解決・検証
                              ├── application/call.go      → 引数パース・型変換
                              ├── infrastructure/mcpclient → MCP Client → MCP Server
                              └── application/call.go      → stdout / stderr
```

### 並列ツール一覧取得

`ListTools` はプロファイル内の全サーバーに **goroutine で並列接続** し、すべてのツールエントリを集約して返す。

## シェル補完（zsh）

```bash
# 現在のシェルに読み込み
source <(mcpctl completion zsh)

# 永続設定
mcpctl completion zsh > ~/.zsh/completions/_mcpctl
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -Uz compinit && compinit' >> ~/.zshrc
```

`call` コマンドではツール名の補完に続き `--パラメータ名` の補完、さらに enum 型パラメータの値補完が動作する。

## エラーハンドリング

- `call` で指定された出力形式が `raw` / `tsv` / `table` 以外の場合、stderrにエラーメッセージを表示して非ゼロで終了
- ツール実行の結果 `res.IsError` が `true` の場合、`"Tool execution returned an error:"` を出力してから内容を表示
- パラメータ JSON やCLI引数のパースに失敗した場合はstderrにエラーを表示して非ゼロで終了

## 注意事項

- `output.format` は `call` の既定出力形式として参照され、CLIの `-o` 指定が優先される
- プロファイル名省略時の動作は設定必須（未設定の場合はエラーになる）
- `cache/` は将来用に予約されており、現時点では使用されない
