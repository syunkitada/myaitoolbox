# `web/src/components/`

複数のページから利用する画面部品です。UIプリミティブは [`ui/README.md`](./ui/README.md)、ファイルツリーとグラフ表示はそれぞれ [`Explorer/README.md`](./Explorer/README.md)、[`GraphView/README.md`](./GraphView/README.md) に分けています。

## Index

| パス | 役割 |
| --- | --- |
| [`ui/README.md`](./ui/README.md) | Radixベースの共通UIプリミティブ。 |
| [`Explorer/README.md`](./Explorer/README.md) | ファイルツリー表示。 |
| [`GraphView/README.md`](./GraphView/README.md) | ファイルリンクグラフのSigma表示。 |
| [`AppDialogs.tsx`](./AppDialogs.tsx) | アプリ全体で使うダイアログ群。 |
| [`AgentSidebar.tsx`](./AgentSidebar.tsx) / [`AgentSidebar.test.tsx`](./AgentSidebar.test.tsx) | プロジェクト共通のAgent右サイドバー。Herdr Agentの一覧・出力・操作・停止・予約送信を提供し、モバイルではSheetとして表示する。 |
| [`CommitDiffView.tsx`](./CommitDiffView.tsx) / [`CommitDiffView.test.tsx`](./CommitDiffView.test.tsx) | コミット差分表示とそのテスト。 |
| [`ContextMenu.tsx`](./ContextMenu.tsx) | ファイル・グラフ操作のコンテキストメニュー。 |
| [`DiffView.tsx`](./DiffView.tsx) / [`DiffView.test.tsx`](./DiffView.test.tsx) | unified diff表示とそのテスト。 |
| [`FileTabs.tsx`](./FileTabs.tsx) / [`FileTabs.test.tsx`](./FileTabs.test.tsx) | 最近開いたファイルのタブと、☆ボタンから開くお気に入りモーダルおよびテスト。 |
| [`FrontmatterForm.tsx`](./FrontmatterForm.tsx) | タスクフロントマターの編集フォーム。 |
| [`GitViewer.tsx`](./GitViewer.tsx) | Git状態・差分画面を埋め込むビュー。 |
| [`Markdown.tsx`](./Markdown.tsx) / [`Markdown.test.tsx`](./Markdown.test.tsx) | サニタイズ済みMarkdown表示とテスト。 |
| [`Mermaid.tsx`](./Mermaid.tsx) | Mermaidダイアグラム描画。 |
| [`MonacoEditor.tsx`](./MonacoEditor.tsx) | Monacoベースのファイルエディタ。 |
| [`NewTaskDialog.tsx`](./NewTaskDialog.tsx) / [`NewTaskDialog.test.tsx`](./NewTaskDialog.test.tsx) | agent kindのデフォルトをcodexとし、YAMLヘッダーを除いたテンプレート本文を初期表示・編集できる、通常タスクとcron・file_created・manual task_triggerの作成ダイアログおよびテスト。 |
| [`RichMarkdown.tsx`](./RichMarkdown.tsx) / [`RichMarkdown.test.tsx`](./RichMarkdown.test.tsx) | ファイルリンク・アウトライン・コードブロック・操作可能なタスクリスト・Mermaid・Vega-Liteを表示し、表示中のファイルペインに応じて内部リンクを開き、モーダル内でText/Jira形式を切り替えられるMarkdownビューアとテスト。 |
| [`SearchBar.tsx`](./SearchBar.tsx) / [`SearchBar.test.tsx`](./SearchBar.test.tsx) | ファイル検索入力とテスト。 |
| [`Sidebar.tsx`](./Sidebar.tsx) / [`Sidebar.test.tsx`](./Sidebar.test.tsx) | ワークスペース全体のサイドバー、Agent遷移、Mybox/Herdrフォーカスの折りたたみデバッグ表示。 |
| [`SyntaxHighlighter.tsx`](./SyntaxHighlighter.tsx) / [`SyntaxHighlighter.test.tsx`](./SyntaxHighlighter.test.tsx) | Prismベースのコード表示。agent出力では相対ファイルパスをFiles画面へリンクできる。 |
| [`TaskProgress.tsx`](./TaskProgress.tsx) / [`TaskProgress.test.tsx`](./TaskProgress.test.tsx) | Markdownタスクリストの進捗表示とテスト。 |
| [`TaskAgentLaunchDialog.tsx`](./TaskAgentLaunchDialog.tsx) | FilesビューアからタスクAgentを起動する種類選択・エラー表示付きモーダル。 |
| [`VegaLite.tsx`](./VegaLite.tsx) | Vega-Lite仕様の解析、相対データURL解決、グラフ描画。 |
| [`TerminalPanel.tsx`](./TerminalPanel.tsx) | WebSocketターミナルの表示。 |
| [`TerminalTabs.tsx`](./TerminalTabs.tsx) | ターミナルセッションのタブ操作。 |
| [`badges.tsx`](./badges.tsx) | ステータス、優先度、タグなどのバッジ。 |
| [`herdr-status.tsx`](./herdr-status.tsx) | Herdrエージェント状態の表示部品。 |
| [`HerdrAgentDetail.tsx`](./HerdrAgentDetail.tsx) | Agentの端末出力（自動補正／Herdr準拠の表示切り替え）、キー送信、クイックコマンド、prompt送信、予約送信を表示する共通部品。モバイルのprompt入力は内容に応じて自動拡張し、Enter以外のキーとコマンドは一覧パレットに集約する。 |
