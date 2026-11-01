# `web/src/components/`

複数のページから利用する画面部品です。UIプリミティブは [`ui/README.md`](./ui/README.md)、グラフ表示は [`GraphView/README.md`](./GraphView/README.md) に分けています。

## Index

| パス | 役割 |
| --- | --- |
| [`ui/README.md`](./ui/README.md) | Radixベースの共通UIプリミティブ。 |
| [`GraphView/README.md`](./GraphView/README.md) | ファイルリンクグラフのSigma表示。 |
| [`AppDialogs.tsx`](./AppDialogs.tsx) | アプリ全体で使うダイアログ群。 |
| [`AgentSidebar.tsx`](./AgentSidebar.tsx) / [`AgentSidebar.test.tsx`](./AgentSidebar.test.tsx) | プロジェクト共通の右サイドバー。Herdr Agentの一覧・出力・操作・停止・予約送信と、Codexモードへの切り替えを提供する。スクリプト実行状態も表示し、モバイルではSheetとして表示する。開閉ボタンにはHerdrとCodexの代表状態を示すステータスアイコンを表示する。 |
| [`CodexChat.tsx`](./CodexChat.tsx) / [`CodexChat.test.tsx`](./CodexChat.test.tsx) | Codex CLI / IDEの会話履歴と状態表示、新規会話、名前変更、Markdown応答、行番号ジャンプ付きプロジェクト内ファイルリンク、実行詳細、差分、Steer、停止、承認と質問回答、Plan/Defaultモード切替、Goal管理、`/compact`を含むコマンドパレットを表示する。入力欄の直上にモードセレクタとプロジェクトごとに保存する最近使ったクイックコマンドを表示する。Files画面のレビューコメントを選択中会話の入力下書きへ追記する。メッセージ送信済みの会話はURLとプロジェクトごとのAgentサイドバー状態に保持し、リロード後やプロジェクト切替後のAgentパネル再表示時に復元する。未送信の空会話は接続中だけ保持し、会話一覧には表示しない。初回送信後に選択状態を保存する。会話表示中は左側に同じプロジェクトのセッションをステータスアイコンだけで並べ、タイトルと状態はツールチップで示す。選択・更新できる。セッション一覧の代表状態を開閉ボタンと折りたたみサイドバーに表示する。入力フォームのコマンドパレット・停止・送信ボタンは右側に縦に並べる。スマートフォンでは会話本文の横はみ出しを抑え、入力欄のフォーカス時に自動拡大されないようにする。 |
| [`AgentLaunchDialog.tsx`](./AgentLaunchDialog.tsx) | Agent kindとHerdr Tabを指定して一般Agentを起動するモーダル。Tabは既存候補を表示しつつ自由入力できる。 |
| [`CommitDiffView.tsx`](./CommitDiffView.tsx) / [`CommitDiffView.test.tsx`](./CommitDiffView.test.tsx) | コミット差分表示とそのテスト。 |
| [`ContextMenu.tsx`](./ContextMenu.tsx) | ファイル・グラフ操作のコンテキストメニュー。 |
| [`DiffView.tsx`](./DiffView.tsx) / [`DiffView.test.tsx`](./DiffView.test.tsx) | unified diff表示とそのテスト。 |
| [`FileExecutionModal.tsx`](./FileExecutionModal.tsx) / [`FileExecutionModal.test.tsx`](./FileExecutionModal.test.tsx) | スクリプト実行中の出力、終了結果、停止、Dismissを表示するモーダルとテスト。 |
| [`FileReviewComments.tsx`](./FileReviewComments.tsx) / [`FileReviewComments.test.tsx`](./FileReviewComments.test.tsx) | localStorageに保存するファイル・行範囲コメント、返信・解決、HerdrまたはCodexの選択中下書きへの追記。選択範囲の操作は本文付近または狭い画面の下端に表示し、ファイルコメント操作と未解決件数はスクロール追従ヘッダーに表示する。 |
| [`FileTabs.tsx`](./FileTabs.tsx) / [`FileTabs.test.tsx`](./FileTabs.test.tsx) | 最近開いたファイルのタブと、プロジェクト付きお気に入りの追加・削除および横断遷移モーダルとテスト。 |
| [`FrontmatterForm.tsx`](./FrontmatterForm.tsx) / [`FrontmatterForm.test.tsx`](./FrontmatterForm.test.tsx) | タスクと通常のMarkdownのフロントマター編集フォームとテスト。 |
| [`GitViewer.tsx`](./GitViewer.tsx) | Git状態・差分画面を埋め込むビュー。 |
| [`Markdown.tsx`](./Markdown.tsx) / [`Markdown.test.tsx`](./Markdown.test.tsx) | サニタイズ済みMarkdown表示と、呼び出し元で指定できるリンクURL変換。 |
| [`Mermaid.tsx`](./Mermaid.tsx) | Mermaidダイアグラム描画。 |
| [`MonacoEditor.tsx`](./MonacoEditor.tsx) | Monacoベースのファイルエディタ。 |
| [`NewTaskDialog.tsx`](./NewTaskDialog.tsx) / [`NewTaskDialog.test.tsx`](./NewTaskDialog.test.tsx) | agent kindのデフォルトをcodexとし、YAMLヘッダーを除いたテンプレート本文を初期表示・編集できる、通常タスクとcron・file_created・manual task_triggerの作成ダイアログおよびテスト。 |
| [`RichMarkdown.tsx`](./RichMarkdown.tsx) / [`RichMarkdown.test.tsx`](./RichMarkdown.test.tsx) | ファイルリンク・アウトライン・コードブロック・操作可能なタスクリスト・Mermaid・Vega-Liteを表示し、表示中のファイルペインに応じて内部リンクを開き、モーダル内でText/Jira形式を切り替えられるMarkdownビューアとテスト。 |
| [`SearchBar.tsx`](./SearchBar.tsx) / [`SearchBar.test.tsx`](./SearchBar.test.tsx) | ファイル検索入力とテスト。 |
| [`Sidebar.tsx`](./Sidebar.tsx) / [`Sidebar.test.tsx`](./Sidebar.test.tsx) | ワークスペース全体のサイドバー。プロジェクトごとのgit・Herdr状態と稼働中・未確認Codex件数を表示し、Agents欄にはHerdr Agentと全プロジェクトの稼働中・未確認Codexセッションを並べる。終了セッションはステータスアイコンまたは確認ボタンで確認するまでブラウザーに保存し、タイトルをクリックして会話を開いても残す。折りたたみ時は状態ドットを表示する。Agent遷移、Mybox/Herdrフォーカスの折りたたみデバッグ表示も扱う。 |
| [`SyntaxHighlighter.tsx`](./SyntaxHighlighter.tsx) / [`SyntaxHighlighter.test.tsx`](./SyntaxHighlighter.test.tsx) | Prismベースのコード表示。ファイル本文ではレビュー用の元行番号を保持し、agent出力では相対ファイルパスをFiles画面へリンクできる。 |
| [`TaskProgress.tsx`](./TaskProgress.tsx) / [`TaskProgress.test.tsx`](./TaskProgress.test.tsx) | Markdownタスクリストの進捗表示とテスト。 |
| [`TaskAgentLaunchDialog.tsx`](./TaskAgentLaunchDialog.tsx) | FilesビューアからタスクAgentを起動する種類選択・エラー表示付きモーダル。 |
| [`VegaLite.tsx`](./VegaLite.tsx) | Vega-Lite仕様の解析、相対データURL解決、グラフ描画。 |
| [`TerminalPanel.tsx`](./TerminalPanel.tsx) / [`TerminalPanel.test.tsx`](./TerminalPanel.test.tsx) | WebSocketターミナルの表示、プロジェクト内で再利用するタブ表示番号と復元処理のテスト。 |
| [`TerminalTabs.tsx`](./TerminalTabs.tsx) / [`TerminalTabs.test.tsx`](./TerminalTabs.test.tsx) | ターミナルセッションのタブ操作、表示復帰時のサイズ調整、ドラッグによる並び替えとテスト。 |
| [`badges.tsx`](./badges.tsx) | ステータス、優先度、タグなどのバッジ。 |
| [`herdr-status.tsx`](./herdr-status.tsx) | Herdrエージェント状態の表示部品。 |
| [`HerdrAgentDetail.tsx`](./HerdrAgentDetail.tsx) | Agentの端末出力（直近200行から表示し、必要に応じて最大5,000行まで追加読込、自動補正／Herdr準拠の表示切り替え）、キー送信、クイックコマンド、prompt送信、予約送信を表示する共通部品。入力途中Promptは親からcontrolled stateとして受け取り、モバイルのprompt入力は内容に応じて自動拡張し、Enter以外のキーとコマンドは一覧パレットに集約する。 |
