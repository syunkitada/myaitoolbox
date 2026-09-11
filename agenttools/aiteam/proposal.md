# Herdrベース Agent Team 設計仕様

## 1. 概要

HerdrをAgent Teamの実行基盤として利用し、FilesystemとHerdrを組み合わせて自律的なAgent Teamを構築する。

基本方針は以下の通り。

* Filesystemを永続的な状態・定義のSource of Truthとする
* HerdrをAgentの実行環境およびAgent間通信基盤とする
* Manager Agentがチーム全体を統括する
* Memberは永続的な役割として存在し、AgentはTask実行時に一時的に起動する
* TaskはFilesystem上のMarkdownとして管理する
* ActiveなTaskのみHerdr上のPaneとして存在させる
* Member同士は必要に応じて直接協業できる
* 判断不能な問題はHumanへエスカレーションする
* TaskやTeamのオーケストレーションを専用の外部システムで管理せず、基本的にManager Agentの判断に委ねる

---

# 2. 基本モデル

Herdrの構造をAgent Teamの概念に以下のように対応させる。

| Herdr           | Agent Team      |
| --------------- | --------------- |
| Directory       | Team            |
| Workspace       | Team            |
| Tab             | Member          |
| Pane            | Active Task     |
| Agent           | Taskを実行するWorker |
| Agent State     | Agentの実行状態      |
| Workspace State | Team全体の実行状態     |

概念的には以下の構造となる。

```text
Team Directory
│
├── members/
│   ├── manager.md
│   ├── researcher.md
│   ├── developer.md
│   └── reviewer.md
│
└── tasks/
    ├── 20260913_feature-x/
    │   └── task.md
    ├── 20260913_research-y/
    │   └── task.md
    └── ...
```

Herdr上では、

```text
Workspace
│
├── Tab: manager
│   └── Pane: manager
│       └── Manager Agent
│
├── Tab: researcher
│   └── Pane: Active Task
│       └── Researcher Agent
│
├── Tab: developer
│   └── Pane: Active Task
│       └── Developer Agent
│
└── Tab: reviewer
    └── Pane: Active Task
        └── Reviewer Agent
```

となる。

---

# 3. Team Directory

1 Directory = 1 Herdr Workspace = 1 Teamとする。

Team Directory自体がTeamの永続的な状態を保持する。

```text
team/
├── members/
│   ├── manager.md
│   ├── researcher.md
│   ├── developer.md
│   └── reviewer.md
│
└── tasks/
    ├── YYYYMMDD_<taskname>/
    │   └── task.md
    └── ...
```

Filesystem上の情報をTeamのSource of Truthとする。

Herdr上のWorkspace、Tab、Pane、Agentは、このFilesystem上の情報をもとに構築されるRuntimeである。

---

# 4. Member

## 4.1 Memberの定義

MemberはTeam内の役割を表す。

```text
members/
    manager.md
    researcher.md
    developer.md
    reviewer.md
```

Memberは永続的に存在する。

一つのRoleに対して原則一人のMemberを配置する。

```text
researcher → 1人
developer  → 1人
reviewer   → 1人
```

同一RoleのMemberを複数作成することは原則として避ける。

---

## 4.2 MemberとAgentの違い

MemberとAgentは同一ではない。

```text
Member = 永続的な役割
Agent  = MemberがTaskを実行するための一時的なプロセス
```

例えばDeveloper MemberはTeamに継続して存在するが、Developer AgentはTaskの実行中だけ存在する。

```text
Developer Member
       │
       ├── Task A
       │     └── Agent start
       │           └── done
       │
       └── Task B
             └── Agent start
                   └── done
```

---

# 5. Memberの構成方針

Team初期化時に、プロジェクトの目的から必要なMemberをManagerが決定する。

基本方針は、

* 少数精鋭とする
* Roleごとに原則1人
* 開発者を複数人にしない
* 不要なMemberを作らない
* TaskのたびにMemberを新規作成しない
* プロジェクトの役割が増えた場合のみ必要に応じて追加する

とする。

例えば、

```text
manager
researcher
developer
reviewer
```

という構成で開始した場合、通常はこのMemberを使い回す。

---

# 6. Task

## 6.1 Taskの定義

TaskはFilesystem上のMarkdownとして管理する。

形式は以下とする。

```text
tasks/
└── YYYYMMDD_<taskname>/
    └── task.md
```

例：

```text
tasks/
└── 20260913_github-crawler/
    └── task.md
```

Task Directory名自体をTaskの識別子として扱う。

---

## 6.2 task.md

Taskの状態と担当MemberをFront Matterで管理する。

```yaml
---
assignee: developer
status: doing
---
```

必要に応じて本文にTaskの詳細を記述する。

例えば、

```markdown
---
assignee: developer
status: todo
---

## Overview

GitHub crawlerを実装する。

## TODO

- [ ] GitHub APIの仕様を確認
- [ ] crawlerを実装
- [ ] testを追加
- [ ] 動作確認

## Notes

既存crawlerの実装を参考にする。
```

---

# 7. Task State

Taskの基本状態は以下とする。

```text
todo
doing
done
blocked
```

基本的なライフサイクルは以下。

```text
             ┌─────────┐
             │   todo  │
             └────┬────┘
                  │
             agent start
                  │
                  ▼
             ┌─────────┐
             │  doing  │
             └────┬────┘
                  │
          ┌───────┴────────┐
          │                │
       complete          blocked
          │                │
          ▼                ▼
      ┌──────┐         ┌─────────┐
      │ done │         │ blocked │
      └──────┘         └────┬────┘
                             │
                           Human
```

Taskが再度必要になった場合は、

```text
done → todo
```

として再実行できる。

---

# 8. TaskとPaneの関係

基本原則：

```text
1 Task = 1 Pane
```

ただし、Herdr上に存在するのは**ActiveなTaskのみ**とする。

Taskが`todo`になったら、そのTaskを実行するためのPaneを生成する。

AgentがTaskを開始した時点で、

```text
todo → doing
```

に更新する。

AgentがTaskを完了したと判断したら、

```text
doing → done
```

に更新する。

Taskが完了したらPaneを削除する。

```text
Task done
    ↓
Pane delete
```

その後、Taskを再び`todo`にした場合は、新しいPaneを生成して再実行する。

---

# 9. FilesystemとHerdrの責務

両者の責務を明確に分離する。

## Filesystem

永続的な情報を保持する。

```text
Filesystem
├── Team definition
├── Member definition
└── Task definition / state
```

## Herdr

Runtimeを提供する。

```text
Herdr
├── Workspace
├── Member Tabs
├── Task Panes
├── Agents
└── Agent communication
```

したがって、

```text
Filesystem = Source of Truth
Herdr      = Runtime
```

とする。

---

# 10. Manager

ManagerはTeam全体のオーケストレーションを担当する。

Manager自身もTeamのMemberであり、Herdr上のTab / Pane / Agentとして存在する。

```text
Workspace
└── manager
    └── Manager Agent
```

Managerは以下を担当する。

1. Humanからの要求を理解する
2. 必要に応じてHumanへヒアリングする
3. 要求をTaskに分解する
4. Taskを作成する
5. Taskの担当Memberを決定する
6. Taskを実行させる
7. Teamの進行状況を監視する
8. 必要に応じてTaskを再編成する
9. 必要に応じてMemberを追加・再編成する
10. プロジェクト全体の意思決定を行う
11. 判断不能な問題をHumanへエスカレーションする

ManagerはTeam内のすべての通信を中継する必要はない。

---

# 11. Managerの初期プロンプト

Team初期化時にManagerへ以下の基本方針を与える。

```text
あなたはこのチームのマネージャです。

チームのメンバーやタスクはこのディレクトリ内ですべて管理されます。

あなたはこのチームを管理する責任があり、私から与えられた指示をタスクに落とし込み、適切なチームメンバーに割り当ててください。

私から与えられた指示が曖昧な場合は、適宜ヒアリングを行い、目的や要件をすり合わせてください。

チームメンバーの構成はあなたが決定し、プロジェクトの目的や必要な役割に応じて適宜再編成してください。

ただし、チームは少数精鋭を基本とし、既存のメンバーを可能な限り再利用してください。

チームメンバー間の協業は許可されています。
メンバーは自身のタスクを遂行するために、必要に応じて他のメンバーへ直接相談、調査依頼、レビュー依頼などを行うことができます。

メンバー間の協業をすべてManagerが中継する必要はありません。

ただし、プロジェクト全体に関わる意思決定、タスクの再編成、チーム構成の変更、判断不能な問題についてはManagerへエスカレーションしてください。

判断できない場合やHumanの承認が必要な場合は、無理に判断せずHumanへ確認してください。
```

---

# 12. Taskの実行

ManagerがTaskを作成し、担当Memberを指定する。

```yaml
---
assignee: developer
status: todo
---
```

その後、ManagerがHerdrを操作してTask用Paneを生成し、Agentを起動する。

概念的には、

```text
Manager
   │
   ├── create task.md
   │
   ├── assignee = developer
   │
   ├── create Pane
   │
   └── herdr agent start
              │
              ▼
         Developer Agent
```

となる。

AgentがTaskを開始したら`status`を`doing`へ変更する。

---

# 13. Memberの基本ルール

Memberは、自分に割り当てられたTaskを遂行する。

Memberは必要に応じて他のMemberと直接協業してよい。

例えば、

```text
Developer → Reviewer
```

としてコードレビューを依頼できる。

また、

```text
Developer → Researcher
```

として技術調査を依頼することもできる。

一方、

```text
Developer → Manager
```

として仕様確認やプロジェクト全体に関わる判断を求めることもできる。

---

# 14. Member間通信

Member間のリアルタイム通信にはHerdrのAgent Promptを利用する。

```text
herdr agent prompt
```

通信はManagerを必ずしも経由しない。

例えばコードレビューでは、

```text
Developer
    │
    │ review request
    ▼
Reviewer
    │
    │ review result
    ▼
Developer
```

という直接通信を許可する。

---

# 15. Member間協業とTaskの違い

Member間の短期的な協業と、独立したTaskは区別する。

## 軽微な協業

Developerが実装中にReviewerへ確認する。

```text
Developer → Reviewer
```

この場合、新しいTaskを必ず作る必要はない。

---

## 独立した作業

正式なコードレビューを実施する必要がある場合は、Reviewer用Taskを作成してもよい。

```text
tasks/
├── 20260913_auth-feature/
│   └── task.md
│
└── 20260913_auth-feature-review/
    └── task.md
```

この場合、

```yaml
---
assignee: reviewer
status: todo
---
```

としてReviewerのTaskとして管理する。

判断基準は、

> 独立した成果物・責任・進捗管理が必要か

とする。

---

# 16. Agentへの質問

MemberがTaskを実行中に疑問を持った場合、質問をそのまま自己判断で処理してはいけない。

質問先を状況に応じて判断する。

### 仕様・プロジェクト方針

```text
Member → Manager
```

### 技術的な相談

```text
Member → Researcher
Member → Developer
Member → Reviewer
```

など、適切なMemberへ直接相談してよい。

Member間で解決できない場合はManagerへエスカレーションする。

---

# 17. Managerへのエスカレーション

以下の場合はManagerへエスカレーションする。

* プロジェクト全体の方針に関係する
* Taskの範囲を変更する必要がある
* 他のTaskとの依存関係に問題がある
* Member間で意見が一致しない
* 自分では判断できない
* 想定外の問題が発生した

Manager自身が判断できない場合はHumanへエスカレーションする。

---

# 18. blocked

`blocked`は、Agentが自力でTaskを継続できない状態を表す。

```text
doing
  ↓
blocked
```

blockedになった場合、Agentは無理に作業を継続しない。

基本的には、

```text
Agent
  ↓
blocked
  ↓
Manager
  ↓
Human
```

としてHumanの介入を待つ。

---

# 19. Manager自身がblockedになった場合

Managerも例外ではない。

Managerが判断不能になった場合は、

```text
Manager
   ↓
blocked
   ↓
Human
```

として停止する。

Managerが判断できない問題を勝手に推測して処理することは禁止する。

---

# 20. Humanへのエスカレーション条件

Humanへのエスカレーションは主に以下の場合に行う。

### Agentがblockedになった

```text
Member
  ↓
blocked
  ↓
Manager
  ↓
Human
```

### Managerがblockedになった

```text
Manager
  ↓
blocked
  ↓
Human
```

### ManagerがHumanの判断を必要とすると判断した

```text
Manager
  ↓
Human approval required
  ↓
Human
```

例えば、

* 重要な仕様決定
* 複数の合理的な選択肢が存在する
* 大きな設計変更
* 破壊的な変更
* プロジェクトの目的そのものに関わる判断

などが該当する。

---

# 21. Taskライフサイクル

Managerは基本的に以下のライフサイクルでTaskを管理する。

```text
Human Request
     │
     ▼
Manager
     │
     │ task decomposition
     ▼
Create task.md
     │
     │ assignee
     ▼
Create Pane
     │
     │ agent start
     ▼
   TODO
     │
     ▼
  DOING
     │
     ├──────────────┐
     │              │
   done           blocked
     │              │
     ▼              ▼
   DONE           Human
     │
     ▼
Delete Pane
```

TaskそのものはFilesystem上に残り続ける。

---

# 22. Teamのライフサイクル

Team自体も動的に変化する。

初期化時：

```text
Team
├── Manager
├── Researcher
├── Developer
└── Reviewer
```

プロジェクトの進行に伴って必要なRoleが増えた場合、

```text
Team
├── Manager
├── Researcher
├── Developer
├── Reviewer
└── Security Specialist
```

のようにMemberを追加することができる。

ただし、Memberの追加は必要最小限にする。

TaskごとにMemberを作成するのではなく、既存Memberを可能な限り再利用する。

---

# 23. Agent Teamの基本原則

このシステムでは、以下の5原則を基本とする。

### 1. Filesystem is the Source of Truth

Team、Member、Taskなどの永続的な情報はFilesystemで管理する。

### 2. Herdr is the Runtime

HerdrはWorkspace、Tab、Pane、Agent、Agent間通信などのRuntimeを提供する。

### 3. Manager is the Orchestrator

ManagerがHumanの要求をTaskへ分解し、Team全体を統括する。

### 4. Member is Persistent, Agent is Ephemeral

Memberは永続的なRoleとして存在し、AgentはTask実行時に一時的に起動する。

### 5. Human is the Final Escalation Point

AgentやManagerが判断できない問題はHumanへエスカレーションする。

---

# 24. 最終的なアーキテクチャ

```text
                         Human
                           │
                           │ request
                           ▼
                    ┌─────────────┐
                    │   Manager   │
                    │    Agent    │
                    └──────┬──────┘
                           │
              task / decision / escalation
                           │
          ┌────────────────┼────────────────┐
          │                │                │
          ▼                ▼                ▼
     Researcher        Developer         Reviewer
        Agent             Agent             Agent
          │                │ ▲                │
          │                │ │                │
          │                └─┼────────────────┘
          │                  │
          └──────────────────┘
             Member collaboration

        ┌────────────────────────────────────┐
        │             Filesystem             │
        │                                    │
        │  members/*.md                      │
        │  tasks/*/task.md                   │
        │                                    │
        │  Source of Truth                   │
        └────────────────────────────────────┘

        ┌────────────────────────────────────┐
        │               Herdr                │
        │                                    │
        │  Workspace = Team                  │
        │  Tab       = Member                │
        │  Pane      = Active Task            │
        │  Agent     = Worker                 │
        │                                    │
        │  Runtime + Communication           │
        └────────────────────────────────────┘
```

---

# 25. 設計思想

このAgent Teamは、専用のTask DB、Message Queue、Agent Orchestratorなどを別途構築することを目的としない。

可能な限り、

```text
Filesystem
+
Herdr
+
Agent Intelligence
```

だけでTeamを成立させる。

特に、Taskの分解、Memberの選択、Member間の協業、Taskの再編成などを固定的なルールや専用Skillで過度に制約せず、Managerおよび各Member Agent自身の判断能力を活用する。

その一方で、

* Taskの永続化
* Taskの状態
* MemberのRole
* Agentのライフサイクル
* blocked時の停止
* Humanへのエスカレーション

といった重要な境界だけは明確に定義する。

これにより、

> **Managerが指揮し、Memberが自律的に協業し、Herdrが実行と通信を担い、Filesystemが状態を永続化する**

というシンプルなAgent Teamを実現する。
