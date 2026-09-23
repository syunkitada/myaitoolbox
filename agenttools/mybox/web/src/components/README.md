# `web/src/components/`

複数のページから利用する画面部品です。UIプリミティブは [`ui/README.md`](./ui/README.md)、ファイルツリーとグラフ表示はそれぞれ [`Explorer/README.md`](./Explorer/README.md)、[`GraphView/README.md`](./GraphView/README.md) に分けています。

## Index

| パス | 役割 |
| --- | --- |
| [`ui/README.md`](./ui/README.md) | Radixベースの共通UIプリミティブ。 |
| [`Explorer/README.md`](./Explorer/README.md) | ファイルツリー表示。 |
| [`GraphView/README.md`](./GraphView/README.md) | ファイルリンクグラフのSigma表示。 |
| [`AppDialogs.tsx`](./AppDialogs.tsx) | アプリ全体で使うダイアログ群。 |
| [`CommitDiffView.tsx`](./CommitDiffView.tsx) / [`CommitDiffView.test.tsx`](./CommitDiffView.test.tsx) | コミット差分表示とそのテスト。 |
| [`ContextMenu.tsx`](./ContextMenu.tsx) | ファイル・グラフ操作のコンテキストメニュー。 |
| [`DiffView.tsx`](./DiffView.tsx) / [`DiffView.test.tsx`](./DiffView.test.tsx) | unified diff表示とそのテスト。 |
| [`FileAgentWidget.tsx`](./FileAgentWidget.tsx) / [`FileAgentWidget.test.tsx`](./FileAgentWidget.test.tsx) | ファイルからHerdrエージェントを起動する部品とテスト。 |
| [`FileTabs.tsx`](./FileTabs.tsx) / [`FileTabs.test.tsx`](./FileTabs.test.tsx) | 開いているファイルのタブとテスト。 |
| [`FrontmatterForm.tsx`](./FrontmatterForm.tsx) | タスクフロントマターの編集フォーム。 |
| [`GitViewer.tsx`](./GitViewer.tsx) | Git状態・差分画面を埋め込むビュー。 |
| [`Markdown.tsx`](./Markdown.tsx) / [`Markdown.test.tsx`](./Markdown.test.tsx) | サニタイズ済みMarkdown表示とテスト。 |
| [`Mermaid.tsx`](./Mermaid.tsx) | Mermaidダイアグラム描画。 |
| [`MonacoEditor.tsx`](./MonacoEditor.tsx) | Monacoベースのファイルエディタ。 |
| [`NewTaskDialog.tsx`](./NewTaskDialog.tsx) | タスク作成ダイアログ。 |
| [`RichMarkdown.tsx`](./RichMarkdown.tsx) / [`RichMarkdown.test.tsx`](./RichMarkdown.test.tsx) | ファイルリンク・アウトライン・コードブロック・操作可能なタスクリストを表示し、モーダル内でText/Jira形式を切り替えられるMarkdownビューアとテスト。 |
| [`SearchBar.tsx`](./SearchBar.tsx) / [`SearchBar.test.tsx`](./SearchBar.test.tsx) | ファイル検索入力とテスト。 |
| [`Sidebar.tsx`](./Sidebar.tsx) | ワークスペース全体のサイドバー。 |
| [`SyntaxHighlighter.tsx`](./SyntaxHighlighter.tsx) / [`SyntaxHighlighter.test.tsx`](./SyntaxHighlighter.test.tsx) | Prismベースのコード表示とテスト。 |
| [`TerminalPanel.tsx`](./TerminalPanel.tsx) | WebSocketターミナルの表示。 |
| [`TerminalTabs.tsx`](./TerminalTabs.tsx) | ターミナルセッションのタブ操作。 |
| [`badges.tsx`](./badges.tsx) | ステータス、優先度、タグなどのバッジ。 |
| [`herdr-status.tsx`](./herdr-status.tsx) | Herdrエージェント状態の表示部品。 |
