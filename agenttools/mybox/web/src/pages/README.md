# `web/src/pages/`

ルーターから表示する画面単位のコンポーネントです。ページ内部の再利用部品は [`../components/README.md`](../components/README.md) にあります。

## Index

| パス | 役割 |
| --- | --- |
| [`BrowserPage.tsx`](./BrowserPage.tsx) / [`BrowserPage.test.tsx`](./BrowserPage.test.tsx) | ドラッグで幅を調整・保存できるファイルエクスプローラー、最近開いたファイルタブとお気に入りモーダル、ヘッダーからのファイル / ディレクトリ作成、メイン選択を切り替えても維持され同じファイルも表示できる参照ペイン付き2ペインビュー、task triggerの右クリック実行、`_tasks/<id>/task.md` のステータス変更と確認付きアーカイブ、作成・実行後の生成ファイル再読込と選択表示、ファイル名・本文検索、最大1GiBのファイルアップロード、進行中・結果ダイアログ、エディタ、Markdownの形式選択コピーと非Markdownテキストの生コピーに対応したファイルプレビュー、各ペインを独立スクロールできるGit差分の左右比較表示とそのテスト。 |
| [`Dashboard.tsx`](./Dashboard.tsx) | プロジェクトダッシュボード。 |
| [`GitPage.tsx`](./GitPage.tsx) / [`GitPage.test.tsx`](./GitPage.test.tsx) | Git状態、remote同期判定、fetch、差分、ログ、ブランチ操作。選択した変更ファイルのdiffを確認しながら編集・保存でき、コミットメッセージ入力中の `Ctrl+Enter` 確定とそのテストを含む。 |
| [`HerdrPage.tsx`](./HerdrPage.tsx) / [`HerdrPage.test.tsx`](./HerdrPage.test.tsx) | HerdrワークスペースのTabs/Panes操作画面。Agent一覧・操作はプロジェクト共通の右サイドバーへ移設し、Herdrのタブ・ペイン操作とテストを担当する。 |
| [`KanbanBoard.tsx`](./KanbanBoard.tsx) / [`KanbanBoard.test.tsx`](./KanbanBoard.test.tsx) | タスクのGTDボードとテスト。 |
| [`KnowledgeGraphPage.tsx`](./KnowledgeGraphPage.tsx) | Markdownリンクのナレッジグラフ画面。 |
| [`ProjectsPage.tsx`](./ProjectsPage.tsx) | プロジェクト一覧・登録・削除。 |
| [`StatsPage.tsx`](./StatsPage.tsx) | ホスト統計画面。 |
