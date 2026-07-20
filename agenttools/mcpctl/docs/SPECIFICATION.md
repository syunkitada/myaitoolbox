# mcpctl 仕様書

## 概要

`mcpctl` は MCP (Model Context Protocol) サーバー群を統合的に操作する CLI 兼 MCP サーバーです。

**2つの動作モード:**

- **CLI Mode**: 人間やAIエージェントが直接操作
- **MCP Server Mode** (`serve`): AIエージェントがMCP経由で接続する統合エンドポイント

## ディレクトリ構成

```
~/.config/mcpctl/
├── config.yaml              # 全体設定
├── profiles/                # プロファイル定義
│   ├── dev.yaml
│   └── prod.yaml
└── cache/                   # キャッシュ
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
| `serve` | | MCPサーバーモード起動（stdio transport） |
| `completion` | `[zsh\|bash]` | シェル補完スクリプト生成 |

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
| `--output` | `-o` | string | `tsv` | 出力形式（raw / tsv / table） |
| `--params` | | string | | パラメータJSON（インライン`{...}`またはファイルパス） |
| `--<paramName>` | `-<s>` | varies | | ツールパラメータ（値なしで boolean true） |

**Human Shortcut:** 最後の引数に `-l` または `-h` を指定すると、実行モードではなく一覧/情報表示モードになる。

## 出力形式（call -o）

| 形式 | 説明 |
|------|------|
| `raw` | レスポンス全体を `json.MarshalIndent` で整形して出力 |
| `tsv` | JSONをパースしタブ区切りで出力（デフォルト） |
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
    command: npx @anthropic/github-mcp-server
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

1. CLI の `--profile` / `-p` フラグ（または MCP Server モードの `profile` パラメータ）
2. `config.yaml` の `default_profile`

## MCP Server モード（serve）

`mcpctl serve` は stdio MCP サーバーとして起動し、以下のツールを公開する:

| MCP Tool | パラメータ | 説明 |
|----------|-----------|------|
| `list` | `profile` (optional) | ツール一覧 |
| `search` | `query` (required), `profile` (optional) | ツール検索 |
| `info` | `tool` (required), `profile` (optional) | ツール詳細 |
| `call` | `tool` (required), `profile` (optional), `params` (optional) | ツール実行 |

各ハンドラーは CLI と同じ `discovery` / `runtime` パッケージを利用する。

## アーキテクチャ

### パッケージ構成

```
cmd/mcpctl/main.go        → エントリーポイント（cli.Execute()）
internal/
├── cli/                   → Cobra コマンド定義
│   ├── root.go            → ルートコマンド + --profile フラグ
│   ├── call.go            → call コマンド + 出力整形
│   ├── list.go            → list コマンド
│   ├── search.go          → search コマンド
│   ├── info.go            → info コマンド
│   ├── profiles.go        → profiles コマンド
│   ├── serve.go           → serve コマンド
│   ├── completion.go      → completion コマンド + zsh補完スクリプト
│   └── completehelper.go  → 補完用隠しコマンド
├── profile/               → プロファイル管理
│   ├── loader.go          → 設定読み込み/保存
│   ├── resolver.go        → プロファイル解決
│   └── validator.go       → バリデーション
├── runtime/
│   └── caller.go          → CallTool（MCPクライアント接続→ツール実行）
├── discovery/             → ツール検出
│   ├── list.go            → ListTools（並列接続）
│   ├── info.go            → GetToolInfo, ParseToolName
│   └── search.go          → SearchTools
├── mcpclient/
│   └── client.go          → MCP クライアントセッション作成
└── mcpserver/
    └── server.go          → MCP サーバーモード実装
```

### データフロー（CLI Mode）

```
User → root.go → call.go
                   ├── profile/resolver.go → プロファイル解決
                   ├── discovery/info.go   → パラメータ型情報取得
                   ├── 引数パース
                   ├── runtime/caller.go   → MCP Client → MCP Server
                   └── call.go (出力整形)  → stdout / stderr
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

- `call` で指定された出力形式が `raw` / `tsv` / `table` 以外の場合、エラーメッセージを表示して終了
- ツール実行の結果 `res.IsError` が `true` の場合、`"Tool execution returned an error:"` を出力してから内容を表示
- パラメータ JSON のパースに失敗した場合はエラーを表示して終了

## 注意事項

- `output.format` の config 設定は現時点ではコード上で参照されていない（CLI の `-o` フラグのみ有効）
- プロファイル名省略時の動作は設定必須（未設定の場合はエラーになる）
