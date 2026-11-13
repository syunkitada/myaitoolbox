# SSH policy とユーザー引き継ぎ

## 許可 host の読み取り

作業対象 project のルートから `.myherdr/ssh-hosts.md` を開き、SSH 接続を実行する前に接続先を確認します。ファイルが存在しない、host の記載がない、または接続先名が一覧にない場合は接続しません。ユーザーに対象 host を一件ずつ追記してもらいます。

```markdown
# このプロジェクトで Codex が接続してよい SSH ホスト

- `devbox`
- `staging`
```

wildcard は使用しません。SSH 設定の `ProxyJump`、`-J`、多段接続を使う場合は踏み台と最終接続先の両方を一覧で確認します。この判断は skill が適用する運用ルールであり、myherdr は通常の `ssh` コマンドを遮断しません。

## リモート Herdr CLI

リモート操作は SSH 先の `herdr` CLI に任せます。`herdr --remote` は使いません。対話接続の例:

```sh
ssh devbox
HERDR_SESSION=codex-project herdr agent list
HERDR_SESSION=codex-project herdr pane read w1:p1
```

非対話操作は `ssh devbox env HERDR_SESSION=codex-project herdr pane list --workspace w1` のように実行できます。実際に渡す host と session は allow-list とプロジェクトの設定を確認してから決めます。鍵や認証情報をコマンド引数、ログ、project ファイルへ書きません。

## ブラウザーでローカル pane を引き継ぐ

人の入力が必要なときは、Codex が端末へ入力するのを止め、対象 pane を特定します。`myherdr serve` が専用 Herdr pane で起動中であることを確認して `myherdr url <session> --pane <pane-id>` を実行し、CLI が返した URL をそのまま提示します。

ユーザーがブラウザーで端末を操作している間、Codex は terminal stream へ追加 input を送信しません。完了連絡後に、Herdr pane の状態を読み直してから再開します。serve 起動または URL 取得が失敗した場合は実際のエラーを伝え、URL を推測しません。

## リモート pane をブラウザーで引き継ぐ

リモート Web 接続の alias は project root の `.myherdr/sessions.toml` に置き、同じ host を `.myherdr/ssh-hosts.md` にも明示します。

```toml
[sessions.remote-codex]
host = "devbox"
herdr_session = "codex-project"
```

この場合は `myherdr url remote-codex` を使います。myherdr は `host` が許可一覧にあることを確認し、SSH batch mode でリモート Herdr stream を中継します。SSH agent または鍵で非対話認証できない場合は接続を開始できません。対話認証が必要ならローカル session 内の SSH prompt をユーザーへ引き継ぎます。
