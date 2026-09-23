# mygit 仕様書

この文書は`mygit`のデータモデル、Workspace探索、manifest、lockfile、コマンド仕様を定義する。実際の導入手順とCLIの利用例は[`usage.md`](./usage.md)を参照する。

## 1. 概要

`mygit` は、複数の Git リポジトリを **manifest + lockfile** で管理するためのシンプルな Git リポジトリ管理ツールである。

主な用途は、複数の Git リポジトリからなるツール・資料・Agent Skill・プロジェクトなどを、再現可能な状態でまとめて取得・更新することである。

ただし、`mygit` 自体は **Knowledge / Skill / Reference などの意味論を持たない**。

`mygit` が扱うのは、あくまで以下の情報だけである。

* Git repository
* repository URL
* revision
* resolved commit
* clone先

---

# 2. 基本モデル

`mygit` は以下のファイル・ディレクトリを基本単位とする。

```text
Workspace/
├── mygit.yaml
├── mygit.lock.yaml
├── .gitignore
└── _repos/
    ├── repository-a/
    ├── repository-b/
    └── repository-c/
```

| 要素                | 役割                             |
| ----------------- | ------------------------------ |
| `mygit.yaml`      | ユーザーが管理するリポジトリ定義               |
| `mygit.lock.yaml` | 解決済みcommitを固定するlockfile        |
| `_repos/`          | デフォルトのclone先                   |
| `.gitignore`      | デフォルトclone先 `_repos/` を親Gitから除外 |

---

# 3. Workspace

## 3.1 Workspaceの定義

`mygit.yaml` が存在するディレクトリを `mygit` の **Workspace** とする。

例えば、

```text
root/
├── mygit.yaml
├── engineering/
│   └── mygit.yaml
└── investment/
    └── mygit.yaml
```

の場合、3つのWorkspaceが存在する。

```text
root/
root/engineering/
root/investment/
```

各Workspaceは独立して管理される。

---

# 4. Workspaceの探索

`mygit` はコマンド実行時の **カレントディレクトリを探索ルート**として、配下を再帰的に探索する。

```text
current/
├── mygit.yaml
├── engineering/
│   └── mygit.yaml
└── investment/
    └── mygit.yaml
```

この場合、3つのWorkspaceを検出する。

## 4.1 探索対象外

以下のディレクトリは再帰探索から除外する。

```text
.git/
_repos/
```

### `.git/`

Git内部データであるため探索しない。

### `_repos/`

`mygit` がcloneしたGit repositoryの中に、偶然 `mygit.yaml` が存在する可能性がある。

例えば、

```text
workspace/
├── mygit.yaml
└── _repos/
    └── some-repository/
        └── mygit.yaml
```

の場合、

```text
workspace/_repos/
some-repository/
```

をWorkspaceとして認識してはいけない。

そのため `_repos/` は探索時にpruneする。

`path` が明示されたRepositoryのclone先も、clone先が存在する場合は再帰探索から除外する。clone先がまだ存在しない場合は、manifestを読み込んだ時点で解決したclone先を探索対象から除外する。

`.git/` はGit内部データとして常にpruneするが、`.git` を持つディレクトリ全体を一律に除外してはいけない。Git管理下にあるNested Workspaceは有効なためである。`path` で指定されたclone先は、manifestから解決した除外対象として扱い、その内部だけを探索しない。

---

# 5. Nested Workspace

Workspaceはネスト可能とする。

```text
root/
├── mygit.yaml
├── engineering/
│   ├── mygit.yaml
│   └── tools/
│       └── mygit.yaml
└── investment/
    └── mygit.yaml
```

これらはそれぞれ独立したWorkspaceとして扱う。

一方、以下はWorkspaceとして扱わない。

```text
root/
├── mygit.yaml
└── _repos/
    └── repository/
        └── mygit.yaml
```

`_repos/` 以下は探索対象外だからである。

---

# 6. mygit.yaml

`mygit.yaml` はユーザーが管理するmanifestである。

基本形式：

```yaml
repositories:
  - name: agent-skills
    url: https://github.com/example/agent-skills.git
    revision: main

  - name: code-review
    url: https://github.com/example/code-review.git
    revision: main

  - name: go-tools
    url: https://github.com/example/go-tools.git
    revision: v1.2.0
```

---

# 7. Repository定義

Repositoryは以下の情報を持つ。

| 項目         |  必須 | 説明                        |
| ---------- | --: | ------------------------- |
| `name`     |  No | Repository名。省略時はURLから自動導出 |
| `url`      | Yes | Git repository URL        |
| `revision` | Yes | branch / tag / commit SHA |
| `path`     |  No | clone先                    |

`name` を指定した場合は、その値をRepositoryの識別子として使用する。空文字、`.`、`..`、`/`、空白、制御文字を含む値は指定できず、単一のパス要素でなければならない。同一Workspace内で重複する `name` は許可しない。

`name` を省略した場合、URLのnamespaceとrepository名から `<namespace>_<repository>` を自動導出する。例えば、`https://github.com/yennanliu/InvestSkill.git` は `yennanliu_InvestSkill` となる。`.git`、末尾スラッシュ、URLエスケープは正規化してから導出する。階層namespaceは各要素を `_` で連結する。

HTTPS、HTTP、SSHなどhostとnamespaceを識別できるURLでは自動導出できる。`file://` URL、ローカルパス、namespaceを含まないURLでは `name` を明示しなければならない。自動導出された値はmanifestを読み込んだ時点で確定し、lockfileの `name` とclone先に保存する。

`path` はWorkspaceを基準とする相対パスとする。絶対パス、Workspace自身をclone先とする指定、予約領域 `_repos/` 配下への明示的な指定、正規化後に同一となる指定は許可しない。`..` によるWorkspace外の指定は許可するが、すべてのWorkspaceで解決したclone先について、重複・親子関係・symlink経由の衝突を検出した場合は実行前に失敗させる。

---

# 8. revision

`revision` には以下を指定できる。

### Branch

```yaml
revision: main
```

### Tag

```yaml
revision: v1.2.0
```

### Commit SHA

```yaml
revision: 8f3a91c2...
```

短いrevisionは、remote上でbranchとtagのいずれか一方に一意に解決できる場合だけ許可する。branchとtagが同名の場合はエラーとし、必要に応じて `refs/heads/<name>` または `refs/tags/<name>` の完全なref名を指定する。annotated tagはcommitへ解決し、commit以外のGit objectは拒否する。

`revision` はユーザーが指定する **要求状態** である。

一方、実際に再現可能な状態は `mygit.lock.yaml` の `commit` で管理する。

---

# 9. Clone先

## 9.1 デフォルト

`path` を指定しない場合、clone先はWorkspace配下の `_repos/` とする。

```yaml
repositories:
  - name: agent-skills
    url: https://github.com/example/agent-skills.git
    revision: main
```

clone先：

```text
Workspace/
└── _repos/
    └── agent-skills/
```

つまり、

```text
_repos/<name>
```

がデフォルトclone先となる。

---

# 10. Clone先のカスタマイズ

Repositoryごとに `path` を指定することで、clone先を変更できる。

```yaml
repositories:
  - name: code-review
    url: https://github.com/example/code-review.git
    revision: main
    path: ./skills/code-review
```

この場合、

```text
Workspace/
├── mygit.yaml
└── skills/
    └── code-review/
```

にcloneされる。

`path` は **Workspaceを基準とした相対パス**として扱う。

---

# 11. Clone先の例

```yaml
repositories:
  # デフォルト
  - name: agent-skills
    url: https://github.com/example/agent-skills.git
    revision: main

  # Workspaceからの相対パス
  - name: code-review
    url: https://github.com/example/code-review.git
    revision: main
    path: ./skills/code-review

  # Workspaceの親ディレクトリ
  - name: shared-tools
    url: https://github.com/example/shared-tools.git
    revision: v1.2.0
    path: ../shared/tools
```

結果：

```text
Workspace/
├── mygit.yaml
├── mygit.lock.yaml
├── .gitignore
├── _repos/
│   └── agent-skills/
└── skills/
    └── code-review/

../shared/
└── tools/
```

---

# 12. `.gitignore` の扱い

`mygit` はデフォルトclone先である `_repos/` を、Workspaceを管理しているGit repositoryから除外する。

推奨するmanaged block：

```gitignore
# mygit:start
/_repos/
# mygit:end
```

既存の `.gitignore` を上書きしてはいけない。

既存の内容を維持したうえで、必要な場合のみmanaged blockを追加する。

---

# 13. path指定時の `.gitignore`

`path` が明示的に指定されたRepositoryについては、`mygit` は `.gitignore` を管理しない。

例えば、

```yaml
repositories:
  - name: code-review
    url: https://github.com/example/code-review.git
    revision: main
    path: ./skills/code-review
```

の場合、

```text
skills/code-review/
```

を `.gitignore` に追加しない。

また、`mygit` は以下の操作も行わない。

* `path` のclone先を `.gitignore` に追加
* `path` のclone先を `.gitignore` から削除
* `path` のために既存の `.gitignore` の内容を変更

Workspace内にpath未指定のRepositoryがある場合は、`_repos/` のmanaged blockだけを独立して管理する。明示的な `path` のために、他の除外ルールを変更することはない。

つまり、`.gitignore` の自動管理対象は **デフォルトclone先である `_repos/` のみ**とする。

---

# 14. mygit.lock.yaml

`mygit.lock.yaml` は `mygit` が生成・更新するlockfileである。

例えば、

```yaml
repositories:
  - name: agent-skills
    url: https://github.com/example/agent-skills.git
    revision: main
    path: _repos/agent-skills
    commit: 8f3a91c2abcdef1234567890abcdef1234567890

  - name: code-review
    url: https://github.com/example/code-review.git
    revision: main
    path: _repos/code-review
    commit: 71ab82deabcdef1234567890abcdef1234567890
```

`path` はmanifestの指定をWorkspace基準で正規化した値であり、未指定の場合は `_repos/<name>` を保存する。`commit` は省略形ではなく、Repositoryのobject formatに応じた完全なobject IDを保存する。

---

# 15. revisionとcommit

`revision` と `commit` は役割が異なる。

```text
mygit.yaml

revision: main
        │
        │ resolve
        ▼
mygit.lock.yaml

commit: 8f3a91c2...
```

### revision

ユーザーが指定した要求。

例：

```yaml
revision: main
```

### commit

実際に使用するGit commit。

例：

```yaml
commit: 8f3a91c2...
```

そのため、`main` が将来更新されても、lockfileが変わらない限り同じcommitを再現できる。

---

# 16. sync

```bash
mygit sync
```

`sync` は **lockfileに記録された状態を再現するコマンド**である。lockfileがまだ存在しない初回実行時は、manifestのrevisionをremoteから解決してlockfileを作成し、その状態を再現する。manifestに追加されたRepositoryがlockfileにない場合も、そのRepositoryだけをremoteから解決してlockfileに追加する。

lockfileに存在するRepositoryについては、`sync` の開始時にmanifestとlockfileの各Repositoryの `name`、`url`、`revision`、`path` が一致することを検証する。lockfileにだけ存在するRepositoryや既存Repositoryの不一致がある場合は、clone先やlockfileを変更せずに失敗する。既存Repositoryのrevisionを明示的に更新する操作は `update` が行う。

基本的には、

```text
mygit.lock.yaml
       │
       ▼
clone / fetch
       │
       ▼
locked commit
```

という動作を行う。

`sync` は、remoteの最新revisionを自動的に採用してはいけない。

clone先が存在する場合はGit repositoryであることとremote URLが一致することを確認する。未コミット変更、未追跡ファイルとの衝突、異なるremote、またはlocked commitへ安全に移動できない状態がある場合は、resetやcleanを行わずに失敗する。

例えば、

```yaml
revision: main
commit: abc123
```

となっている場合、remoteの `main` が

```text
def456
```

に進んでいても、`sync` は `abc123` を使用する。

---

# 17. update

```bash
mygit update
```

`update` は `mygit.yaml` の `revision` をremoteから再解決し、lockfileを更新する。

既存clone先のremote URLがmanifestのURLと異なる場合、`update`は既存cloneのremoteを自動変更・削除せずに失敗する。repositoryのURLを変更する場合は、利用者が既存clone先の内容を確認して削除した後、`update`を再実行してcloneを再作成する。

`update` は全Repositoryの解決に成功した後、lockfileを一時ファイル経由で原子的に置き換える。lockfileの置き換えまたはlocal repositoryの更新に失敗した場合はエラーを返し、resetやcleanによる自動ロールバックは行わない。lockfileが存在しない場合も `update` が新規作成する。

例えば、

```yaml
# mygit.yaml
revision: main
```

現在のremote：

```text
main -> def456
```

の場合、

```yaml
# mygit.lock.yaml
revision: main
path: _repos/agent-skills
commit: def456
```

へ更新する。

概念的には、

```text
mygit.yaml
   │
   │ resolve remote revision
   ▼
latest commit
   │
   ▼
mygit.lock.yaml
   │
   ▼
local repository
```

となる。

---

# 18. syncとupdateの違い

| コマンド     | remoteの最新状態を確認 | lock更新 | local更新 |
| -------- | -------------: | -----: | ------: |
| `sync`   |       必要に応じて取得 |     No |     Yes |
| `update` |            Yes |    Yes |     Yes |
| `status` |            Yes |     No |      No |

重要なのは、

> `sync` は再現、`update` は更新

という役割分担である。

---

# 19. status

```bash
mygit status
```

`status` は現在の状態を確認するためのコマンドとする。

少なくとも `missing`（clone先なし）、`locked`（locked commitと一致）、`dirty`（未コミット変更あり）、`drifted`（別commit）、`remote-outdated`（remoteのrevisionが更新済み）、`remote-unavailable`（remote確認不能）、`invalid`（remoteまたはlockfile不一致）を区別して表示する。`status` はremote確認に失敗した場合も、localで確認できた状態と失敗理由を表示し、lockfileを変更しない。

例えば、

```text
agent-skills
  revision: main
  locked:   8f3a91c2
  remote:   71ab82de
  status:   remote-outdated

code-review
  revision: v1.2.0
  locked:   abc12345
  remote:   abc12345
  status:   locked
```

のように表示できる。

ただし、`status` はlockfileを変更しない。

---

# 20. Repositoryの識別

Workspace内のRepositoryは `name` によって識別する。

例えば、

```yaml
repositories:
  - name: agent-skills
    ...
  - name: code-review
    ...
```

の場合、

```text
_repos/agent-skills
_repos/code-review
```

となる。

同一Workspace内で `name` が重複することは許可しない。

---

# 21. Repository URL

`url` はGit repositoryを取得するためのURLである。

例：

```yaml
url: https://github.com/example/project.git
```

SSH URLも利用可能とする。

```yaml
url: git@github.com:example/project.git
```

`mygit` はRepositoryのホスティングサービスについて特別な意味を持たない。

Gitとしてclone/fetchできればよい。

---

# 22. Git-only設計

`mygit` はGit repository管理だけを担当する。

例えば、

```text
engineering/
investment/
knowledge/
skills/
references/
```

などのディレクトリ名や意味を認識しない。

また、

* Agent Skill
* Knowledge
* Reference
* Document
* Source
* Investment
* Engineering

などの概念を`mygit`に組み込まない。

例えば、

```text
engineering/mygit.yaml
investment/mygit.yaml
```

があっても、`mygit`から見ると単なる2つのWorkspaceである。

---

# 23. 他の収集ツールとの分離

将来的にGit以外のReferenceを扱う必要が生じた場合、それは別のツールで管理する。

例えば、

```text
mygit
  └── Git repository

myweb
  └── Web resources

myarchive
  └── Archive resources
```

のように役割を分離する。

`mygit` にGit以外のリソース管理機能を追加しない。

---

# 24. ディレクトリ例

例えば以下の構成を考える。

```text
project/
├── mygit.yaml
├── mygit.lock.yaml
├── .gitignore
├── _repos/
│   ├── agent-skills/
│   └── code-review/
│
├── engineering/
│   ├── mygit.yaml
│   └── _repos/
│       └── go-tools/
│
└── investment/
    ├── mygit.yaml
    └── skills/
        └── investment-skills/
```

この場合、

```text
project/
project/engineering/
project/investment/
```

が3つの独立したWorkspaceとなる。

各Workspaceはそれぞれ、

```text
mygit.yaml
mygit.lock.yaml
_repos/
```

を独立して管理する。

`investment/skills/investment-skills/` は `path` によって指定されたclone先なので、`mygit` はこのディレクトリを `.gitignore` 管理しない。

---

# 25. コマンド

初期バージョンでは以下を基本コマンドとする。

```bash
mygit sync
mygit update
mygit status
```

## sync

```bash
mygit sync
```

lockfileの状態を再現する。lockfileがない初回実行では、manifestのrevisionを解決してlockfileを作成してから再現する。manifestに追加されたRepositoryがlockfileにない場合は、そのRepositoryだけを解決してlockfileへ追加する。

## update

```bash
mygit update
```

remoteのrevisionを再解決してlockfileを更新する。

## status

```bash
mygit status
```

現在の状態を確認する。

---

# 26. 将来的なコマンド

必要になった場合、以下を追加できる。

```bash
mygit list
mygit clean
mygit diff
```

ただし、初期実装では必須としない。

---

# 27. 設計原則

`mygit` は以下の原則で設計する。

### 1. Gitに集中する

`mygit` はGit repositoryの管理だけを行う。

### 2. ManifestとLockを分離する

```text
mygit.yaml
  = ユーザーの要求

mygit.lock.yaml
  = 再現可能な実体
```

とする。

### 3. syncとupdateを分離する

```text
sync
  = 再現

update
  = 更新
```

とする。

### 4. デフォルトはシンプルにする

clone先を指定しない場合は、

```text
_repos/<name>
```

とする。

### 5. 必要な場合だけclone先を変更できる

```yaml
path: ./somewhere
```

を指定することでclone先を変更できる。

### 6. 自動生成領域を明確にする

デフォルトでは、

```text
_repos/
```

を`mygit`管理下の生成領域とする。

### 7. 明示的なpathはユーザー管理とする

`path` を指定した場合、その場所について`mygit`は`.gitignore`を管理しない。

### 8. Workspaceを独立させる

各`mygit.yaml`を独立したWorkspaceとして扱う。

### 9. cloneされたRepositoryをWorkspaceとして誤認しない

探索時には、

```text
.git/
_repos/
```

をpruneする。

---

# 28. 全体像

最終的な`mygit`のモデルは以下のようになる。

```text
                    current directory
                           │
                           ▼
                  recursive discovery
                           │
             ┌─────────────┼─────────────┐
             ▼             ▼             ▼
        Workspace A   Workspace B   Workspace C
             │             │             │
       mygit.yaml     mygit.yaml     mygit.yaml
             │             │             │
             ▼             ▼             ▼
          resolve        resolve        resolve
             │             │             │
             ▼             ▼             ▼
       lockfile A     lockfile B     lockfile C
             │             │             │
             ▼             ▼             ▼
          _repos/        _repos/        _repos/
```

Repository単位では、

```text
mygit.yaml
     │
     │ revision
     ▼
 remote Git repository
     │
     │ resolve
     ▼
 commit SHA
     │
     ▼
mygit.lock.yaml
     │
     │ sync
     ▼
 clone destination
```

となる。

Clone先は、

```text
path未指定
    ↓
Workspace/_repos/<name>
```

または、

```text
path指定あり
    ↓
Workspace/<path>
```

である。

`.gitignore` の自動管理対象は、**path未指定の場合の `_repos/` のみ**とする。

---

# 29. 最小構成

最小限のWorkspaceは以下で成立する。

```text
workspace/
├── mygit.yaml
└── mygit.lock.yaml
```

`mygit` 実行時に、

```text
workspace/
├── mygit.yaml
├── mygit.lock.yaml
├── .gitignore
└── _repos/
    └── repository/
```

へ展開される。

つまり、ユーザーが管理するものは基本的に、

```text
mygit.yaml
mygit.lock.yaml
```

の2つであり、cloneされたRepositoryは生成物として扱う。

---

# 30. まとめ

`mygit` は、

> **複数のGit repositoryをmanifestとlockfileで再現可能に管理するためのGit専用ツール**

とする。

基本形は、

```text
mygit.yaml
mygit.lock.yaml
_repos/
```

である。

Repositoryのclone先は、

```yaml
# デフォルト
- name: foo
  url: ...
  revision: main
```

なら、

```text
_repos/foo/
```

となる。

明示的に、

```yaml
- name: foo
  url: ...
  revision: main
  path: ./somewhere/foo
```

とすれば、

```text
somewhere/foo/
```

にcloneされる。

この場合、`mygit` はそのclone先を`.gitignore`の管理対象としない。

この設計によって、`mygit` はKnowledgeやAgent Skillなどの上位概念を持たず、**「Git repositoryをどこに、どのrevisionで取得し、どのcommitに固定するか」だけを管理するシンプルなツール**として独立できる。
