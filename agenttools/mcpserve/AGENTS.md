# Instructions for AI agents

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
- Entrypoint は Application のみを利用する。
- Application は UseCase を実装し、Domain のみを利用する。
- Infrastructure は Domain の `interface` を実装する。

## ディレクトリ構成

```
cmd/
    <entrypoint>/
        main.go     # エントリーポイント
internal/
    application/    # アプリケーション層（UseCase）
    domain/         # ドメイン層（ビジネスルール、インターフェース）
    infrastructure/ # インフラストラクチャ層（実装）
    providers/      # MCPプロバイダー実装
        <provider>/
            application/
            domain/
            infrastructure/
```

### Example

```
cmd/
    myapi/
        main.go
internal/
    domain/
        provider.go       # Providerインターフェース
        registry.go       # プロバイダーレジストリ
    infrastructure/
        server.go         # Server実装
    providers/
        monitoring/
            application/
                usecase1.go
                ...
            domain/
                database1_repository.go
                externalservice1_client.go
                ...
            infrastructure/
                database1/
                    repository.go
                externalservice1/
                    client.go
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
