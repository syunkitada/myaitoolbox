# Instructions for AI agents

## 参考ドキュメント

- [Go アーキテクチャ・設計方針](../../docs/golang/golang_architecture.md)
- [Go プロジェクト構成](../../docs/golang/golang_project_structure.md)
- [Go 技術スタック](../../docs/golang/golang_technology_stack.md)

## Architecture

```
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

```
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

## 機能追加・変更時のルール

機能追加・変更時には、対応するREADME.md、docs/* 内のファイルを参照し、必要に応じて更新すること。

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
