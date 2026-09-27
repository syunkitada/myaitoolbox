# `myntfy`

ntfyを使って通知を送信・購読するCLIです。topicは推測困難なランダム値としてローカルに保存し、送信時に毎回指定せずに利用できます。

## 使い方

このディレクトリでインストールします。

```bash
go install ./cmd/myntfy
```

初回はtopicを生成します。

```bash
myntfy topic regenerate --prefix server01
myntfy topic show
```

`--prefix`にはサーバー名など送信元を識別できる値を指定します。prefixは英数字・`_`・`-`のみで、最大31文字です。生成されるtopicは`<prefix>-<ランダム32桁hex>`形式になります。

prefixに`.`や`/`などの利用できない文字が含まれる場合は、該当文字を`-`へ置換する確認を表示します。`y`または`yes`で置換して生成し、それ以外の入力ではキャンセルします。

`topic show`で表示したtopicをiPhoneのntfyアプリで購読すると、通知を受け取れます。

通知の送信:

```bash
myntfy send "バックアップが完了しました"
myntfy send --title "Deploy" --priority high --tags tada,deploy "デプロイが完了しました"
```

CLIで同じtopicのイベントを購読する場合は、JSON Linesのストリームを出力します。

```bash
myntfy subscribe
```

topicを変更する場合は、`regenerate`後にiPhoneアプリ側でも新しいtopicを購読し直してください。旧topicをntfy側で無効化する操作ではありません。

## 設定

- topicは既定で`$XDG_CONFIG_HOME/myntfy/topic`（またはOSのユーザー設定ディレクトリ配下）に保存します。
- `MYNTFY_CONFIG`でtopicファイルのパスを変更できます。
- 接続先の既定値は`https://ntfy.sh`です。`MYNTFY_SERVER`または各コマンドの`--server`で変更できます。
- `--server`は`MYNTFY_SERVER`より優先されます。
- topicファイルは`0600`、親ディレクトリは新規作成時に`0700`で保存します。

送信はntfyの[Publishing API](https://docs.ntfy.sh/publish/)、購読は[Subscribe via API](https://docs.ntfy.sh/subscribe/api/)のJSONストリームを利用します。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | `myntfy`固有の開発手順と完了時の検証コマンド。 |
| [`.gitignore`](./.gitignore) | ローカルビルドで生成する`myntfy`バイナリの除外設定。 |
| [`.golangci.yml`](./.golangci.yml) | `golangci-lint`の静的解析設定。 |
| [`Makefile`](./Makefile) | ビルド、テスト、lint用コマンド。 |
| [`cmd/`](./cmd/) | `myntfy` CLIのエントリポイント。詳細は`cmd/README.md`を参照。 |
| [`internal/`](./internal/) | topic管理とntfy HTTPクライアント。詳細は`internal/README.md`を参照。 |
| [`go.mod`](./go.mod) | Goモジュールと依存関係の定義。 |
| [`go.sum`](./go.sum) | Go依存関係のチェックサム。 |
| [`README.md`](./README.md) | `myntfy`の使い方と構成。本文書。 |
