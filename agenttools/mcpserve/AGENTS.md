# `mcpserve` 開発ガイドライン

このディレクトリでは、複数のMCP Server実装を単一のバイナリに内包する`mcpserve`を開発します。

## 参考ドキュメント

- [Go アーキテクチャ・設計方針](../../docs/golang/golang_architecture.md)
- [Go プロジェクト構成](../../docs/golang/golang_project_structure.md)
- [Go 技術スタック](../../docs/golang/golang_technology_stack.md)

## README.mdの役割

各ディレクトリの `README.md` は、そのディレクトリの目的・使い方・構成を説明する入口です。README.md は、対象ディレクトリ全体と、配下のディレクトリおよびファイルについて説明責任を持ちます。

- ルートの [README.md](./README.md) はリポジトリ全体のIndexとし、`## Index` にトップレベルのディレクトリとファイルを掲載する。
- 各ディレクトリのREADME.mdには、直接の子ディレクトリとファイルの役割を記載する。子ディレクトリにREADME.mdがある場合はリンクし、詳細な説明はそのREADME.mdに委譲する。
- `archives/`、`inbox/`, `_inbox/`、`tasks/`とその配下には、README.mdを必須としない。
- README.mdから参照するパスは、原則として対象README.mdからの相対リンクにする。
- Indexを作成するときは、Git管理対象のファイルとディレクトリだけを掲載し、Git管理対象外のものは除外する。
- ディレクトリまたはファイルを追加・削除・移動・改名した場合は、同じ変更で関係するREADME.mdの構成説明・Index・リンクも更新する。
- 既存のディレクトリやファイルの役割・使い方が変わった場合も、関係するREADME.mdを更新する。
- 生成物やローカル専用ファイルをIndexの対象外にする場合は、README.mdまたは`.gitignore`でその扱いが分かるようにする。

## Indexの記載例

直接の子ディレクトリとGit管理対象のファイルを掲載します。

```markdown
## Index

| パス | 役割 |
| --- | --- |
| [`src/`](./src/) | アプリケーション本体。詳細は `src/README.md` を参照 |
| [`config.toml`](./config.toml) | 設定ファイル |
```

`tmp/`、`.env`、生成物、未追跡ファイルなど、Git管理対象外のものはIndexに掲載しません。

## 変更時の確認

変更前後でディレクトリ構造とファイル一覧を確認し、README.mdの記載漏れ・リンク切れ・古い説明がないことを確認する。新しいディレクトリを追加する場合は、必要に応じてそのディレクトリにもREADME.mdを追加する。

## 実装時の確認

- 仕様や利用方法を変更した場合は、[`docs/`](./docs/)の仕様書・開発手順と、関係するREADME.mdを同じ変更で更新する。
- Goコードは`gofmt`で整形し、テストは一時ディレクトリまたはローカルリポジトリを使って外部環境に依存させない。
- ファイルやディレクトリを追加・削除・移動・改名した場合は、README.mdのIndexと相対リンクを更新する。
- 既存のユーザーデータや作業ツリーを壊す操作（広範囲の削除、無確認のresetなど）は行わない。

## Architecture

```text
Entrypoint
      │
      ▼
Application
      │
      ▼
   Domain
      ▲
      │
Infrastructure
```

- Domain はビジネスルールのみを持つ。
  - Domain は他レイヤを参照してはいけない。
  - Domain には `type`・`interface` 定義のみ記述する。
  - Domain には `func` は定義しない。
- Entrypoint は Application、Domain、Infrastructure を利用する（DIのため）。
- Application は UseCase を実装し、Domain のみを利用する。
- Infrastructure は Domain の `interface` を実装する。
- Module は `internal/modules/` に配置し、各モジュールは同じ Layered Architecture を採用する。
  - Module は Domain のみに依存し、Infrastructure や Application に依存しない。

## ディレクトリ構成

```text
cmd/
    <entrypoint>/
        main.go     # エントリーポイント
internal/
    entrypoint/     # DI、プロバイダー登録、サーバー起動
    domain/         # ドメイン層（ビジネスルール、インターフェース）
    infrastructure/ # インフラストラクチャ層（実装）
    modules/        # MCPモジュール実装
        <module>/
            application/
            domain/
            infrastructure/
```

### Example

```
cmd/
    mcpserve/
        main.go
internal/
    entrypoint/
        registry.go     # プロバイダーレジストリ
        bootstrap.go    # DI、サーバー起動
    domain/
        provider.go     # Provider、Server インターフェース
    infrastructure/
        server.go       # Server実装
    modules/
        monitoring/
            provider.go         # モジュールプロバイダー
            application/
                app.go
                wrap.go
                ...
            domain/
                alert.go
                metric.go
                ...
            infrastructure/
                alertmanager.go
                prometheus.go
                ...
```

domain/database1_repository.go
```
type Database1Repository interface {
    FindUserByID(id string) (*User, error)
    SaveUser(entity *User) error
}

type User struct {
    ID   string
    Name string
}
```

domain 内に func を記述しない。パース・フォーマット・ユーティリティ関数は Application 層または Infrastructure 層に配置する。

infrastructure/database1/repository.go
```
type database1Repository struct {}

func NewDatabase1Repository() Database1Repository {
    return &database1Repository{}
}

func (r *database1Repository) FindUserByID(id string) (*User, error) {
    // 実際のデータベースアクセス処理
    return &User{ID: id, Name: "Example"}, nil
}

func (r *database1Repository) Save(user *User) error {
    // 実際のデータベース保存処理
    return nil
}
```

## Provider Response Format Rules

全てのツールは成功時に `structuredContent` を返すこと。形式は以下の通り:

```json
{
  "structuredContent": {
    "meta": { /* クエリパラメータ、件数、メタ情報 */ },
    "data": { /* または配列 */ }
  }
}
```

- `meta`: リクエストパラメータ、件数、フィルタ条件などのメタ情報
- `data`: ツールの実行結果本体（オブジェクトまたは配列）

エラー時は `IsError: true` を設定し `structuredContent` は省略すること。

ヘルパー: `newStructuredResult(text, meta, data)` を使用すること。

## 開発完了時の必須確認

開発を完了する前に、このリポジトリのルートで次のコマンドを順番に実行する。

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
go install ./cmd/mcpserve
```

`golangci-lint`は、初回またはバージョン更新時に次のコマンドでインストールする。バージョンを固定して、開発者間で検証結果を揃える。

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

`go vet ./...`と`golangci-lint run ./...`を静的解析として実行する。`golangci-lint`の設定は[`.golangci.yml`](./.golangci.yml)で管理する。

`go install ./cmd/mcpserve`が成功したことを確認してから、開発完了とする。インストール先は環境の`GOBIN`または`GOPATH/bin`に従う。

## AGENTS.mdのルール追加

AGENTS.md自体への直接的な指示ではない内容について、AGENTS.mdに新しいルールを追加すべきだと判断した場合は、追加前にユーザーへ相談し、合意を得たうえでルール化する。
