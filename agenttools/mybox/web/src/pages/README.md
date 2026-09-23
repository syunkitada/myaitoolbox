# `web/src/pages/`

ルーターから表示する画面単位のコンポーネントです。ページ内部の再利用部品は [`../components/README.md`](../components/README.md) にあります。

## Index

| パス | 役割 |
| --- | --- |
| [`BrowserPage.tsx`](./BrowserPage.tsx) | ファイルエクスプローラー、エディタ、ファイル名行から形式選択モーダルを開けるMarkdownプレビュー。 |
| [`Dashboard.tsx`](./Dashboard.tsx) | プロジェクトダッシュボード。 |
| [`GitPage.tsx`](./GitPage.tsx) | Git状態、remote同期判定、fetch、差分、ログ、ブランチ操作。選択した変更ファイルのdiffを確認しながら編集・保存できる。 |
| [`HerdrPage.tsx`](./HerdrPage.tsx) / [`HerdrPage.test.tsx`](./HerdrPage.test.tsx) | Herdrワークスペース・エージェント画面とテスト。 |
| [`KanbanBoard.tsx`](./KanbanBoard.tsx) / [`KanbanBoard.test.tsx`](./KanbanBoard.test.tsx) | タスクのGTDボードとテスト。 |
| [`KnowledgeGraphPage.tsx`](./KnowledgeGraphPage.tsx) | Markdownリンクのナレッジグラフ画面。 |
| [`ProjectsPage.tsx`](./ProjectsPage.tsx) | プロジェクト一覧・登録・削除。 |
| [`StatsPage.tsx`](./StatsPage.tsx) | ホスト統計画面。 |
