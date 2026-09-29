# `web/src/pages/`

ルーターから表示する画面単位のコンポーネントです。ページ内部の再利用部品は [`../components/README.md`](../components/README.md) にあります。

## Index

| パス | 役割 |
| --- | --- |
| [`BrowserPage.tsx`](./BrowserPage.tsx) / [`BrowserPage.test.tsx`](./BrowserPage.test.tsx) | ドラッグで幅を調整・保存できるファイルエクスプローラー、task triggerの右クリック実行、`_tasks/<id>/task.md` のステータス変更と確認付きアーカイブ、作成・実行後の生成ファイル再読込と選択表示、ファイル名・本文検索、最大1GiBのファイルアップロード、進行中・結果ダイアログ、エディタ、ファイル名行から形式選択モーダルを開けるMarkdownプレビュー、各ペインを独立スクロールできるGit差分の左右比較表示とそのテスト。 |
| [`Dashboard.tsx`](./Dashboard.tsx) | プロジェクトダッシュボード。 |
| [`GitPage.tsx`](./GitPage.tsx) | Git状態、remote同期判定、fetch、差分、ログ、ブランチ操作。選択した変更ファイルのdiffを確認しながら編集・保存できる。 |
| [`HerdrPage.tsx`](./HerdrPage.tsx) / [`HerdrPage.test.tsx`](./HerdrPage.test.tsx) | Herdrワークスペース・エージェント画面。Herdr focusを維持したままWeb UI focusを別管理し、agentパネルの開閉状態と出力・プロンプト欄の縦サイズを保存・復元する機能、サーバー永続化された日時指定prompt送信（一覧・キャンセル）とテスト。 |
| [`KanbanBoard.tsx`](./KanbanBoard.tsx) / [`KanbanBoard.test.tsx`](./KanbanBoard.test.tsx) | タスクのGTDボードとテスト。 |
| [`KnowledgeGraphPage.tsx`](./KnowledgeGraphPage.tsx) | Markdownリンクのナレッジグラフ画面。 |
| [`ProjectsPage.tsx`](./ProjectsPage.tsx) | プロジェクト一覧・登録・削除。 |
| [`StatsPage.tsx`](./StatsPage.tsx) | ホスト統計画面。 |
