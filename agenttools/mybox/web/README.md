# `web/`

React + TypeScript + Viteで構成したWeb UIです。開発サーバー、Vitest、Playwrightをここから実行し、`npm run build` の成果物を `internal/webui/dist/` へコピーしてGoバイナリに同梱します。

## 使い方

```bash
npm install
npm run dev       # Vite開発サーバー
npm test          # Vitest
npm run e2e       # Playwright
npm run build     # 本番ビルド
```

ルートからは `make web-dev`、`make web-build`、`make e2e` を利用できます。

## Index

| パス | 役割 |
| --- | --- |
| [`src/README.md`](./src/README.md) | UI本体、APIクライアント、状態、グラフ、ページ。 |
| [`tests/README.md`](./tests/README.md) | Playwright E2Eテスト。 |
| [`test-results/README.md`](./test-results/README.md) | Git管理下に残っているPlaywright実行状態。編集対象外。 |
| [`components.json`](./components.json) | UIコンポーネント生成・スタイル設定。 |
| [`dumpstate.cjs`](./dumpstate.cjs) | 開発時の状態確認用スクリプト。 |
| [`e2e-debug.cjs`](./e2e-debug.cjs) | E2Eデバッグ用スクリプト。 |
| [`e2e-mouse.cjs`](./e2e-mouse.cjs) | E2Eでマウス操作を補助するスクリプト。 |
| [`e2e-sends.cjs`](./e2e-sends.cjs) | E2Eで入力送信を補助するスクリプト。 |
| [`index.html`](./index.html) | ViteのHTMLエントリポイント。 |
| [`package.json`](./package.json) | npm scriptsと依存関係の定義。 |
| [`package-lock.json`](./package-lock.json) | npm依存関係の固定解決結果。 |
| [`playwright.config.ts`](./playwright.config.ts) | Playwrightの実行設定。 |
| [`tsconfig.app.json`](./tsconfig.app.json) | アプリケーション用TypeScript設定。 |
| [`tsconfig.app.tsbuildinfo`](./tsconfig.app.tsbuildinfo) | TypeScript増分ビルドの状態。 |
| [`tsconfig.json`](./tsconfig.json) | TypeScriptプロジェクト設定。 |
| [`tsconfig.node.json`](./tsconfig.node.json) | Vite設定などNode側のTypeScript設定。 |
| [`tsconfig.node.tsbuildinfo`](./tsconfig.node.tsbuildinfo) | Node側TypeScript増分ビルドの状態。 |
| [`vite.config.ts`](./vite.config.ts) | Viteのビルド・開発サーバー設定。 |
| [`vitest.config.ts`](./vitest.config.ts) | Vitestのテスト設定。 |
