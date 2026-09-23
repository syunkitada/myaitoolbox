# mygit 利用方法

`mygit`は、複数のGit repositoryをmanifestとlockfileで管理するCLIです。`mygit.yaml`に取得したいrepositoryと要求するrevisionを記述し、通常は`mygit.lock.yaml`に実際に使用するcommitを固定します。

## 前提

- GitがPATHに存在すること
- Git repositoryへアクセスできること（HTTPSまたはSSH）
- Go 1.25以降（ソースから実行・ビルドする場合）

ソースから実行する場合は、`agenttools/mygit`ディレクトリで次のようにします。

```bash
go run ./cmd/mygit status
```

`PATH`から常用できるようにインストールする場合は、次のコマンドを実行します。実行ファイルは`GOBIN`（未設定の場合は`GOPATH/bin`）へ配置されます。

```bash
go install ./cmd/mygit
mygit status
```

利用可能なサブコマンドとオプションは、ヘルプで確認できます。

```bash
mygit --help
mygit status --help
```

ローカルに実行ファイルを置いて使う場合は、次のようにビルドできます。

```bash
go build -o mygit ./cmd/mygit
```

## Workspaceを作成する

`mygit.yaml`があるディレクトリがWorkspaceです。まずWorkspaceを作成し、manifestを記述します。

```bash
mkdir -p ~/workspaces/tools
cd ~/workspaces/tools
```

```yaml
# mygit.yaml
repositories:
  - name: agent-skills
    url: https://github.com/example/agent-skills.git
    revision: main

  - name: code-review
    url: git@github.com:example/code-review.git
    revision: v1.2.0
    path: ./skills/code-review
```

`path`を省略したrepositoryは、`_repos/<name>`へcloneされます。`path`を指定した場合はWorkspaceからの相対パスとして扱います。`~/`で始まるパスはHOMEディレクトリを基準に解決できます。`name`は単一のパス要素であり、`/`や`..`は使用できません。予約領域である`_repos/`配下を明示的な`path`にすることもできません。

`unlock: true`を指定したrepositoryはlockfileへ記録されず、`sync`と`update`のたびに`revision`の最新commitへ更新されます。通常のrepositoryと`unlock: true`のrepositoryは同じWorkspaceで混在できます。

## 基本ワークフロー

### 1. 初回取得

`mygit.lock.yaml`がない場合、`sync`はmanifestのrevisionをremoteで解決してlockfileを作成し、そのcommitで各repositoryをcloneします。初回取得は次のコマンドで実行できます。

```bash
mygit sync
```

lockfileが既にある場合、`sync`は記録されたcommitをそのまま再現します。

初回実行後は、次のような構成になります。

```text
tools/
├── mygit.yaml
├── mygit.lock.yaml
├── .gitignore
├── _repos/
│   └── agent-skills/
└── skills/
    └── code-review/
```

デフォルトclone先の`_repos/`については、既存の`.gitignore`を保持したままmanaged blockが追加されます。明示的な`path`のclone先は自動的に`.gitignore`へ追加されません。

### 2. lockfileの更新

```bash
mygit update
```

`update`はmanifestのrevisionをremoteで再解決し、lockfileを更新します。remoteの新しいrevisionを取り込みたい場合や、manifestのrepository定義・revisionを変更した場合に実行してください。

既存clone先のremote URLとmanifestのURLが異なる場合、`update`は既存cloneのremoteを自動変更・削除せずに失敗します。repositoryのURLを変更した場合は、未コミット・未追跡ファイルを退避または不要であることを確認してから、対象clone先を利用者が削除し、`mygit update`を再実行してcloneを再作成してください。例えばclone先が`_repos/example`の場合は次のようにします。

```bash
rm -rf _repos/example  # 内容を確認してから実行する
mygit update
```

### 3. 固定状態の再現

```bash
mygit sync
```

`sync`はlockfileのcommitをそのまま使用します。remoteのbranchが進んでいても、lockfileを変更したり最新commitへ移動したりしません。

lockfileに存在するrepositoryのURL、revision、clone先がmanifestと一致している必要があります。manifestに追加されたrepositoryがlockfileにない場合は、`sync`がそのrepositoryのrevisionを解決してlockfileへ追加します。lockfileにだけ存在するrepositoryや、既存repositoryの不一致がある場合は失敗します。

repositoryの不一致がある場合は、エラーにmanifestとlockfileの件数、およびlockfileに不足・追加されているrepository名が表示されます。revisionを明示的に更新したい場合は、`mygit update`を実行してください。

### 4. 状態確認

```bash
mygit status
```

`status`はlockfileを変更しません。repositoryごとに、主に次の状態を表示します。

`sync`と`update`は、処理したWorkspaceとrepositoryごとに、revision解決、clone、fetch、checkout、lockfile更新などの内容を標準出力へ表示します。`status`は各repositoryのlockfile・local・remoteの状態を表示します。

| 状態 | 意味 |
| --- | --- |
| `missing` | clone先が存在しない |
| `locked` | local HEADがlockfileのcommitと一致する |
| `dirty` | 未コミット変更または未追跡ファイルがある |
| `drifted` | local HEADがlockfileと異なる |
| `remote-outdated` | remoteのrevisionがlockfileより進んでいる |
| `remote-unavailable` | remoteの状態を確認できない |
| `unlocked` | lockfileでcommitを固定していない |
| `invalid` | lockfile、remote、clone先の構成が不正 |

## lockfileの扱い

`mygit.lock.yaml`はGitで管理してください。`unlock: true`を指定していないrepositoryについて、manifestのrevisionだけではなく、URL、正規化済みclone先、完全なcommit object IDを記録します。`unlock: true`のrepositoryはlockfileに記録されません。

通常は次の使い分けです。

```text
mygit update  # remoteの新しい状態を取り込み、lockfileを更新
mygit sync    # lockfileに記録した状態を再現
mygit status  # 状態を確認するだけ
```

## 安全上の動作

- 既存clone先のremote URLがmanifestと異なる場合は処理しません。
- 未コミット変更や未追跡ファイルがあるclone先では、`reset`や`clean`を実行せず失敗します。
- `.gitignore`対象の未追跡ファイルはdirty判定の対象外です。ただし、checkout先の追跡ファイルとパスが衝突する場合はGitがcheckoutを拒否することがあります。
- clone先がGit repositoryでない場合、既存ファイルを上書きしません。
- 複数Workspace間でclone先が重複または親子関係になる場合は、処理開始前に失敗します。
- `sync`と`update`はカレントディレクトリ配下のWorkspaceを再帰的に処理します。`_repos/`とclone済みの明示的`path`配下は探索しません。

作業中の変更を保存したい場合は、対象repository側でcommitまたはstashを行ってから再実行してください。強制的な破棄オプションは提供していません。

## Nested Workspace

カレントディレクトリ以下に複数の`mygit.yaml`がある場合、それぞれを独立したWorkspaceとして処理します。

```text
root/
├── mygit.yaml
├── engineering/
│   └── mygit.yaml
└── investment/
    └── mygit.yaml
```

この場合、`root`、`root/engineering`、`root/investment`がすべて処理対象です。

## 対応範囲

初期実装で提供するコマンドは`sync`、`update`、`status`です。`list`、`clean`、`diff`は仕様上の将来候補であり、現時点では実装していません。
