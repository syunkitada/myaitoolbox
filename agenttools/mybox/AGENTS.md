# `mybox` 開発ガイドライン

このディレクトリでは、ローカルプロジェクトのタスク・ファイル・Git・AIエージェントなどを扱うCLI・Web UIワークスペースツール`mybox`を開発します。

## README.mdの役割

各ディレクトリの `README.md` は、そのディレクトリの目的・使い方・構成を説明する入口です。README.md は、対象ディレクトリ全体と、配下のディレクトリおよびファイルについて説明責任を持ちます。

- ルートの [README.md](./README.md) はリポジトリ全体のIndexとし、`## Index` にトップレベルのディレクトリとファイルを掲載する。
- 各ディレクトリのREADME.mdには、直接の子ディレクトリとファイルの役割を記載する。子ディレクトリにREADME.mdがある場合はリンクし、詳細な説明はそのREADME.mdに委譲する。
- `archives/`、`inbox/`, `_inbox/`、`tasks/`とその配下には、README.mdを必須としない。
- README.mdから参照するパスは、原則として対象README.mdからの相対リンクにする。
- Indexを作成するときは、Git管理対象のファイルとディレクトリだけを掲載し、Git管理対象外のものは除外する。
- ディレクトリまたはファイルを追加・削除・移動・改名した場合は、同じ変更で関係するREADME.mdの構成説明・Index・リンクも更新する。
- 既存のディレクトリやファイルの役割・使い方が変わった場合も、関係するREADME.mdを更新する。
- 生成物やローカル専用ファイルをIndexの対象外にする場合は、README.mdまたは`.gitignore`でその扱いが分かるようにする。

## Indexの記載例

直接の子ディレクトリとGit管理対象のファイルを掲載します。

```markdown
## Index

| パス | 役割 |
| --- | --- |
| [`src/`](./src/) | アプリケーション本体。詳細は `src/README.md` を参照 |
| [`config.toml`](./config.toml) | 設定ファイル |
```

`tmp/`、`.env`、生成物、未追跡ファイルなど、Git管理対象外のものはIndexに掲載しません。

## 変更時の確認

変更前後でディレクトリ構造とファイル一覧を確認し、README.mdの記載漏れ・リンク切れ・古い説明がないことを確認する。新しいディレクトリを追加する場合は、必要に応じてそのディレクトリにもREADME.mdを追加する。

## 実装時の確認

- 仕様や利用方法を変更した場合は、関係するREADME.md、存在する場合は`docs/`内のファイルと、API仕様を同じ変更で更新する。
- Goコードは`gofmt`で整形し、Web UIのコードは既存のnpm scriptsとプロジェクトの書式に従う。
- Goテストは一時ディレクトリまたはローカルリポジトリを使い、Web UIテストも外部環境に依存させない。
- `openapi.yaml`を変更した場合は、生成コードを直接編集せず`make generate`で再生成する。
- ファイルやディレクトリを追加・削除・移動・改名した場合は、README.mdのIndexと相対リンクを更新する。
- `internal/`の依存方向は、EntrypointからApplication・Domain・Infrastructure、ApplicationからDomain、InfrastructureからDomainの内向き依存を基本とする。DomainやApplicationからInfrastructureへ依存させない。
- 既存のユーザーデータや作業ツリーを壊す操作（広範囲の削除、無確認のresetなど）は行わない。

## 生成物とWeb UI

- `internal/entrypoint/api/`の生成コードは[`openapi.yaml`](./openapi.yaml)から生成されるため、生成ファイルを直接編集しない。
- `internal/webui/dist/`は`web/`のビルド成果物を置くディレクトリで、実体は生成物として扱う。
- `node_modules/`、`test-results/`など既存のGit管理下にある生成物は、機能追加やドキュメント更新で編集しない。

## 開発完了時の必須確認

開発を完了する前に、このリポジトリのルートで次のコマンドを順番に実行する。Web UIをバイナリへ同梱するため、`make web-build`を先に実行する。

```bash
make web-build
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
go install ./cmd/mybox
```

Web UIの変更を含む場合は、次の確認も実行する。

```bash
cd web && npm test
```

CLIまたはWeb UIのE2Eに関係する変更では、必要に応じて次も実行する。

```bash
make e2e
```

`golangci-lint`は、初回またはバージョン更新時に次のコマンドでインストールする。バージョンを固定して、開発者間で検証結果を揃える。

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

`go vet ./...`と`golangci-lint run ./...`を静的解析として実行する。`golangci-lint`の設定は[`.golangci.yml`](./.golangci.yml)で管理する。

`go install ./cmd/mybox`が成功したことを確認してから、開発完了とする。インストール先は環境の`GOBIN`または`GOPATH/bin`に従う。

## AGENTS.mdのルール追加

AGENTS.md自体への直接的な指示ではない内容について、AGENTS.mdに新しいルールを追加すべきだと判断した場合は、追加前にユーザーへ相談し、合意を得たうえでルール化する。
