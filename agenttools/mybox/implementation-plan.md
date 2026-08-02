# mybox 実装計画書（v1）

## 1. 目的と範囲

[proposal.md](./proposal.md) で定義された **mybox**（Markdownベースの個人ワークスペース）を実装するための計画を定義します。

### スコープ（v1）

* リソース: タスク（Task）・ナレッジ（Knowledge）
* CLI（cobra）による全機能の提供
* Web UI（HTTP Server + フロントエンド）による表示・編集
* Markdownを唯一のデータソースとし、データベースは使用しない

### アウトオブスコープ（v1）

* Skill / Reference / Event / Agent（将来拡張）
* マルチユーザー・認証
* サーバー単体運用（`serve` はローカル利用を想定）

---

## 2. 参照ドキュメント

| ドキュメント | 用途 |
| --- | --- |
| [proposal.md](./proposal.md) | 機能仕様 |
| [Go アーキテクチャ・設計方針](../../docs/golang/golang_architecture.md) | レイヤリング・依存方向 |
| [Go プロジェクト構成](../../docs/golang/golang_project_structure.md) | ディレクトリ構成 |
| [Go 技術スタック](../../docs/golang/golang_technology_stack.md) | 使用ライブラリ |
| [Webアプリの参考](./tmp/references) | Web UI の技術・UI 参考 |

---

## 3. 技術スタック

[golang_technology_stack.md](../../docs/golang/golang_technology_stack.md) に従い、以下を使用します。

### Go（バックエンド）

| 用途 | ライブラリ |
| --- | --- |
| CLI | `github.com/spf13/cobra` |
| HTTP Server | `github.com/labstack/echo/v4` |
| OpenAPI | `github.com/oapi-codegen/oapi-codegen` |
| 設定ファイル | `github.com/goccy/go-yaml` |
| ログ | `log/slog`（標準） |
| テスト | `testing` + `github.com/stretchr/testify` |
| Lint | `golangci-lint` |

### フロントエンド

proposal は Obsidian / Perlite 相当の機能（Graph View、Mermaid、Backlinks、WikiLink、GTDボード）を要求するため、SPA を採用します。

| 用途 | ライブラリ |
| --- | --- |
| フレームワーク | **React + TypeScript + Vite** |
| Graph View | `react-force-graph` |
| Mermaid | `mermaid` |
| ドラッグ＆ドロップ | `@dnd-kit/core` |
| テスト | `vitest` + `@testing-library/react` |
| E2E | `@playwright/test` |

* ビルド成果物は `go:embed` で Go バイナリに同梱（単一バイナリ配布）

> Graph View は **react-force-graph** を採用します。理由:
> - React コンポーネントとして組み込みやすく、Node/Link の操作（ドラッグ・ホバー・クリック）を標準サポート
> - Canvas 描画のためノード数が増えても描画性能を維持できる
> - Obsidian の Graph View と同様の対話操作（ノード移動・連鎖ハイライト）を少ないコードで実現できる

---

## 4. プロジェクト構成

[golang_project_structure.md](../../docs/golang/golang_project_structure.md) の標準レイアウトに従います。

```text
mybox/
├── cmd/
│   └── mybox/
│       └── main.go                  # エントリーポイント（CLI + serve の起動）
├── internal/
│   ├── entrypoint/
│   │   ├── bootstrap.go             # DI、設定ロード、共通起動処理
│   │   └── server.go                # HTTP サーバー起動・ルーティング
│   ├── application/
│   │   ├── project.go               # プロジェクト UseCase
│   │   ├── task.go                  # タスク UseCase
│   │   ├── knowledge.go             # ナレッジ UseCase
│   │   └── search.go                # 横断検索 UseCase
│   ├── domain/
│   │   ├── task.go                  # Task エンティティ・Status・Priority
│   │   ├── knowledge.go             # Knowledge エンティティ
│   │   ├── project.go               # Project エンティティ
│   │   ├── task_repository.go       # TaskRepository ポート
│   │   ├── knowledge_repository.go  # KnowledgeRepository ポート
│   │   ├── config_store.go          # ConfigStore ポート
│   │   ├── searcher.go              # Searcher ポート
│   │   └── errors.go                # ドメインエラー
│   └── infrastructure/
│       ├── config/
│       │   └── store.go             # YAML 設定ファイルの読み書き
│       └── markdown/
│           ├── frontmatter.go       # Front Matter のパース・シリアライズ
│           ├── task_repository.go   # TaskRepository 実装
│           ├── knowledge_repository.go  # KnowledgeRepository 実装
│           ├── searcher.go          # Searcher 実装
│           └── template.go          # テンプレート解決・描画
└── web/                             # フロントエンド（Phase 7）
    ├── index.html
    ├── src/
    │   ├── main.tsx
    │   ├── api/                     # REST API クライアント
    │   ├── components/
    │   ├── features/
    │   │   ├── board/               # GTDボード
    │   │   ├── tasks/
    │   │   └── knowledge/
    │   └── ...
    └── package.json
```

### レイヤー責務と依存規則

proposal のアーキテクチャ図（CLI / HTTP Server → Application → Domain）を、[golang_architecture.md](../../docs/golang/golang_architecture.md) のレイヤリングにマッピングします。

| レイヤ | 責務 | 依存先 |
| --- | --- | --- |
| Entrypoint | cobra コマンド、HTTP ルーティング、DI、設定ロード、プロセス制御 | Application / Domain / Infrastructure |
| Application | ユースケース実装、バリデーション、フロー制御 | Domain（ポート）のみ |
| Domain | エンティティ、値オブジェクト、エラー、ポート（interface） | なし |
| Infrastructure | Markdown / 設定ファイル操作の実装 | Domain（ポート実装） |

* **Domain に `func` は記述しない**（`type`・`interface`・定数のみ。パース等は Application / Infrastructure に配置）※ mcpserve の慣習に準拠
* **1 Adapter = 1 Interface** を維持する

### ポート設計の考え方

[architecture.md](../../docs/golang/golang_architecture.md) の「Split by Responsibility」に従い、**Capability 単位**でポートを定義します。

| ポート | Capability |
| --- | --- |
| `TaskRepository` | タスクの Markdown 永続化（1ディレクトリ=1タスク） |
| `KnowledgeRepository` | ナレッジの Markdown ファイル永続化 |
| `ConfigStore` | 設定ファイル（YAML）の読み書き |
| `Searcher` | Markdown リポジトリ横断の全文検索 |

Task と Knowledge はストレージモデル（ディレクトリ規約）が異なるため別 Capability とします（Entity 単位の分割ではなく、ストレージ責務の違いに基づく）。

---

## 5. 設定ファイル

proposal の設定形式を維持しつつ、保存場所と読込順序を定義します。

### 保存場所

* `~/.config/mybox/config.yaml`（XDG Base Directory 準拠）
* 環境変数 `MYBOX_CONFIG` で上書き可能

### 内容

```yaml
projects:
  - name: toolbox
    path: ~/workspace/toolbox

default_project: toolbox
```

### 動作仕様

* `project add` はパスを絶対パスに正規化して保存
* `project remove <name>` で削除（`default_project` が削除対象なら未設定にする）
* 設定ファイルがない場合は `project add` で初回作成

### state.yaml（Web UI の状態管理）

お気に入り・最近開いたファイルなど、Markdown を汚さない UI 状態は `~/.config/mybox/state.yaml` に保存します。

```yaml
favorites:
  - architecture/hexagonal
  - tasks/20260801_0900_fix-login/task.md

recent_files:
  - architecture/hexagonal
  - golang/context
```

* `favorites`: お気に入り登録したナレッジのパス／タスクID
* `recent_files`: 最近開いたファイル（先頭が最新、上限50件）
* Web UI からのみ更新（CLI では変更しない）

---

## 6. Markdownスキーマ

### 6.1 タスク

ディレクトリ構成

```text
tasks/YYYYMMDD_HHmm_<task-name>/
    task.md
```

`task.md`（Front Matter + 本文）

```yaml
---
id: 20260801_0900_fix-login
title: fix-login
status: todo
priority: medium
assignee: ""
due: ""
tags: []
project: toolbox
created: 2026-08-01T09:00:00
---
```

* `id`: ディレクトリ名と同一。`task show` 等の引数として使用
* `status`: `todo` / `doing` / `blocked` / `review` / `done`（GTDボードのカラムと一致）
* `priority`: `low` / `medium` / `high` / `urgent`
* `due`: ISO 8601 日付。未設定は空文字
* 状態変化は Front Matter の直接編集のみ（独自データモデルは持たない）

### 6.2 ナレッジ

通常の Markdown ファイル

```text
knowledge/architecture/hexagonal.md
```

```yaml
---
title: Hexagonal Architecture
aliases: []
tags:
  - golang
type: permanent
created: 2026-07-10T08:28:13
lastmod: 2026-07-10T08:28:13
---
```

* `path`: `knowledge/` からの相対パス（`.md` を除く）。`show` 等の引数として使用
* WikiLink: `[[path]]`、`[[path|表示名]]` をサポート
* Front Matter が存在しない Markdown も読み込める（title は H1 から推定）

### 6.3 テンプレート

```text
templates/task/task.md
templates/knowledge/knowledge.md
```

* `text/template` で `{{.Name}}` `{{.Date}}` 等を描画
* 解決順序: `./templates/` → `<default-project>/templates/` → 組み込みデフォルト

---

## 7. CLI コマンド仕様

[proposal.md](./proposal.md) の CLI 構成をそのまま実装します。

```text
mybox
├── serve
├── search <query>
├── project list | add <path> | remove <name>
├── task
│   ├── list      (--all --status --tag --assignee --json)
│   ├── search <query> (--json)
│   ├── show <task-id>
│   ├── create --name <task-name>
│   ├── edit <task-id>
│   ├── set <task-id> (--status --priority --assignee --due)
│   └── archive <task-id>
└── knowledge
    ├── list      (--all --tag --json)
    ├── search <query> (--json)
    ├── show <path>
    ├── create <path>
    ├── edit <path>
    ├── move <old> <new>
    └── rename <old> <new-name>
```

### 共通仕様

* 構造化データ出力コマンドは `--json` をサポート
* 対象プロジェクトの指定は `--project <name>`（未指定なら `default_project`）
* パス検証: `..` を含むパス、`knowledge/` 外への遷移はエラー（パストラバーサル対策）
* `edit` は `$EDITOR`（未設定時は `vi`）で `task.md` / 対象ファイルを起動

### タスク作成フロー

1. `--name` をスラッグ化（空白→`-`、小文字化）
2. テンプレート解決（`./templates/task/task.md` → default project → 組み込み）
3. `tasks/YYYYMMDD_HHmm_<slug>/` を作成し `task.md` を生成
4. 生成した `id` を標準出力

### アーカイブフロー

`tasks/<id>/` を `archives/tasks/<id>/` へ移動。

* Git リポジトリ内なら `git mv`、それ以外は `os.Rename`
* `archives/tasks/` が存在しない場合は自動作成

---

## 8. 検索仕様

### 対象（ナレッジ）

* タイトル / ファイル名 / Front Matter / Markdown本文 / WikiLink

### タスク

* `task.md` の Front Matter（title・status・tags・assignee）と本文

### 実装方針

* データベースを使用しないため、インメモリインデックス＋全文スキャンを実装
  * `Searcher.Search(ctx, query, SearchOption)` をポート化
  * 実装: 対象ディレクトリを走査し、Front Matter・本文を取得して大文字小文字を無視した部分一致＋スコアリング
  * リポジトリが大規模になるまで、専用インデックスファイルは作成しない（Start Simple）
* 共通コマンド `mybox search` はタスク＋ナレッジを横断、`--type` で絞り込み

### `--json` 出力形式

```json
{
  "results": [
    {
      "type": "task",
      "id": "20260801_0900_fix-login",
      "path": "tasks/20260801_0900_fix-login/task.md",
      "title": "fix-login",
      "snippet": "..."
    },
    {
      "type": "knowledge",
      "path": "architecture/hexagonal",
      "title": "Hexagonal Architecture",
      "snippet": "..."
    }
  ]
}
```

---

## 9. Web UI（HTTP API + フロントエンド）

### 9.1 HTTP API（echo）

REST API を提供。フロントエンドと CLI はどちらも Application 層を経由します。

| メソッド | パス | 説明 |
| --- | --- | --- |
| GET | `/api/meta` | プロジェクト・設定・タグ一覧・最近のファイル等のメタ情報 |
| GET | `/api/search?q=&type=&project=` | 横断検索 |
| GET | `/api/tasks` | タスク一覧（status/tag/assignee で絞り込み） |
| POST | `/api/tasks` | タスク作成 |
| GET | `/api/tasks/:id` | タスク詳細 |
| PATCH | `/api/tasks/:id` | ステータス等の更新（GTDボードのDnDで使用） |
| POST | `/api/tasks/:id/archive` | アーカイブ |
| GET | `/api/knowledge?path=` | ナレッジ一覧（パスで絞り込み） |
| GET | `/api/knowledge/content?path=` | Markdown 本文取得 |
| PUT | `/api/knowledge/content` | Markdown 編集保存 |
| POST | `/api/knowledge` | ナレッジ作成 |
| POST | `/api/knowledge/move` | 移動 |
| POST | `/api/knowledge/rename` | リネーム |
| GET | `/api/graph` | Graph View 用ノード・リンク情報（WikiLink/Backlinks から生成） |

* `serve` の `--read-only` 時は書き込み系エンドポイントを 403 にする
* 静的アセットは `go:embed` で同梱し、SPA の fallback ルーティングを実装

### 9.2 フロントエンド画面

| 画面 | 機能 |
| --- | --- |
| ダッシュボード | プロジェクト選択、最近開いたファイル、タグ一覧 |
| GTDボード | Todo / Doing / Blocked / Review / Done のカンバン、DnDで `PATCH /api/tasks/:id` |
| タスク一覧・詳細・編集 | 検索、ステータス更新、アーカイブ |
| ナレッジエクスプローラー | フォルダツリー、新規作成、移動、リネーム |
| ナレッジビューア | Markdown 表示、Mermaid、WikiLink、Backlinks、アウトライン、編集 |
| Graph View | ノード・リンク表示 |
| 検索 | 全文検索結果 |
| お気に入り | `~/.config/mybox/state.yaml` で管理 |

---

## 10. 実装フェーズ

各フェーズの完了条件を明確にし、順次進めます。

### Phase 0: 基盤構築

* `go.mod` 初期化（module: `github.com/syunkitada/myaitoolbox/mybox`）
* ディレクトリ構成作成
* golangci-lint 設定、Makefile（`build` / `test` / `lint`）
* `cmd/mybox/main.go` で `mybox` コマンドの起動とバージョン表示

### Phase 1: ドメイン層

* `Task` / `Status` / `Priority` / `Knowledge` / `Project` エンティティと値オブジェクト
* ポート: `TaskRepository` / `KnowledgeRepository` / `ConfigStore` / `Searcher`
* `errors.go`（NotFound、InvalidPath、AlreadyExists など）

### Phase 2: インフラ層

* `frontmatter.go`: Front Matter のパース・シリアライズ（goccy/go-yaml）
* `task_repository.go`: 一覧 / 取得 / 作成 / Front Matter更新 / アーカイブ
* `knowledge_repository.go`: 一覧 / 取得 / 作成 / 移動 / リネーム
* `config_store.go`: 設定ファイルの読み書き
* `template.go`: テンプレート解決と描画
* 単体テスト（一時ディレクトリ上で検証）

### Phase 3: アプリケーション層

* `project.go` / `task.go` / `knowledge.go` / `search.go` のユースケース
* バリデーション（パス・ステータス・優先度・必須項目）
* ユースケースの単体テスト

### Phase 4: CLI

* cobra による全コマンド実装
* `--json` 出力、`$EDITOR` 連携、エラー時の exit code（`1: 一般エラー / 2: 引数エラー`）
* `serve` は Phase 6 までスタブ（`not implemented` でも可、ただし起動自体は可能にする）

### Phase 5: 検索

* `Searcher` 実装（スキャン方式）
* 横断検索 `mybox search`、`task search`、`knowledge search`

### Phase 6: HTTP API

* OpenAPI 仕様（`openapi.yaml`）を作成し、oapi-codegen で Go のハンドラインターフェースを生成
* echo による REST API 実装（生成された型・インターフェースを利用）
* `serve` コマンド完成（`--host` / `--port` / `--no-browser` / `--read-only`）
* ブラウザ自動起動（`xdg-open` / `open`）
* API の単体テスト（httptest）

### Phase 7: Web UI

* フロントエンドのスキャフォールド（Vite + React + TS）+ vitest / Playwright のセットアップ
* `go:embed` によるバイナリ同梱と SPA 配信
* ナレッジ機能（エクスプローラー / 表示 / 編集 / WikiLink / Backlinks / Graph（react-force-graph）/ Mermaid / 検索 / タグ / お気に入り / 最近）
* タスク機能（一覧 / 詳細 / 編集 / 検索 / アーカイブ）
* コンポーネントの単体テスト（vitest + Testing Library）

### Phase 8: GTDボード

* カンバンボード実装と DnD によるステータス変更
* ステータス変更の Front Matter への反映確認

### Phase 9: E2Eテスト・品質・仕上げ

* Playwright による E2E テスト（serve を起動して Web UI の主要フローを検証）
  * ナレッジの表示・編集・検索
  * GTDボードでの DnD によるステータス変更と Front Matter 反映
* リポジトリ・ユースケースのテスト網羅
* CLI の E2E テスト（スクリプトによるコマンド実行）
* README 作成（インストール・使い方）
* proposal.md との機能差分レビュー（DOD）

---

## 11. テスト計画

| 対象 | 内容 |
| --- | --- |
| frontmatter | パース・シリアライズの往復テスト、欠損項目の許容 |
| task_repository | 作成→取得→更新→アーカイブの一連のフロー（tempdir） |
| knowledge_repository | 作成・移動・リネーム・パストラバーサル拒否 |
| config_store | 初期化・読み込み・書き込み・`default_project` 削除時の挙動 |
| application | 各ユースケースの正常系・異常系 |
| CLI | 各コマンドの実行テスト（`--json` のスキーマ検証） |
| API | httptest によるハンドラテスト、`--read-only` の 403 検証 |
| search | 検索対象（タイトル / Front Matter / 本文 / WikiLink）の検証 |
| フロントエンド | vitest + Testing Library によるコンポーネント・ロジックの単体テスト |
| E2E | Playwright による Web UI の主要フロー検証（表示・編集・検索・GTDボードの DnD） |

---

## 12. リスクと対応

| リスク | 影響 | 対応 |
| --- | --- | --- |
| 検索が大量ファイルで遅くなる | パフォーマンス | v1 はスキャン方式で十分と判断。必要になった時点でインメモリインデックス追加 |
| WikiLink の整合性（移動・リネーム時の参照先） | データ整合 | v1 は移動・リネーム時に対象ファイルのみ更新。リンク先の全件置換は Phase 7+ の改善課題 |
| 並行編集の競合 | データ整合 | 最後書き込み勝ちを前提。Web UI は編集中の他セッション競合を簡易検出 |
| パス操作のセキュリティ | 安全性 | パス正規化＋プレフィックス検証を共通処理として実装 |
| oapi-codegen による API 型変更のコスト | 保守性 | OpenAPI 仕様を source of truth として管理し、`make generate` で生成を一元化 |

---

## 13. 決定事項

Phase 7 前までに確定する予定だった項目は、以下の通り決定済みです。

| 項目 | 決定 |
| --- | --- |
| フロントエンド構成 | React + TypeScript + Vite |
| Graph View ライブラリ | react-force-graph |
| フロントエンドのテスト | vitest + @testing-library/react + @playwright/test（E2E） |
| お気に入り・最近ファイルの保存先 | `~/.config/mybox/state.yaml` |
| OpenAPI 定義 | oapi-codegen を導入 |

---

## 14. 進め方の方針

* 各フェーズは「ビルド・テストが通る状態」を維持しながら進める
* フェーズ完了ごとに提案元の proposal.md と差分を確認する
* 新規ライブラリ導入時は [golang_technology_stack.md](../../docs/golang/golang_technology_stack.md) の代替選定基準を確認する
* 実装時に本計画書の内容から乖離する場合は、計画書を更新する
