---
name: myherdr
description: Use myherdr with Herdr for project-scoped Codex sessions, approved SSH destinations, and browser terminal handoff to a user.
---

# myherdr

myherdr は Herdr の名前付き session をプロジェクト単位で使い、ローカルの Herdr pane をユーザーのブラウザーへ引き継ぐための CLI と運用規約を提供します。Herdr が提供する workspace、pane、agent の一般操作は Herdr skill に従ってください。

## プロジェクト session

- pane の外からプロジェクト作業を始めるときは、そのプロジェクトのルートで `myherdr start` を使います。既定名は `codex-<project-name>` です。Git repository の場合は Git root が使われます。
- `myherdr start` が既に Herdr pane 内で実行されていると伝えた場合、入れ子で Herdr を起動しません。現在の pane/session を使います。
- ユーザーへ接続コマンドを伝えるときは `myherdr session-name` を実行し、出力された正確な `herdr --session ...` コマンドを使います。
- Herdr が見つからない、または session に接続できないときは、CLI の案内と失敗内容を伝えます。存在しない pane や URL を推測しません。

## SSH 接続前の確認

SSH を起動する前に、作業中 project の `.myherdr/ssh-hosts.md` を読みます。ファイルがない、空、または接続先が明示されていない場合は SSH を実行せず、ユーザーに一覧を準備するよう依頼します。

- 一覧にある接続先名だけを `ssh` に渡します。ワイルドカードを展開したり、一覧にない IP/host へ置き換えたりしません。
- `ProxyJump` など踏み台を使うときは、経由する各 SSH host が一覧にあることを確認します。
- リモート作業では `herdr --remote` を使いません。`ssh <allowed-host>` で接続し、リモート側の Herdr CLI を使います。非対話操作では SSH の remote command として Herdr CLI を実行します。
- リモートの名前付き session は `HERDR_SESSION=<name>` で選びます。リモート側 Codex へ myherdr を導入しません。
- この一覧は Codex が守る運用方針です。myherdr が SSH 接続をネットワーク層で遮断するとは説明しません。鍵、password、token、MFA 情報は project file へ保存しません。

詳しい確認とコマンド例は [`references/ssh-policy.md`](./references/ssh-policy.md) を参照してください。

## 人の入力が必要になったとき

password、host key 確認、MFA、承認など人の入力が必要になったら、対象 terminal への操作を止めます。ユーザーがブラウザーで操作している間、Codex は同じ terminal へ入力・resize・別コマンドを送りません。ユーザーが完了したと伝えるまで待ちます。

1. `myherdr url <session>` を実行して serve の稼働を確認します。serve が停止中という案内が出たら、Herdr skill の手順で専用 pane を用意し、対象プロジェクトのルートで `myherdr serve` をフォアグラウンド起動して開いたままにします。pane の作業場所が違う場合は、起動前にプロジェクトルートへ移動します。
2. `myherdr url <session>` を再実行し、表示された URL をユーザーへ渡します。特定 pane の場合は `--pane <pane-id>` を使います。
3. URL の取得に失敗したら URL を作り上げず、serve/session/pane の失敗理由を伝えます。
4. ブラウザー上で複数 pane が表示された場合、ユーザーに対象を選んでもらいます。Web アプリは pane を作成しません。
5. ユーザーが認証や確認を終えたと連絡するまで対象 terminal へ書き込みません。完了連絡後に状態を読み直して作業を続けます。

`myherdr serve` はローカルでのみ待ち受けます。ローカルの `ssh` 認証 prompt を引き継ぐときは、その SSH が走っているローカル Herdr pane の URL を使います。

## リモート pane のブラウザー接続

`.myherdr/sessions.toml` の URL alias を使い、同じプロジェクトの `.myherdr/ssh-hosts.md` に記載された SSH host へ接続します。リモート Web 接続では SSH agent または鍵を使った非対話認証が必要です。対話認証を求められたら無理に続けず、ローカル pane 上の SSH をユーザーへ引き継ぐか、非対話 SSH の準備を依頼します。

[`references/ssh-policy.md`](./references/ssh-policy.md) に第2段階の設定例があります。
