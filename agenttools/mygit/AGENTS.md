# `mygit` 開発ガイドライン

このディレクトリでは、複数のGit repositoryをmanifestとlockfileで再現可能に管理する`mygit`を開発します。

## 実装時の確認

- 仕様を変更した場合は、`docs/`の仕様書・利用方法と、関係するREADME.mdを同じ変更で更新する。
- Goコードは`gofmt`で整形し、テストは一時ディレクトリまたはローカルリポジトリを使って外部環境に依存させない。
- ファイルやディレクトリを追加・削除・移動した場合は、`README.md`のIndexと相対リンクを更新する。
- 既存のユーザーデータや作業ツリーを壊す操作（広範囲の削除、無確認のresetなど）は行わない。

## 開発完了時の必須確認

開発を完了する前に、`agenttools/mygit`で次のコマンドを順番に実行する。

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
go install ./cmd/mygit
```

`golangci-lint`は、初回またはバージョン更新時に次のコマンドでインストールする。バージョンを固定して、開発者間で検証結果を揃える。

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

`go vet ./...`と`golangci-lint run ./...`を静的解析として実行する。`golangci-lint`の設定は[`.golangci.yml`](./.golangci.yml)で管理する。

`go install ./cmd/mygit`が成功したことを確認してから、開発完了とする。インストール先は環境の`GOBIN`または`GOPATH/bin`に従う。
