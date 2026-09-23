# `mygit/`

複数のGit repositoryをmanifestとlockfileで再現可能に管理する`mygit`の仕様と実装をまとめるディレクトリです。

Repositoryに`unlock: true`を指定すると、そのRepositoryはlockfileへ記録せず、`sync`・`update`の実行時に`revision`を再解決します。

## 簡単な使い方

このディレクトリで実行ファイルをインストールします。

```bash
go install ./cmd/mygit
```

次に、管理対象Workspaceで`mygit.yaml`を作成します。`name`を省略すると、URLのユーザー名または組織名とrepository名を`_`で連結した名前が`_repos/`配下に使われます。

```yaml
repositories:
  - url: https://github.com/yennanliu/InvestSkill.git
    revision: main
```

Workspaceでは、まず`sync`を実行します。`mygit.lock.yaml`が既に存在する場合はその状態を再現し、lockfileがない場合はmanifestのrevisionを解決してlockfileを作成してからcloneします。manifestに追加されたrepositoryがlockfileにない場合も、そのrepositoryだけを解決してlockfileへ追加します。

```bash
mygit sync    # lockfileの状態を再現（初回はlockfileも作成）
mygit status  # 状態を確認
```

remoteの新しいrevisionを取り込みたいとき、またはmanifestのrepository定義やrevisionを変更したときは`update`を実行します。その後は生成・更新されたlockfileを共有して`sync`を実行できます。repositoryのURLを変更した場合は、既存cloneのremoteを自動変更せず`update`が失敗するため、既存cloneを確認のうえ削除してから`update`を再実行します。

```bash
mygit update  # remoteを解決してlockfileを更新
mygit sync    # lockfileの状態を再現
mygit status  # 状態を確認
```

この例では、repositoryは`_repos/yennanliu_InvestSkill`に保存されます。manifest、lockfile、clone先の詳細は[`docs/usage.md`](./docs/usage.md)を参照してください。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | `mygit`固有の開発手順と完了時の検証コマンド。 |
| [`.golangci.yml`](./.golangci.yml) | `golangci-lint`の静的解析設定。 |
| [`README.md`](./README.md) | `mygit`実装と仕様の入口。 |
| [`cmd/`](./cmd/) | `mygit` CLIのエントリポイント。詳細は `cmd/README.md` を参照。 |
| [`docs/`](./docs/) | `mygit`の仕様書と利用方法。詳細は `docs/README.md` を参照。 |
| [`internal/`](./internal/) | manifest、Workspace探索、Git操作、各コマンドの実装。詳細は `internal/README.md` を参照。 |
| [`go.mod`](./go.mod) | Goモジュール定義。 |
| [`go.sum`](./go.sum) | Go依存関係のチェックサム。 |
