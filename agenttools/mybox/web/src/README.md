# `web/src/`

Web UIのアプリケーションコードです。ページと再利用コンポーネント、APIクライアント、状態管理、Markdown・グラフ関連のロジックを分けています。

## Index

| パス | 役割 |
| --- | --- |
| [`api/README.md`](./api/README.md) | REST APIの型付きクライアント。 |
| [`components/README.md`](./components/README.md) | ページから利用するUIコンポーネント。 |
| [`graph/README.md`](./graph/README.md) | ファイルリンクグラフのモデルと投影処理。 |
| [`hooks/README.md`](./hooks/README.md) | React hooksと非同期UI状態。 |
| [`lib/README.md`](./lib/README.md) | UI共通の小さなライブラリとナビゲーションイベント。 |
| [`pages/README.md`](./pages/README.md) | 画面単位のコンポーネント。 |
| [`state/README.md`](./state/README.md) | エクスプローラーとグラフの永続状態。 |
| [`test/README.md`](./test/README.md) | テスト環境の初期化。 |
| [`utils/README.md`](./utils/README.md) | Markdown、Herdr、レイアウト、パスなどの純粋な補助処理。 |
| [`App.tsx`](./App.tsx) | ルーティングとアプリケーション全体のレイアウト。 |
| [`globals.css`](./globals.css) | Tailwind/CSS変数を含むグローバルスタイル。 |
| [`main.tsx`](./main.tsx) | ReactルートとBrowser Routerの起動。 |
| [`monaco-setup.ts`](./monaco-setup.ts) | Monaco Editorの言語・ワーカー設定。 |
| [`vite-env.d.ts`](./vite-env.d.ts) | Viteの型定義参照。 |

