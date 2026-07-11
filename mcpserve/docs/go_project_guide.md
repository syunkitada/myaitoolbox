# Go プロジェクトガイド

Go プロジェクトで共通して利用できるコーディング規約・パターン・ベストプラクティスです。

## ディレクトリ構成

### 基本構成

```
cmd/
    <entrypoint>/
        main.go                 # エントリーポイント
internal/
    domain/                     # ドメイン層（型・インターフェースのみ）
    application/                # アプリケーション層（UseCase）
    infrastructure/             # インフラストラクチャ層（実装）
```

### モジュール構成（大規模プロジェクト）

機能ドメインごとにモジュールを切り、各モジュールが独立したレイヤード構造を持つ:

```
internal/
    domain/                     # コアドメインの型・インターフェース
    application/                # コアの UseCase、ファサード
    infrastructure/             # コアのインフラ実装
    modules/
        <module>/
            domain/             # モジュール固有の型・インターフェース
            application/        # モジュール固有の UseCase
            infrastructure/     # モジュール固有のインフラ実装
```

## レイヤードアーキテクチャ

DDD (Domain-Driven Design) のレイヤードアーキテクチャを採用し、依存関係を厳密に制御します。

```
Entrypoint (cmd/)
      │
      ▼
Application (internal/application/)
      │
      ▼
   Domain (internal/domain/)       ← コアドメイン
      ▲
      │
Infrastructure (internal/infrastructure/)
```

### レイヤーの責務

| レイヤー | 責務 | 許可されるimport |
|---------|------|-----------------|
| **Entrypoint** | CLI 解析、サーバー起動、依存の組み立て | Application のみ |
| **Application** | UseCase 実装、ファサード | Domain のみ |
| **Domain** | `type` と `interface` 定義のみ | 他レイヤを参照不可 |
| **Infrastructure** | Domain の interface 実装、外部通信 | Domain のみ |

### 制約

- Domain には `func` を定義しない（`type` と `interface` のみ）
- Domain から他レイヤを import しない
- Infrastructure は Domain の interface を実装するが、Application には依存しない
- Entrypoint は Infrastructure を直接参照しない（Application を経由する）
- パース、フォーマット、ユーティリティ関数は Application 層または Infrastructure 層に配置する

### 制約違反がもたらす問題

| 制約違反 | 起こる問題 |
|---------|----------|
| Domain に func を書く | 外部依存が混入し、テストが困難になる |
| Domain が Infrastructure を import する | 循環参照、ドメインロジックの外部サービス依存 |
| Infrastructure が Application を import する | 関心の分離が崩れる、再利用性が低下 |

## 命名規則

### パッケージ名

- 小文字の単数形（`user`, `order`, `monitoring`）
- テスト用パッケージは `_test` サフィックスではなく、同パッケージで書く（`package domain` のまま）
- パッケージプレフィックスを避ける（`userPackage` → `user`）

### ファイル名

| ファイル | 記述する内容 |
|---------|------------|
| `domain/<entity>.go` | 型定義（struct）、Repository インターフェース |
| `application/app.go` | UseCase 構造体とメソッド |
| `application/*_utils.go` | ユーティリティ関数（パース、フォーマット等） |
| `infrastructure/<service>.go` | 外部サービスクライアントの実装 |
| `infrastructure/*_test.go` | テスト（httptest モックサーバー等） |
| `init.go` | `init()` 関数による自動登録 |

### 構造体・関数名

```go
// 構造体: 大文字CamelCase
type UserClient struct{}

// 構造体フィールド: 大文字CamelCase
type User struct {
    ID    string
    Name  string
    Email string
}

// 関数・メソッド: 大文字CamelCase
func NewUserClient(url string) *UserClient {}
func (c *UserClient) FindByID(ctx context.Context, id string) (*User, error) {}

// 未公開構造体: 小文字camelCase
type userClient struct {
    baseURL string
}

// コンストラクタ: New + 構造体名
func NewUserRepository(db *sql.DB) domain.UserRepository {}
```

### インターフェース命名

```go
// リポジトリ: Entity名 + Repository
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

// クライアント: Service名 + Client
type AlertmanagerClient interface {
    GetAlerts(ctx context.Context, filters ...string) ([]Alert, error)
}

// メソッド: 動詞 + 名詞
FindByID(ctx, id)     // OK
GetUser(ctx, id)      // OK
UserByID(ctx, id)     // NG
```

### 定数

```go
// 文字列列挙型
type OrderStatus string

const (
    OrderStatusPending   OrderStatus = "pending"
    OrderStatusCompleted OrderStatus = "completed"
)
```

## 各レイヤーの書き方

### Domain レイヤー

型定義とリポジトリインターフェースのみを定義します。`func` は書きません。

```go
package domain

import "context"

type User struct {
    ID    string
    Name  string
    Email string
}

type OrderStatus string

const (
    OrderStatusPending   OrderStatus = "pending"
    OrderStatusCompleted OrderStatus = "completed"
)

type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
    List(ctx context.Context) ([]User, error)
}
```

### Infrastructure レイヤー

Domain のインターフェースを実装します。

```go
package infrastructure

import (
    "context"
    "github.com/.../internal/domain"
)

type userRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
    return &domain.User{ID: id, Name: "Example"}, nil
}

func (r *userRepository) Save(ctx context.Context, user *domain.User) error {
    return nil
}
```

- 構造体は未公開（`userRepository`）にする
- コンストラクタは `NewXxxRepository` でエクスポート
- Domain の型は `domain.User` とパッケージプレフィックスを付けて明確にする

### Application レイヤー

UseCase を実装し、Domain のみに依存します。

```go
package application

import (
    "context"
    "github.com/.../internal/domain"
)

type App struct {
    userRepo  domain.UserRepository
    orderRepo domain.OrderRepository
}

func NewApp(userRepo domain.UserRepository, orderRepo domain.OrderRepository) *App {
    return &App{
        userRepo:  userRepo,
        orderRepo: orderRepo,
    }
}

func (a *App) GetUser(ctx context.Context, id string) (*domain.User, error) {
    return a.userRepo.FindByID(ctx, id)
}

func (a *App) CreateUser(ctx context.Context, name, email string) (string, error) {
    user := &domain.User{
        Name:  name,
        Email: email,
    }
    return a.userRepo.Save(ctx, user)
}
```

- フィールドには Domain のインターフェース型を使用（具象型ではなく）
- Infrastructure は DI で注入する
- ビジネスルールを UseCase ロジックに記述する

## エラーハンドリング

### 基本原則

- エラーは値として扱い、`panic` は使わない（テスト時の `panic` は例外）
- エラーをラップして文脈を追加する（`%w`）
- 呼び出し元にエラーを返し、処理を委譲する

```go
func (a *App) GetUser(ctx context.Context, id string) (*domain.User, error) {
    user, err := a.userRepo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    return user, nil
}
```

### ドメイン固有エラー

```go
package domain

type NotFoundError struct {
    Entity string
    ID     string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
}

// 使用例
func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
    // ...
    if user == nil {
        return nil, &domain.NotFoundError{Entity: "User", ID: id}
    }
    return user, nil
}
```

### Infrastructure でのエラー変換

```go
func (c *grafanaClient) QuerySummary(ctx context.Context, ...) ([]MetricSummary, error) {
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("grafana request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("grafana returned status %d", resp.StatusCode)
    }
    // ...
}
```

## 設定

### 環境変数 (.env)

`godotenv` を使って `.env` ファイルから読み込みます。

```go
// main.go
if err := godotenv.Load(); err != nil {
    // .env ファイルがなくてもエラーにしない
}

dbURL := os.Getenv("DATABASE_URL")
if dbURL == "" {
    dbURL = "postgres://localhost:5432/mydb" // デフォルト値
}
```

### CLI フラグ (cobra)

```go
var (
    port    string
    logLevel string
)

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "My Application",
    RunE:  run,
}

func init() {
    rootCmd.PersistentFlags().StringVar(&port, "port", "8080", "listen port")
    rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level")
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 設定の優先順位

```
CLI フラグ > 環境変数 > .env ファイル > デフォルト値
```

## ログ (slog)

Go 1.21 以降の標準 `log/slog` を使用します。

### 初期化

```go
func initLogger(level string) {
    var lvl slog.Level
    switch strings.ToLower(level) {
    case "debug":
        lvl = slog.LevelDebug
    case "warn":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}
```

### 使用法

```go
// インフォメーション
slog.Info("server started", "addr", addr, "port", 8080)

// デバッグ
slog.Debug("processing request", "path", r.URL.Path)

// エラー
slog.Error("failed to connect", "error", err, "host", dbHost)
```

### ポイント

- JSON ハンドラを使用（構造化ログ）
- キーは小文字スネークケース（`user_id`, `error`）
- エラーには `"error"` キーを使用

## テスト戦略

### レイヤー別のテスト手法

| レイヤー | テスト手法 | ツール |
|---------|----------|--------|
| Domain | 型の動作確認（ほぼ不要） | 標準 `testing` |
| Application | モックリポジトリで UseCase テスト | インターフェースのモック |
| Infrastructure (registry) | メモリ上での登録/取得テスト | 標準 `testing` |
| Infrastructure (client) | 外部API モックサーバーテスト | `net/http/httptest` |
| Entrypoint | 結合テスト | 実際のサーバー起動 |

### モックリポジトリ

```go
type mockUserRepository struct {
    users map[string]*domain.User
}

func (m *mockUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
    if u, ok := m.users[id]; ok {
        return u, nil
    }
    return nil, &domain.NotFoundError{Entity: "User", ID: id}
}

func (m *mockUserRepository) Save(ctx context.Context, user *domain.User) error {
    m.users[user.ID] = user
    return nil
}
```

### httptest モックサーバー

```go
func TestGetUser(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(domain.User{ID: "1", Name: "Test"})
    }))
    defer ts.Close()

    client := NewUserClient(ts.URL)
    user, err := client.FindByID(context.Background(), "1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if user.Name != "Test" {
        t.Errorf("expected Name=Test, got %s", user.Name)
    }
}
```

### テストコマンド

```bash
# 全テスト実行
go test ./...

# 特定パッケージ
go test ./internal/modules/user/...

# カバレッジ
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 競合検出
go test -race ./...
```

## デザインパターン

### レジストリパターン (init() + blank import)

プラグイン型の拡張性が必要な場合に利用します。

```
infrastructure/registry.go          ← グローバルレジストリ
modules/<module>/init.go            ← init() で自動登録
application/imports.go              ← blank import でトリガー
```

```go
// infrastructure/registry.go
var providers = make(map[string]domain.Provider)

func Register(p domain.Provider) {
    if _, exists := providers[p.Name()]; exists {
        panic(fmt.Sprintf("provider %q already registered", p.Name()))
    }
    providers[p.Name()] = p
}

func Get(name string) (domain.Provider, bool) {
    p, exists := providers[name]
    return p, exists
}

func List() []domain.Provider {
    var list []domain.Provider
    for _, p := range providers {
        list = append(list, p)
    }
    return list
}
```

```go
// modules/<module>/init.go
func init() {
    infrastructure.Register(New())
}
```

```go
// application/imports.go
import (
    _ "github.com/.../internal/modules/monitoring"
    _ "github.com/.../internal/modules/logging"
)
```

### 登録フロー

```
1. main.go が application パッケージを import
2. application/imports.go の blank import が各モジュールをロード
3. 各モジュールの init() が実行される
4. infrastructure.Register() がグローバル map に登録
5. main.go で application.Get(name) して利用
```

### ファサードパターン

Application パッケージが Infrastructure の詳細を隠すファサードとして機能します。

```go
package application

import "github.com/.../internal/infrastructure"

func List() []domain.Provider {
    return infrastructure.List()
}

func Get(name string) (domain.Provider, bool) {
    return infrastructure.Get(name)
}
```

- Entrypoint は Application のみを import する
- Infrastructure の存在を Entrypoint から隠蔽する

### ラッパーパターン（クロージャ）

横断的な関心事（ログ、メトリクス等）をハンドラに追加する場合:

```go
func WrapTool(handler func(context.Context, *Request) (data, meta interface{}, err error)) func(context.Context, *Request) (data, meta interface{}, err error) {
    return func(ctx context.Context, req *Request) (data, meta interface{}, err error) {
        slog.Info("tool called", "tool", req.Name, "params", req.Args)
        data, meta, err = handler(ctx, req)
        if err != nil {
            slog.Error("tool error", "tool", req.Name, "error", err)
        }
        return data, meta, err
    }
}
```

## コーディングヒント

### インターフェースの最小化

```go
// OK: 使用するメソッドのみ定義
type Reader interface {
    Read(ctx context.Context, id string) (*Entity, error)
}

// NG: 不要なメソッドまで含める
type Repository interface {
    Read(ctx context.Context, id string) (*Entity, error)
    Write(ctx context.Context, entity *Entity) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context) ([]Entity, error)
    Count(ctx context.Context) (int, error)
}
```

### ctx の扱い

- 全ての公開関数に `context.Context` を最初の引数として受け取る
- テスト時は `context.Background()` を使用する

```go
func (a *App) GetUser(ctx context.Context, id string) (*domain.User, error) {
    // ctx を通してリポジトリに渡す
    return a.userRepo.FindByID(ctx, id)
}
```

### エクスポート/非公開

- インターフェースとコンストラクタはエクスポート
- 具象構造体は非公開（`userRepository`）
- テスト用のモックはテストファイル内に定義
