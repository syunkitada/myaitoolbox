# mybox 仕様書（v1）

## 概要

**mybox** は、Markdown をベースとした個人向けワークスペースです。

v1では以下の2種類のリソースを管理します。

* タスク（Task）
* ナレッジ（Knowledge）

Markdownリポジトリを**唯一のデータソース（Single Source of Truth）**とし、CLI・Web UI・将来的なAIエージェントはすべて同じMarkdownファイルを操作します。

データベースは使用しません。

将来的には以下のリソースを追加できる設計とします。

* Skill
* Reference
* Event
* Agent

---

# Markdownリポジトリのディレクトリ構成

```text
tasks/
    20260801_0900_fix-login/
        task.md

knowledge/
    architecture/
        hexagonal.md

archives/
    tasks/
        20260720_1000_old-task/
            task.md

templates/
    task/
        task.md

    knowledge/
        knowledge.md
```

---

# CLI構成

```text
mybox
├── serve
├── search
├── project
│
├── task
│   ├── list
│   ├── search
│   ├── show
│   ├── create
│   ├── edit
│   ├── set
│   └── archive
│
└── knowledge
    ├── list
    ├── search
    ├── show
    ├── create
    ├── edit
    ├── move
    └── rename
```

---

# 共通コマンド

## 検索

タスク・ナレッジを横断検索します。

```bash
mybox search <query>
```

### オプション

```text
--type task|knowledge
--project <project>
--json
```

### 例

```bash
mybox search oauth

mybox search ingress --type knowledge

mybox search login --type task
```

---

## Web UIの起動

Web UIを起動します。

```bash
mybox serve
```

デフォルトでは

* HTTPサーバーを起動
* ブラウザを自動で開く
* カレントプロジェクトを表示

デフォルトURL

```text
http://127.0.0.1:8080
```

### オプション

```text
--host <host>
--port <port>
--no-browser
--read-only
```

---

## プロジェクト管理

### 一覧

```bash
mybox project list
```

### 登録

```bash
mybox project add .
```

または

```bash
mybox project add ~/workspace/myproject
```

### 削除

```bash
mybox project remove <project-name>
```

---

# タスクコマンド

## 一覧表示

```bash
mybox task list
```

### オプション

```text
--all
--status
--tag
--assignee
--json
```

---

## 検索

```bash
mybox task search <query>
```

---

## 詳細表示

```bash
mybox task show <task-id>
```

---

## 作成

```bash
mybox task create --name <task-name>
```

### 動作

1. `./templates/task/task.md` を利用
2. 存在しない場合は `<default-project>/templates/task/task.md` を利用
3. 以下を作成

```text
tasks/YYYYMMDD_HHmm_<task-name>/
```

---

## 編集

```bash
mybox task edit <task-id>
```

`$EDITOR` で `task.md` を開きます。

---

## 更新

```bash
mybox task set <task-id>
```

更新可能項目

```text
--status
--priority
--assignee
--due
```

---

## アーカイブ

```bash
mybox task archive <task-id>
```

以下へ移動します。

```text
archives/tasks/
```

---

# ナレッジコマンド

## 一覧

```bash
mybox knowledge list
```

### オプション

```text
--all
--tag
--json
```

---

## 検索

```bash
mybox knowledge search <query>
```

検索対象

* タイトル
* ファイル名
* Front Matter
* Markdown本文
* WikiLink

---

## 詳細表示

```bash
mybox knowledge show <path>
```

例

```bash
mybox knowledge show architecture/hexagonal
```

---

## 作成

```bash
mybox knowledge create <path>
```

例

```bash
mybox knowledge create golang/context
```

生成されるファイル

```text
knowledge/golang/context.md
```

---

## 編集

```bash
mybox knowledge edit <path>
```

`$EDITOR` で開きます。

---

## 移動

```bash
mybox knowledge move <old> <new>
```

---

## リネーム

```bash
mybox knowledge rename <old> <new-name>
```

---

# 設定ファイル

```yaml
projects:
  - name: toolbox
    path: ~/workspace/toolbox

default_project: toolbox
```

---

# Web UI

Web UIは **Obsidian** と **Perlite** を参考にした構成とします。

## ナレッジ機能

* Markdown表示
* Markdown編集
* WikiLink
* Backlinks
* Graph View
* Mermaid
* 全文検索
* エクスプローラー
* アウトライン
* タグ一覧
* お気に入り
* 最近開いたファイル

## タスク機能

Markdownで管理されるタスクを **GTDボード（カンバン）** として表示します。

### ボード

* Todo
* Doing
* Blocked
* Review
* Done

### 機能

* GTDボード表示
* ドラッグ＆ドロップによるステータス変更
* タスク一覧
* タスク詳細表示
* タスク編集
* タスク検索
* 完了タスクのアーカイブ

タスクはMarkdownファイルを直接編集するため、データベースは不要です。

---

# アーキテクチャ

```text
                 +----------------------+
                 |        mybox         |
                 +----------+-----------+
                            |
          +-----------------+-----------------+
          |                                   |
         CLI                             HTTP Server
          |                                   |
          +-----------------+-----------------+
                            |
                   Application Layer
                            |
              +-------------+-------------+
              |                           |
      Task Repository         Knowledge Repository
              |                           |
              +-------------+-------------+
                            |
                  Markdown Repository
```

CLIとWeb UIは共通のApplication Layerを利用します。

将来的なAIエージェントやMCPサーバーも、このApplication Layerを利用することを前提とします。

CLI、HTTP Serverは、Golangで実装し、以下を参考にします。

- [Go アーキテクチャ・設計方針](../../docs/golang/golang_architecture.md)
- [Go プロジェクト構成](../../docs/golang/golang_project_structure.md)
- [Go 技術スタック](../../docs/golang/golang_technology_stack.md)

Webアプリケーションフロントエンドは以下を参考にします。

- [Webアプリの参考](./tmp/references)

---

# 設計方針

* Markdownを唯一のデータソースとする
* データベースは使用しない
* タスクは1ディレクトリ＝1タスクとする
* ナレッジは通常のMarkdownファイルとして管理する
* テンプレートから新規作成する
* CLIとWeb UIは共通のApplication Layerを利用する
* RepositoryはMarkdownファイルを直接操作する
* すべてのコマンドはスクリプトから利用しやすい設計とする
* 構造化データを出力するコマンドは `--json` をサポートする
* Gitとの親和性を重視する
* GTDボードはMarkdownタスクのビューであり、独自のデータモデルは持たない
* 将来的に Skill、Reference、Event、Agent を追加できる拡張性を持つ

