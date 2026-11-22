# 内部ブラウザ

内部ブラウザは、プロジェクトごとのPlaywright/ChromiumセッションをWeb UIとAI agentが共有する機能です。ユーザーはライブ画面を見ながら操作を引き取り、AIはアクセシビリティ情報を観測してsemantic referenceで操作します。

## 利用者向け

- Browser画面からpublic HTTPまたはHTTPSのページを開き、映像表示、ユーザー操作、AI操作の承認を行います。HTTP通信は暗号化されないため、機密情報にはHTTPSを使ってください。
- 承認モードは`manual`が既定です。ユーザーが選択した`auto_safe`では、ページ移動やworkerが低リスクと判定する一部操作を自動承認します。
- 送信、購入、削除などの操作や機密フィールドへの入力は自動承認されません。ユーザーはセッション中に制御を引き取れます。
- worker異常終了時はセッションがerrorになり、UIから明示的に再起動します。プロファイルは保持されますが、セッションや操作は自動復元されず、承認モードはmanualに戻ります。

## 構成

Go側の`browserSessionHub`がプロジェクトセッション、worker数、アイドル終了、制御権、承認、監査を管理します。Node.js workerはloopbackで待ち受け、ランダムBearer tokenでGoプロセスからのWebSocket接続を認証します。workerはChromium persistent contextをプロジェクト別プロファイルで起動します。

遷移先はpublic HTTPまたはHTTPSに限られ、localhostとプライベートネットワークは拒否されます。ただし、ユーザーが指定したmybox Filesページ`http://127.0.0.1:1111/projects/autohotkey/dashboard/files/macos/karabiner/README.md`は利用できます。同ページからは同一originのGET/HEAD、表示中のprojectに対する空のJSONを使ったCodex会話作成POST、選択中threadのCodex会話WebSocketだけを許可します。他のloopback宛て、書き込みHTTPメソッド、WebSocket接続は引き続き拒否されます。HTTP通信は暗号化されません。Service Workerは無効化され、ページ内WebSocketは公開WSSまたは上記のCodex会話WebSocketに限られます。public HTTP通信はworkerのloopback proxyでURLとDNS解決先を検査して転送し、指定したmybox loopback通信はcontext routeのallowlistを通過した後に直接接続します。public HTTPS通信はCONNECT proxyを通して検査済みIPへ接続します。

## 承認と監査

`auto_safe`の判定はページ内容やラベル等に基づくため、完全な安全境界ではありません。サイト側の予期しない副作用があり得るため、重要な操作は画面を確認し、必要ならユーザーが制御を引き取ってください。

監査ログは操作種別や結果などを記録しますが、URL、クエリ、フォーム値、入力値、token、password、エラー本文は記録しません。保存先やローテーション設定は[利用ガイドの内部ブラウザ設定](./user-guide.md#内部ブラウザの認証リバースプロキシ方針)を参照してください。

## 開発者向け参照

- [`../browser-worker/README.md`](../browser-worker/README.md) — workerのセットアップ、ネットワーク境界、テストfixture。
- [`../internal/entrypoint/browser.go`](../internal/entrypoint/browser.go) — Go側のセッション管理。
- [`../browser-worker/src/index.mjs`](../browser-worker/src/index.mjs) — worker側のPlaywright操作。
