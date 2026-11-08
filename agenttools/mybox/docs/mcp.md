# MCP連携

myboxには、内部ブラウザをagentから操作するMCPサーバー`mymcpserve`との連携経路と、CodexのMCP設定を表示する診断機能があります。Codex診断画面自体はMCPサーバーを起動せず、疎通・health checkもしません。

## 内部ブラウザ向けMCP

`mymcpserve`はmyboxのBrowser APIを呼び出してagentにブラウザ操作ツールを提供します。各session操作では`project`と`session_id`を指定します。myboxとMCPサーバーを同一ホストに置く場合はloopbackの`MYBOX_URL`を使えます。remote agent経路ではTLSで保護した認証済み経路と、mybox・mymcpserveで一致する`MYBOX_BROWSER_AGENT_TOKEN`が必要です。tokenはユーザー認証の代替ではありません。

ブラウザの承認、制限、監査動作は[内部ブラウザ文書](./internal-browser.md)に従います。MCP経由でもこれらのブラウザ制御を迂回するものではありません。

## Codexが使うMCPサーバーの診断

AgentのCodex Diagnostics画面はCodex CLIの設定情報からMCPサーバー名、scope、source、transport、command、status、auth status等を表示します。これは設定状態の読み取りであり、サーバープロセスを起動したり接続確認したりする機能ではありません。診断画面の説明は[利用ガイド](./user-guide.md#機能)を参照してください。

## 関連箇所

- [`../internal/entrypoint/codex.go`](../internal/entrypoint/codex.go) — Codex MCP構成の読み取り。
- [`../openapi.yaml`](../openapi.yaml) — Codex診断APIの契約。
- [利用ガイド](./user-guide.md#内部ブラウザの認証リバースプロキシ方針) — URL、agent token、公開時の運用設定。
