# Browser worker

`mybox`から起動されるPlaywright実行プロセスです。Chromiumをpersistent contextで起動し、Go側のセッション管理を介して、ページ操作・アクセシビリティ観測・JPEGフレーム配信を行います。セッション開始時とAI操作後にも現在の画面を明示送信し、mybox側は最新フレームをUIの新規接続時に再送します。

## 開発用セットアップ

```sh
npm install
npx playwright install chromium
```

`mybox`起動時は既定でこのディレクトリの`src/index.mjs`をNode.jsで起動します。別のworkerを使う場合は`MYBOX_BROWSER_WORKER`にworkerのパスを指定します。

workerはmyboxからのみ接続できる127.0.0.1のWebSocketで待ち受け、ランダムなBearer tokenで認証します。対象URLはv1ではpublic HTTPSに限定し、localhost・プライベートネットワーク・HTTPは拒否します。Service Workerは無効化し、ページ内WebSocketも同じURL検査を通します。Chromiumの外向きHTTPS接続はworker内のloopback proxyを経由し、検査済みのIPへ接続します。

myboxはプロジェクトごとにworkerを起動します。workerが異常終了すると、myboxはそのプロジェクトのセッションを`error`状態に遷移させ、保留中の操作を失敗させます。他のプロジェクトのworkerは継続します。Web UIのRestart browserから新しいworkerとセッションを明示的に開始できます。Chromiumプロファイルは保持されますが、セッションの自動復元や自動再操作は行いません。

ブラウザセッションは`manual`（既定）または`auto_safe`の承認モードを持ちます。`auto_safe`はAIのページ移動と、公開HTTPSリンク・メニュー展開などworkerが低リスクと判定した操作を自動実行します。フォーム送信、危険語を含む操作、機密フィールド入力は自動承認しません。ページ内容とラベルに基づく判定は完全な安全境界ではなく、クリック先サイトが予期しない副作用を起こす可能性があります。モード変更はmybox Web UIのユーザー操作に限られ、セッション再起動で`manual`へ戻ります。

E2Eでは `MYBOX_BROWSER_TEST_FIXTURE_URL` と `MYBOX_BROWSER_TEST_FIXTURE_FILE` を同時に設定できます。workerは完全一致するURLだけをファイル内容でinterceptし、外部ネットワークへ接続しません。この設定はテスト専用で、本番環境では設定しないでください。

## Index

| パス | 役割 |
| --- | --- |
| [`src/README.md`](./src/README.md) | worker本体の説明。 |
| [`package.json`](./package.json) | workerのnpm依存関係と起動script。 |
| [`package-lock.json`](./package-lock.json) | npm依存関係の固定解決結果。 |
| [`README.md`](./README.md) | セットアップとセキュリティ境界。 |
