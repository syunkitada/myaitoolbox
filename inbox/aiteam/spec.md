# aiteam: Herdr-based Agent Team 実装仕様書

Version: 1.4
Status: Draft

本仕様は [proposal.md](./proposal.md) を実現するための具体的な実装仕様を定義する。

---

## 1. 目的とスコープ

### 1.1 目的

Herdr と Filesystem を組み合わせて、自律的な Agent Team を構築する。

- Filesystem を永続的な状態・定義の Source of Truth とする
- Herdr を Agent の実行環境および Agent 間通信基盤とする
- Manager Agent がチーム全体を統括する
- Member は永続的な役割として存在し、Agent は Task 実行時に一時的に起動する
- タスクやチームのオーケストレーションは専用の外部システムを置かず、基本的に Manager Agent の判断に委ねる
- ただし、FS→Herdr の機械的同期と役割定義は「薄い CLI」と「Agent 用 Skill」で支援する

### 1.2 実装方針

| 観点 | 決定事項 |
| --- | --- |
| Agent 技術 | Member ごとに `kind` を設定可能（opencode / codex / claude 等）。`herdr agent start --kind` に渡す |
| CLI | `aiteam`（薄い CLI）。FS スケルトン生成、FS↔Herdr 同期、状態確認、Task の機械的操作のみ行う。判断はしない |
| Skill | `aiteam` Skill（マークダウン）を Agent へ注入し、運用ルール・操作手順・協業・エスカレーション手順を教える |
| FS↔Herdr 同期 | 常駐 Watcher を置かない。Manager Agent が Task 作成/完了時に CLI を呼び、その場で Pane 生成・削除する |
| Human↔Manager 通信 | 会話は team タブから `cd members/manager/` して manager role と行う（使い捨て Pane）。会話は必ずしもタスク化せず、manager が判断してタスク化する。role への直接依頼も例外的に許容する |
| Manager の実行形態 | manager は通常の role として扱う。タスク化された依頼は manager 自身の task（assignee: manager）として実行し、分解して Member へアサインする。実作業は manager role のタブで行う。team タブは Human のシェル（role を直接起動する場所） |
| Human の介入 | 通知機構を持たない。Human は herdr の状態（Pane 表示・Agent status）を見て権限・確認待ち・blocked のセッションに気づき、該当セッションへ直接介入する。team タブは介入に使わない |

### 1.3 作業対象の置き場所

- 各 Task の作業ファイルは `tasks/<taskdir>/workspace/` に保存する
- `workspace/` 配下は `.gitignore` により git 管理しない（成果物は git 管理対象外の作業領域）
- Manager および Member Agent の Pane の cwd は原則として Task の作業ディレクトリを基準にする（詳細は §6.3）

---

## 2. 全体アーキテクチャ

```text
                       Human
                         │
                         │ 会話（team タブから cd members/<role>/ して role と対話）
                         │ 依頼（cd members/manager/ → manager がタスク化。role への直接依頼も例外許容）
                         │ 介入（各 role のセッションへ直接。herdr 状態を見て気づく）
                         ▼
                  ┌───────────────┐
                  │  aiteam CLI   │ <── 薄い機械的ヘルパ（判断しない）
                  └───────┬───────┘
                          │
              ┌───────────┴───────────┐
              ▼                       ▼
   ┌────────────────────┐   ┌────────────────────┐
   │     Filesystem     │   │       Herdr        │
   │  (Source of Truth) │   │    (Runtime)        │
   │                    │   │                    │
   │  aiteam.toml       │   │  Workspace = Team  │
   │  members/<role>/   │   │  Tab = Member      │
   │  tasks/*/task.md   │   │  Pane = Active Task│
   │  tasks/*/workspace │   │  Agent = Worker    │
   └────────────────────┘   │  Team Tab(Human の │
                             │ シェル、role 非対応) │
                             └─────────┬──────────┘
                                       │
                       Service 連携（herdr agent prompt）
                            Manager / Member Agent 相互
                            │
                            └── Culture: aiteam Skill（運用ルール）
```

構成要素:

| 要素 | 役割 |
| --- | --- |
| `aiteam` CLI | FS スケルトン生成 / FS↔Herdr 同期 / 状態確認 / Task・Session の機械的操作。判断は一切行わない |
| `aiteam` Skill | Agent に注入する運用ルール。Task 状態の更新方法、Pane の材料化、協業、介入（エスカレーション）手順を定義 |
| Manager Agent | オーケストレーションの判断者。依頼の受け口（`members/manager/` で会話）。依頼を作業と判断したら自身の task としてタスク化し、分解・作成・実行・監視する |
| Member Agent | Task 実行時に一時的に起動する作業 Agent。Role 定義 + Task 定義 + Skill を初期プロンプトに含む |
| Team Tab | role を持たない Human のシェル。ここから `cd members/<role>/` して各 role と対話する。使い捨て Pane のみを扱い、永続情報は持たない |

---

## 3. ディレクトリ構成

### 3.1 Team Directory

```text
team/
├── aiteam.toml              # チーム定義
├── .gitignore               # tasks/*/workspace/ と members/*/scratch/ の除外などを定義
├── AGENTS.md                # （任意）Agent への共通指示
│
├── members/
│   ├── manager/             # manager role の常設デスク（tab 対応）
│   │   ├── manager.md       # Role 定義・状態
│   │   ├── notes.md         # （任意）role 固有メモ
│   │   └── scratch/         # 一時ファイル置き場（git 管理対象外）
│   ├── researcher/
│   │   ├── researcher.md
│   │   └── scratch/
│   ├── developer/
│   │   ├── developer.md
│   │   └── scratch/
│   └── reviewer/
│       ├── reviewer.md
│       └── scratch/
│
├── tasks/
│   └── YYYYMMDD_<taskname>/
│       ├── task.md          # Task 定義・状態（Source of Truth）。manager タスクは assignee: manager
│       └── workspace/       # 作業ファイル置き場（git 管理対象外）
│
└── .aiteam/                 # aiteam CLI のランタイム情報（再生成可能）
    ├── state.json           # Herdr とのマッピング（workspace/tab/pane/agent）
    └── roster.md            # Member ↔ Agent 名の対応表（Agent が参照）
```

- `members/<role>/` は各 role の常設デスク。Herdr の role tab（label と cwd を対応）と一対一に対応する
  - `<role>.md`（Role 定義）と必要に応じ role 固有メモ・下書きを置く
  - `scratch/` は一時ファイル置き場で git 管理しない（`.gitignore` に `members/*/scratch/` を追加）
- team タブは role を持たない「Human のシェル」。専用ディレクトリは持たず、ここから任意の role dir に `cd` して Agent を起動し対話する（§5）
- `tasks/*/workspace/` は git 管理しない（`.gitignore` に `tasks/*/workspace/` を追加）
- `.aiteam/` は再生成可能なランタイムキャッシュ。Source of Truth ではない
- `tasks/<dir>/task.md` と `tasks/<dir>/workspace/` 以外に成果物を置く場合は `.aiteam/state.json` の `note` 等で明示する
- 会話・相談の内容は Filesystem に残さない。必要な文脈は manager（または直接依頼を受けた role）がタスク化する際に `task.md` へ写す
- 複数の Team が同一 Herdr 上で並行管理されることを前提とし、Agent 名・Workspace label には `{team}-` プレフィックスを必ず付与する（Team 間で衝突しない）

### 3.2 命名規則

- Task Directory 名 = 識別子: `YYYYMMDD_<taskname>`（例: `20260913_github-crawler`）
- Member Role: `[a-z][a-z0-9_-]{1,31}`（例: `manager`, `researcher`, `developer`）
- Role Directory: `members/<role>/`（例: `members/developer/`）
- Team Tab label（Herdr）: `{team}`（例: `aiteam`）。Human のシェル。role を持たない
- Role Tab label（Herdr）: `{role}`（例: `developer`）。cwd = `members/<role>/`
- Agent 名: `{team}-{role}`（例: `aiteam-developer`）。ヘルドルール `[a-z][a-z0-9_-]{0,31}` に準拠
- 複数 Team が同一 Herdr で管理されるため、Agent 名は `{team}-` プレフィックスで Team ごとに一意にする

---

## 4. ファイルフォーマット

### 4.1 aiteam.toml

```toml
name = "aiteam"              # チーム名。Workspace label / Agent 名プレフィックス
default_kind = "opencode"    # members/<role>/<role>.md に kind 指定がない場合のデフォルト
default_max_concurrency = 1  # Member ごとの同時実行 Agent 数のデフォルト（role.md で上書き可）
session = ""                 # 使用する Herdr session（空=現在のセッション）
cwd = "./"                   # Team Directory（通常は aiteam.toml の場所）
```

### 4.2 members/<role>/<role>.md

Role 定義（永続的な役割）。front matter でメタ情報、本文で Role プロンプトを記述する。

各 role は `members/<role>/` ディレクトリを持ち、Herdr の role tab（cwd 対応）と一対一で結び付く。

```markdown
---
role: developer
label: Developer          # 表示用
kind: opencode            # herdr agent start --kind に渡す値（省略時は default_kind）
max_concurrency: 1        # この Member の同時実行 Agent 数上限（省略時は default_max_concurrency）
description: 実装担当
---
あなたはこのチームの開発担当（Developer）です。cwd は members/developer/（あなたのデスク）です。

## 責任

- アサインされた Task を遂行する
- 実装コードはタスクの workspace ディレクトリに保存する
- 必要に応じて Reviewer にレビューを依頼する

## 基本ルール

- タスク開始時: task.md の status を `doing` に更新する
- 完了時: 成果物を確認し、status を `done` に更新して Manager へ完了を伝える
- 判断 / 権限待ちになった場合は self 判断せず task.md の status を `blocked` に更新し、Human（または Manager）の介入を待つ

（以下、Role 固有の指示を自由に記述）
```

- `members/<role>/<role>.md` の本文は、Human が該当 role と対話する際の初期プロンプトにもそのまま用いる
- `members/<role>/notes.md` は role 固有の共有メモ（任意）。git 管理対象
- `members/<role>/scratch/` は一時ファイル置き場。git 管理しない
- `members/<role>/` の cwd で起動した Agent は「その role として」応答する（Role 定義 + AITEAM_SKILL.md を読み込む）

`max_concurrency` は「同一 Member が同時に実行できる Active Task / Collab Session の数」。

### 4.3 tasks/<taskdir>/task.md

Task 定義・状態。

```markdown
---
assignee: developer
status: doing
depends_on: []            # 事前に完了が必要な Task の名前一覧（省略可）
---

## Overview

GitHub crawler を実装する。

## 目標

- 指定リポジトリのメタデータを取得できる
- 結果は workspace/ に JSON で出力する

## TODO

- [ ] GitHub API の仕様を確認
- [ ] crawler を実装
- [ ] test を追加
- [ ] 動作確認
```

- `assignee`: この Task の担当 Role。通常は `researcher` / `developer` / `reviewer`。Human の依頼を引き受けた manager 自身が担当する場合は `manager` を指定する
- `depends_on`: この Task を開始する前に `done` になっている必要がある Task の識別子（`YYYYMMDD_<taskname>`）のリスト。単一依存の場合はスカラでも許容する
- `priority` は導入しない。優先度の判断は Manager に委ねる

### 4.4 status の値

`todo` / `doing` / `done` / `blocked`。

- `blocked` = 「外部（Human / Manager）の介入待ち」。Human は herdr の状態を見て blocked セッションを発見し、該当セッションへ直接介入する（§5.4）

ライフサイクル:

```text
todo → doing → done        （正常）
todo → doing → blocked     （要 Human / Manager 介入）
done → todo                （再実行）
blocked → todo             （介入・判断後に再開）
```

### 4.5 .aiteam/state.json スキーマ

```json
{
  "version": 1,
  "team": "aiteam",
  "workspace_id": "w1",
  "team_tab_id": "w1:t0",
  "members": {
    "manager": {
      "tab_id": "w1:t1",
      "root_pane_id": "w1:p1",
      "cwd": "members/manager/",
      "agent_name": "aiteam-manager",
      "kind": "opencode",
      "max_concurrency": 1,
      "type": "manager"
    },
    "developer": {
      "tab_id": "w1:t2",
      "root_pane_id": "w1:p3",
      "cwd": "members/developer/",
      "agent_name": "aiteam-developer",
      "kind": "opencode",
      "max_concurrency": 1,
      "type": "member"
    }
  },
  "sessions": {
    "20260913_github-crawler": {
      "assignee": "developer",
      "pane_id": "w1:p4",
      "agent_name": "aiteam-developer",
      "type": "task"
    }
  }
}
```

- `sessions` の key: Task の場合は Task 識別子、Collab の場合は `<role>-<seq>` などの一時 ID
- `sessions` の `type`: `task`（正式 Task）/ `collab`（アドホック協業セッション）
- Member の同時実行数は `Members の max_concurrency`（または `aiteam.toml` の `default_max_concurrency`）で制御する
- `team_tab_id`: team タブ（Human のシェル）の tab ID。role は持たず、会話自体は状態を持たず使い捨て Pane を使う
- members の `cwd`: 各 role tab の root pane の cwd（`members/<role>/`）。`aiteam sync` が確認・警告する
- manager も通常の Member と同じ扱い。manager の task / session は assignee として `sessions` に現れ、Agent 名は `{team}-manager`
- このファイルは `aiteam sync` により再生成可能なキャッシュ

---

## 5. Herdr マッピング

| Herdr | Agent Team |
| --- | --- |
| Workspace | Team |
| Tab | Member（Role tab は `members/<role>/` と対応、Team tab は Human のシェル） |
| Pane | Active Task（または Collab Session、会話 Pane） |
| Agent | Task を実行する Worker |

### 5.1 構築手順

1. Workspace 生成: `herdr workspace create --label <team> --cwd <teamdir>`
2. Team Tab 生成: `herdr tab create --workspace <ws> --label <team> --cwd <teamdir>`（Human のシェル。role を持たない）
3. Role Tab 生成: 各 `members/<role>/` に対して `herdr tab create --workspace <ws> --label <role> --cwd <members/<role>/`
4. 会話時: Human が Team Tab（または任意の Tab）から `cd members/<role>/` し、その場で Pane を開いて role Agent を起動して対話する（§5.4）。aiteam CLI は介在しない
5. Task 実行時: 担当 Member（manager 含む）の Role Tab 内で Pane を split し、Agent を起動して prompt する

### 5.2 Tab / Pane レイアウト

```text
Workspace (team)
│
├── Tab: team               # Human のシェル。専用 dir を持たない（cwd = teamdir）
│   └── Pane (chat): 必要に応じ cd + role Agent を起動（使い捨て）
│
├── Tab: manager            # manager role（cwd = members/manager/）
│   ├── Pane (root): shell（アイドル）
│   └── Pane (task): Manager Agent（manager タスク実行時のみ）
│
├── Tab: researcher         # cwd = members/researcher/
│   ├── Pane (root): shell（アイドル）
│   └── Pane (task): Researcher Agent（Active Task 時のみ）
│
├── Tab: developer          # cwd = members/developer/
│   ├── Pane (root): shell（アイドル）
│   └── Pane (task): Developer Agent（Active Task 時のみ）
│
└── Tab: reviewer           # cwd = members/reviewer/
    ├── Pane (root): shell（アイドル）
    └── Pane (task): Reviewer Agent（Active Task 時のみ）
```

- Team Tab は role を持たない「Human のシェル」。ここから任意の role dir（`cd members/<role>/`）へ移動して Agent を起動し、その role と対話する
- 依頼の受付は原則 manager（team タブ → `members/manager/` で対話）。role への直接依頼も例外的に許容する（§5.4）
- Role Tab の root pane は各 `members/<role>/` にあり、その role の「デスク」。実作業（タスク Pane）もこの Tab 内で行う

### 5.3 制約

- **Member ごとの同時実行 Agent 数は `max_concurrency`（デフォルト 1）まで**とする
  - 上図のレイアウトは `max_concurrency=1` の場合。`2` 以上の場合、同一 Member の Tab に複数の Task Pane が並ぶ
  - Manager は担当 Member の `sessions` 数が `max_concurrency` に達している間、新規 Task をアサインしない
- **Agent 名は `{team}-{role}` を基本とし、同時実行時に衝突する場合はサフィックスを付与する**（例: `aiteam-developer-2`）
  - 複数 Team が同一 Herdr 上で管理されるため、`{team}-` プレフィックスで Team 間の一意性を保つ
  - サフィックス付与は `aiteam task start` / `aiteam session start` が行う
- Task 完了/終了時に対応 Pane を削除する（`herdr pane close`）。Agent 名はその後解放される
- 会話 Pane は使い捨て。閉じてもタスク化済みの内容は `tasks/` に残る

### 5.4 Human の接点

#### 依頼（manager 経由）

- Human は Team Tab から `cd members/manager/` し、その場で Pane を開いて manager Agent（opencode 等）を起動し対話する（`herdr agent start <team>-manager --cwd members/manager/`）
- cwd が `members/manager/` にあることで、起動した Agent は「manager として」応答する
- 会話は使い捨て。内容は永続しない。閉じるのも Human が行う
- 会話中、manager が作業と判断した依頼のみタスク化し、必要に応じて待つか、Pane を閉じて監視へ移る
- aiteam CLI は会話に介在しない（タスク化後の `task new` / `task start` のみ関与）

#### 直接依頼（role への例外経路）

- Human が各 role に直接依頼したい場合は、Team Tab から `cd members/<role>/` してその role Agent を起動し、内容を伝える
- `<role>.md` の定義がそのまま読み込まれるため、初回起動でも role の文脈が揃う
- この直接依頼は例外的受け口。依頼を受けた role は自身を assignee とする Task（`aiteam task new --assignee <role>`）として作成する
- ただし **直接依頼でも manager の編成判断を優先する**：role は複数タスクが並ぶ場合に manager へ知らせ、優先順位の調整を委ねる
- 参照・相談・Q&A は Task 化せず会話のまま終える

#### 介入（エスカレーションの受付）

- 通知機構を持たない。Human は herdr の状態（Pane 表示・`aiteam status`・Agent status）を適宜見て、権限待ち・確認待ち・`blocked` のセッションを発見する
- 発見したらその role のセッション（Pane）へ直接介入する（返答・許可・判断を伝える）。該当 role の Tab か `cd members/<role>/` した場所から応答する
- 介入後、該当 Agent は作業を再開し、task の status を `todo` / `doing` へ更新する

---

## 6. aiteam CLI 仕様

### 6.1 原則

- CLI は「機械的で決定的な操作」のみ行う。判断しない
- 各コマンドの内部では原則として `herdr` CLI を呼び出して実現する
- 標準出力は JSON と human-readable の両方をサポートする（`--format json`）
- 異常時は標準エラーにメッセージを出し、非ゼロ終了コードで抜ける

### 6.2 コマンド一覧

| コマンド | 内容 |
| --- | --- |
| `aiteam init` | Team Directory スケルトン生成（bundled テンプレート） |
| `aiteam sync` | FS↔Herdr の整合を行う（workspace/tab/pane の生成・整理、state.json 更新） |
| `aiteam status` | チーム状態（members / tabs / agents / sessions / tasks）を表示 |
| `aiteam roster` | Member ↔ Agent 名の対応表を表示・roster.md を更新 |
| `aiteam member add <role> [--kind] [--label]` | Member 定義を追加（`members/<role>/` ディレクトリ一式 + state 更新） |
| `aiteam member rm <role>` | Member 定義を削除（実行中の session がある場合は失敗） |
| `aiteam member list` | Member 一覧表示 |
| `aiteam task new <name> --assignee <role> [--title] [--depends-on <task>[,<task>...]]` | Task ディレクトリと task.md(todo) を作成（assignee に manager を指定可能） |
| `aiteam task start <taskid>` | Task の Pane を材料化し、Agent を起動して初期プロンプトを送る |
| `aiteam task finish <taskid> [--status done\|blocked]` | 後始末。Pane を閉じ、state をクリア（status 更新は Agent 側で実施されるが、安全のため CLI でも反映可） |
| `aiteam task list [--status todo\|doing\|done\|blocked]` | Task 一覧表示（depends_on も表示） |
| `aiteam session start <role> --prompt <text>` | アドホック協業セッションを開始（Pane split + Agent start + prompt） |
| `aiteam session stop <role>` | 協業セッションを終了（Pane close） |
| `aiteam agent list` | 現在の Herdr 上の Live Agent 一覧 |

### 6.3 各コマンドの詳細

#### `aiteam init`

- 指定ディレクトリ（デフォルト `.`）に以下を生成
  - `aiteam.toml`（`--name` を反映、デフォルトはディレクトリ名）
  - `members/<role>/`（デフォルトで `manager` / `researcher` / `developer` / `reviewer` の4つを生成。`--members` で指定可。各 dir に `<role>.md` と空の `scratch/` を作成）
  - `tasks/`
  - `.gitignore`（`tasks/*/workspace/` と `members/*/scratch/` を追加）
  - `.aiteam/` は空
- `--herdr` 指定時は続けて `aiteam sync` 相当で Workspace と Tab も生成する
- 冪等。既存ファイルは上書きしない

初回推奨フロー:

```bash
mkdir aiteam-team && cd aiteam-team && git init
aiteam init --name aiteam
aiteam sync --create          # workspace + team tab + role tabs 生成
# 依頼・相談は team tab から cd members/<role>/ して Agent を直接起動する
```

- Human は herdr TUI の Team Tab で `cd members/manager/` し、manager Agent（opencode 等）を起動して依頼・相談する
- 会話中にタスク化が必要になったら manager が判断し、`aiteam task new <name> --assignee manager` → `aiteam task start` で実作業へ移す
- 人から role への直接依頼は `cd members/<role>/` してその role Agent に伝える（例外的受け口、§5.4）
- 会話のまま完結する相談・Q&A は Pane を閉じて終了（内容は永続しない）

#### `aiteam sync`

FS（Source of Truth）を Herdr に反映する。判断はせず、機械的に整合させる。

| 処理 | 内容 |
| --- | --- |
| Workspace 確認 | `aiteam.toml` のチームに対応する Workspace が無ければ作成（`herdr workspace create`） |
| Team Tab 確認 | チーム名の Tab（label = `{team}`）が無ければ作成（`herdr tab create --label <team>`）。Human のシェルとして使い、専用 dir は持たない |
| Member Tab 確認 | `members/<role>/` の各 Role に対応する Tab が無ければ作成（`herdr tab create --label <role> --cwd members/<role>/`） |
| state 更新 | `.aiteam/state.json` の workspace_id / team_tab_id / tab_id / root_pane_id / agent_name（cwd 含む）を再生成 |
| 孤立 Pane 検出 | どの Task にも属さない Task Pane があれば警告（勝手に削除しない） |

- role の tab が既にある場合も、`members/<role>/` への cwd の不一致を検出して警告する
- `--prune` 付きで、done 済み Task の Pane を閉じる
- 既定では Agent の起動・停止は行わない（判断を伴うため）

#### `aiteam task new`

```bash
aiteam task new github-crawler --assignee developer
# → tasks/20260913_github-crawler/task.md 生成（assignee: developer, status: todo）
# → tasks/20260913_github-crawler/workspace/ 生成

aiteam task new feature-x --assignee developer --depends-on 20260913_github-crawler
# → depends_on: [20260913_github-crawler] を front matter に記載

aiteam task new manage-crawler --assignee manager --title "crawler を実装する"
# → manager 自身のタスクとして作成（manager はこれを分解してメンバーへアサイン）
```

- 日付はシステム日付（YYYYMMDD）を使用
- `--depends-on` は Task 識別子をカンマ区切りで指定。`task.md` に `depends_on` として記載される
- `--assignee manager` で Human からの依頼を manager 自身の task として作れる（会話の内容は task.md の本文に写す）
- workspace/ は空ディレクトリとして生成

#### `aiteam task start <taskid>`

1. `task.md` を読み、`assignee` / `kind` / `depends_on` を解決する
2. `depends_on` の各 Task が全て `done` であることを確認する
   - 未完了の依存がある場合はエラー終了（起動しない）。依存解決は Manager の編成判断に委ねる
3. `state.json` から担当 Member の `tab_id` / `root_pane_id` / `max_concurrency` を取得
   - 見つからない場合は `aiteam sync` と同等の補正を行う
4. 担当 Member の実行中 `sessions` 数が `max_concurrency` 以上なら失敗（並列上限）
5. Agent 名を決定する
   - 実行中 Session がない場合: `<team>-<role>`
   - 実行中 Session が 1 つ以上の場合は末尾に連番: `<team>-<role>-<n>`（例: `aiteam-developer-2`）
6. root pane を split: `herdr pane split --pane <root_pane_id> --direction right --cwd <taskdir> --no-focus`
   - cwd は Task のディレクトリ（`tasks/<taskid>`）を基準にし、Agent は task.md と workspace/ へアクセスできるようにする
7. Agent 起動: `herdr agent start <agent_name> --kind <kind> --pane <pane_id>`
8. 初期プロンプトを送信: `herdr agent prompt <agent_name> <初期プロンプト>`
   - 初期プロンプトは CLI が自動生成する（§7 参照）。内容 = Role 定義(`members/<role>/<role>.md`) + Task 定義(task.md) + Skill ルール + roster
9. `state.json` の sessions に登録する
10. 完了通知: 標準出力に Pane ID / Agent 名を返す。以後は Manager が `herdr agent wait` 等で監視する

初期プロンプト（CLI 生成）の構造:

```text
あなたは <role>（<label>）です。担当タスクを遂行してください。

【自身の役割】
<members/<role>/<role>.md 本文>

【担当タスク】
<tasks/<taskid>/task.md 全文>

【運用ルール】
<aiteam Skill の要約: status 更新・workspace 使用・協業・エスカレーション>

【チーム情報】
Member: <roster の対応表>
自 Agent 名: <agent_name>
```

#### `aiteam task finish <taskid>`

1. `task.md` から現在の `status` を読み取る（必要に応じ `--status` で指定された値を反映）
2. slots 数から担当 Member に残る実行中 Session 数を再計算し、`max_concurrency` を超過しないことを確認
3. sessions から該当 Pane / Agent を取得し `herdr pane close` で閉じる
4. sessions のエントリを削除する
5. blocked の場合は Pane を閉じずに残すオプション `--keep-pane` を提供（調査用）

#### `aiteam session start <role> --prompt <text>`

- Task を作らずに行う軽量な協業セッション（proposal §15 の「軽微な協業」に相当）
- 担当 Member の実行中 `sessions` 数が `max_concurrency` 以上なら失敗（並列上限）
- Task と同じ要領で Agent 名決定 → Pane split → Agent start → prompt（text を渡す）
- state.json の sessions に `type: collab` で登録（セッション ID は `<role>-<seq>` 形式）
- 終了は `aiteam session stop <role>`

#### 会話（と相談）について

- Human↔role の会話は CLI コマンドを用意しない。Human が herdr TUI の Team Tab から `cd members/<role>/` して Agent を直接起動し、自ら Pane を開閉して行う（§5.4 依頼・直接依頼）

---

## 7. aiteam Skill（Agent 運用ルール）

Agent（Manager / Member）に注入するオペレーティングルール。1 ファイルのマークダウンで構成し、初期プロンプトに埋め込む。

### 7.1 配布方法

- CLI に同梱し、`aiteam init` 時に Team Directory へ `AITEAM_SKILL.md` として展開する
- `aiteam task start` が初期プロンプトに要約を埋め込む（Member Agent 向け。manager タスクの場合は Manager ルールを強調）
- Human が `cd members/<role>/` して直接起動する role Agent（opencode 等）には、同じファイルを読み込ませる運用とする（会話中に `.aiteam/AITEAM_SKILL.md` を参照させるか、初期プロンプトへ貼る）
- opencode ユーザの場合は .opencode の skill 登録も推奨する（詳細は §10.3）

### 7.2 コンテンツ

#### A. Source of Truth

- チーム・メンバー・タスクの永続情報は Filesystem にのみ存在する
- 状態の更新は常に `tasks/<taskid>/task.md` の front matter を編集して行う
- Herdr 上の Pane / Tab は一時的な実行環境。Kill / Close されても FS が残る
- 会話は永続しない。作業が必要な内容だけが `tasks/<id>/task.md` に残る

#### B. Manager の会話・タスク化ルール（Manager 向け）

- Human との会話は `members/manager/` で行う。相談・Q&A はその場で応答し、無理にタスク化しない
- Human の依頼を作業が必要と判断した場合のみ、自分を assignee とした task を作成する
  - `aiteam task new <name> --assignee manager` → task.md の本文に依頼内容・文脈を写す → `aiteam task start`
- 作成した manager タスクでは、依頼をさらに分解してメンバーへアサインし（`--assignee <role>`）、完了まで監視する
- タスク化しないと判断した会話の内容は永続しない。必要なら自分の判断根拠を task.md に添えて管理対象にする（任意）

#### B'. 直接依頼（Member 向け、例外的受け口）

- Human が `members/<role>/` で role Agent を起動し、直接依頼・相談してくることがある
- 相談・Q&A はタスク化せずその場で応答する
- 作業と判断した場合は自分を assignee とした Task を作成する（`aiteam task new <name> --assignee <role>`）
- 既存の実行中 Task と優先順位が衝突する・チーム全体にかかわる場合は manager へ知らせ、編成判断を委ねる

#### C. Task 実行ルール

- 開始時: `status: doing` に更新
- 作業ファイルは `workspace/` 以下に保存（git 管理対象外）
- 完了時: 成果物を確認後 `status: done` に更新し、タスク完了を Manager へ伝える
- 継続不能: 自己判断せず `status: blocked` に更新し、理由を task.md に追記して停止

#### D. ファイル / Pane の操作

- Agent は `herdr` CLI と `aiteam` CLI を直接利用してよい
- 自 Pane の ID: `$HERDR_PANE_ID`。呼び出し元の Tab を対象にする場合は `--current` を使う
- 他の Member と通信する際は音声を Agent 名で指定する（`herdr agent prompt <team>-<role> "..." --wait`）

#### E. Member 間協業

- 対象 Member の Agent が Live の場合は直接プロンプトで協業してよい
  - 協業対象は roster または `aiteam roster` で確認し、タスクに関連する Agent 名（`<team>-<role>` またはサフィックス付き）を指定する
  - 例: `herdr agent prompt aiteam-reviewer "タスク X のコードをレビューしてください" --wait`
- 対象 Member の Agent が Live でない場合、Agent の起動は Manager の責務。Manager へ Session 開始を依頼する
- `max_concurrency` が 1 の場合、対象 Role は常に 1 つのみなので `${team}-${role}` で直接指定できる
- 正式な成果物・責任・進捗管理が必要な場合は独立 Task を作成する（Manager が判断）

#### F. エスカレーション

- Manager へ: プロジェクト方針 / スコープ変更 / 依存関係 / 意見不一致 / 判断不能。Member は Manager へ伝え、判断を仰ぐ
- Human へ: Manager / Member が判断不能・権限待ち・確認待ち。→ task を `blocked` に更新し、理由を task.md に追記して待機する。推測で処理しない
  - Human は herdr の状態を見て blocked セッションを発見し、該当セッションへ直接介入する（§5.4）。team タブは介入に使わない
- blocked の Task を勝手に `done` にしない。Human / Manager の介入・判断後に `todo` へ戻す

---

## 8. ライフサイクル設計

### 8.1 Task

```text
aiteam task new ──> tasks/<id>/task.md (status: todo)
        │
        ▼
aiteam task start ──> Pane split + Agent start + initial prompt
        │               │
        │               ▼
        │          Agent が status: doing に更新
        │               │
        │     ┌─────────┴──────────┐
        │  作業継続成功         判断不能
        │     │                    │
        │     ▼                    ▼
        │  status: done        status: blocked
        │     │                    │
        ▼     ▼                    ▼
aiteam task finish ──> Pane close + state クリア
```

- `done → todo`: Task を再実行する場合は `aiteam task start` を再実行（新 Pane）
- `blocked → todo`: Human / Manager 判断後に再実行

### 8.2 Member Agent

- Agent は Task 実行中だけ同一 Role に 1 つ存在する
- Pane close（= Agent 終了）で名前は解放される
- 再アサイン時は同じ Agent 名で再起動する

### 8.3 Collab Session

- Task を伴わない一時的な Agent インスタンス
- `aiteam session start/stop` で管理
- Live Agent 名と `max_concurrency`（並列上限）制約は Task と共通

### 8.4 Manager

- Manager は通常の Member（type: manager）として扱う。常駐せず、task 単位で起動する
- Human との会話は Human が `members/manager/` で起動する使い捨て Pane で行う（Human が自ら開閉）。内容は永続しない
- 依頼を作業と判断したタスクは manager 自身の task（assignee: manager）として実行し、Pane を閉じても `tasks/` に進捗・成果物が残る。再開は `aiteam task start` の再実行
- manager の監視ループ（§9）に従い、タスクの分解・開始・完了検知・Pane 削除・介入判断を行う

---

## 9. Manager のオーケストレーション手順

Human は team タブから `cd members/manager/` して manager と会話し、Manager は作業と判断した場合に自身の task を作って以下のループを運用する。

```text
[Human ↔ Manager]（team タブから cd members/manager/ した会話）
      │
      ▼
1. 依頼内容を理解。作業が必要ならタスク化
      aiteam task new <name> --assignee manager
      （task.md 本文に依頼内容・文脈を写す）
      aiteam task start <id>     → manager 自身の作業 Pane が起動
      │
      ▼
2. タスクを分解してメンバーへアサイン
      aiteam task new <sub> --assignee <role> [--depends-on ...]
      │
      ▼
3. 各タスク開始（待機付き）
      aiteam task start <sub>
      herdr agent wait <team>-<role> --until ...
      │
      ├── done: 成果物確認 → aiteam task finish <sub>
      │         → 必要に応じて後続タスク（レビュー等）を作成
      │
      └── blocked / タイムアウト:
            aiteam status / herdr agent read で状況確認
            → manager が判断 or 自身の対応タスクを作成
            → 判断不能なら自身を blocked にして Human の介入を待つ
      │
      ▼
4. 全体完了 → 自身の task を done にし、task.md / workspace に成果を残す
      aiteam task finish <id>
```

- 複数 Member への同時ディスパッチは可能（各 Member は `max_concurrency` まで）
- Agent 名は取得（roster / `aiteam status`）し、`herdr agent wait <agent_name>` で待機する
- Task の `depends_on` を確認し、依存が `done` になっていない Task は開始しない（並べ替え・待機は Manager の編成判断）
- Manager は自らが判断できる範囲でタスクの再編成・再アサインを行う
- Manager 自身が判断不能 / 権限待ち / 確認待ちになった場合は、task を `blocked` に更新して待機し、Human の直接介入を待つ（§5.4）。推測で処理しない
- 介入後に Human が判断を伝えると、Manager は `aiteam task start` で再開して続きから作業する

---

## 10. 実装計画

### 10.1 リポジトリ構成

```text
agenttools/aiteam/
├── proposal.md            # 元設計（既存）
├── spec.md                # 本仕様
└── cmd/
    └── aiteam/
        └── main.go
    internal/
        ├── domain/        # 型定義（Team, Member, Task, Session, State)
        ├── application/   # ユースケース（init / sync / status / task 等）
        ├── infrastructure/
        │   ├── fs/        # FS 読み書き（front matter パース等）
        │   └── herdr/     # herdr CLI 呼び出しラッパ
        └── cli/           # CLI パースとコマンド実行
    assets/
        ├── members/       # デフォルト Member テンプレート（<role>/<role>.md 形式）
        ├── aiteam.toml.tmpl
        └── AITEAM_SKILL.md
    docs/
        └── development.md # 開発ガイド
```

### 10.2 技術スタック

- Go（リポジトリ内の mcpctl / mcpserve と同じ構成慣習に従う）
- Herdr との連携は `herdr` バイナリをサブプロセスで呼び出し、JSON をパース
- YAML front matter は既存の Go YAML ライブラリでパース
- テスト: `tests/` に CLI 統合テスト（`herdr --no-session` のヘッドレス利用を想定）

### 10.3 Skill の opencode 連携

- `aiteam init` で生成する `.aiteam/AITEAM_SKILL.md` を元に、必要に応じて opencode の Skill として登録する
- Manager / Member（opencode）の `aiteam task start` 時に Skill が初期プロンプトへ挿入される。Human が `cd members/<role>/` して直接起動する role Agent（opencode）にも同一 Skill をロードできる構成をサポートする

### 10.4 実装ステップ

| Phase | 内容 |
| --- | --- |
| 1 | CLI 骨格 + `init` / `member` / `task new`（FS のみ） |
| 2 | `sync` / `status` / `roster`（Herrd 連携 + state.json。team タブ生成含む） |
| 3 | `task start` / `task finish` / `session start` / `session stop`（Pane 材料化と初期プロンプト生成。manager タスク対応含む） |
| 4 | AITEAM_SKILL.md の整備（`cd members/<role>/` 起動時にも読み込ませる運用を含む） |
| 5 | 端末での実運用検証（Human との対話・タスク化・介入・再開含む） |

---

## 11. 検証とテスト

### 11.1 ユニットテスト

- front matter パース / task 状態遷移バリデーション / ワークスペース名の計算
- owner 命名規則・Agent 名の文字列制約（`[a-z][a-z0-9_-]{0,31}`）

### 11.2 統合テスト

- ヘッドレス Herdr（`herdr server` or `--no-session` + 一時セッション）で:
  - `init → sync → task new → task start → (モック Agent) → task finish` の一連の流れ
  - `max_concurrency` 制約の検証（1 / 2 で挙動確認、Agent 名の連番付与含む。manager タスクのサフィックス含む）
  - `depends_on` 未完了 Task の `task start` 失敗
  - done/blocked 時の Pane クリーンアップ
  - `sync` が team タブと role タブを作成すること（`team_tab_id` の解決、role タブの cwd = `members/<role>/`）
  - manager タスク（assignee: manager）が通常の Member と同じ機構で起動・終了すること
  - `init` が `members/<role>/<role>.md` と `scratch/` を作成し、`.gitignore` に `members/*/scratch/` を含めること

### 11.3 E2E（実機）

- opencode を Manager / Member に利用し、Human 役を関数側で模擬しながら
  - Human が team タブから `cd members/manager/` して manager と対話 → 依頼 → manager が `task new --assignee manager` → 分解 → メンバー実行 → 完了
  - blocked → エスカレーション → Human が該当セッションへ直接介入 → `task start` で再開
  - Member 間の直接協業（Developer → Reviewer）
  - 相談・Q&A がタスク化されないこと（会話 Pane を閉じて終了）

---

## 12. エラー処理とリカバリ

| ケース | 対応 |
| --- | --- |
| Pane が誤って閉じられた | `aiteam sync` が孤立検出 / `aiteam task start` の再実行で復帰 |
| Agent が異常終了した | Manager が `herdr agent wait` のタイムアウト / `agent get` の unknown で検知し、Task を reopen する |
| state.json の破損 | `aiteam sync` で FS + Herdr から再生成 |
| `max_concurrency` 超過の起動要求 | CLI が実行中 session 数を確認し失敗させる |
| `depends_on` が未完了 | CLI が未完了を検知して起動を中止。Manager が編成判断する |
| Task の assignee が存在しない | `aiteam sync` 後にエラー。Manager が Member を追加するよう誘導 |
| blocked 状態の放置 | task.md の理由を読み、Human ／ Manager が介入。推測で done にしない |
| 会話 Pane を誤って閉じた | 会話は永続しないため、必要な内容はタスク化されず失われる。重要な依頼は対話中に `task new --assignee <role>` でタスク化してから閉じる運用 |
| 会話 Pane とタスクの Agent 名の衝突 | Human が `cd members/<role>/` して直接起動する際に、`herdr agent start <team>-<role>` の名が実行中タスクと重複した場合はサフィックス付き名（例: `<team>-<role>-chat`）を使う（Herdr の Live Agent 名は一意） |
| 会話が長引いて必要な文脈が散逸 | 受付 role（Manager または直接依頼を受けた role）が要約を伴って自身の task（assignee: その role）を生成し、task.md へ文脈を写す運用を Skill で指示 |

---

## 13. 未解決事項・今後の拡張

- **Human への通知**: エスカレーションや blocked の通知は Herdr 側の表示・通知に任せる。Human が Herdr 上のタブ/Pane の状態を見て適宜介入する運用を前提とし、aiteam 側では通知基盤を持たない
- **複数 Team の同時運用**: 同一 Herdr 上で複数 Team を管理する前提。Agent 名・Workspace に `{team}-` プレフィックスで分離する（現仕様に反映済み）
- **Agent の並列度**: Member ごとに `max_concurrency` で制御（デフォルト 1、`members/<role>/<role>.md` で上書き可）。2 以上にすると Agent 名に連番が付与される（現仕様に反映済み）
- **Task 間依存**: task.md の `depends_on` で表現する（現仕様に反映済み）。ブロックチェーン的依存の自動解決・可視化は今後検討可能
- **Roster の自動更新**: Member Agent への Prompt 時に roster を自動同梱する仕組み（現状は CLI 生成プロンプトに埋め込み）
- **会話の永続性**: `members/<role>/` での会話を任意でログ保存するオプションは将来追加検討（現状はタスク化された内容のみ FS に残る）
- **Manager のタスク再開**: blocked 後に `aiteam task start` で再開する際、前回実行の文脈（起動済みタスク一覧・途中経過）を初期プロンプトへ引き継ぐ仕組みは今後改良可能
- **複数依頼の並行処理**: manager も `max_concurrency` の制約を受けるため、複数依頼の同時タスク化はデフォルトでは逐次。将来は manager の並列度を上げることも検討可能