# `web/src/utils/`

ページやコンポーネントから利用する、ドメイン寄りの補助ロジックを置きます。可能な範囲でUIレンダリングから分離してテストします。

## Index

| パス | 役割 |
| --- | --- |
| [`clipboard.ts`](./clipboard.ts) | クリップボードへのコピー。 |
| [`graphCollision.ts`](./graphCollision.ts) / [`graphCollision.test.ts`](./graphCollision.test.ts) | グラフノードの衝突計算とテスト。 |
| [`herdr-agent-commands.ts`](./herdr-agent-commands.ts) | Herdrエージェント向けコマンド生成。 |
| [`herdr-file-agent.ts`](./herdr-file-agent.ts) / [`herdr-file-agent.test.ts`](./herdr-file-agent.test.ts) | ファイルからエージェントを起動する引数・表示名処理とテスト。 |
| [`herdr-layout.ts`](./herdr-layout.ts) / [`herdr-layout.test.ts`](./herdr-layout.test.ts) | Herdrペイン分割・リサイズレイアウトとテスト。 |
| [`herdr-status.ts`](./herdr-status.ts) / [`herdr-status.test.ts`](./herdr-status.test.ts) | Herdrで確認したworking状態と、mybox未確認の完了状態をlocalStorageへ保持する状態遷移とテスト。 |
| [`agent-output-display.ts`](./agent-output-display.ts) / [`agent-output-display.test.ts`](./agent-output-display.test.ts) | Agent出力の表示モード判定と自動補正（改行・空行・行末空白の正規化）。 |
| [`agent-sidebar-status.ts`](./agent-sidebar-status.ts) / [`agent-sidebar-status.test.ts`](./agent-sidebar-status.test.ts) | プロジェクト別Agentの抽出と、ヘッダー表示用ステータスの集約。 |
| [`line-diff.ts`](./line-diff.ts) / [`line-diff.test.ts`](./line-diff.test.ts) | Monacoの行差分計算。大規模入力では計算量を制限する。 |
| [`markdown-completions.ts`](./markdown-completions.ts) / [`markdown-completions.test.ts`](./markdown-completions.test.ts) | Markdown入力の遅延補完候補とモデル単位の状態管理・テスト。 |
| [`markdown-link-completions.ts`](./markdown-link-completions.ts) / [`markdown-link-completions.test.ts`](./markdown-link-completions.test.ts) | 相対Markdownリンク補完とテスト。 |
| [`markdown.ts`](./markdown.ts) / [`markdown.test.ts`](./markdown.test.ts) | Markdown変換・リンク処理・Jira wiki markupへのコピー変換とテスト。 |
| [`prism-langs.ts`](./prism-langs.ts) | Prism対応言語の登録。 |
| [`routes.ts`](./routes.ts) | base pathとプロジェクトURLの構築。 |
| [`scheduled-prompts.ts`](./scheduled-prompts.ts) | サーバーAPIの予約プロンプトを画面表示用へ変換し、旧ブラウザ保存データを扱う補助。 |
