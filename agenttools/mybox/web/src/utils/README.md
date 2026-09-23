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
| [`markdown-completions.ts`](./markdown-completions.ts) | Markdown入力の補完候補。 |
| [`markdown-link-completions.ts`](./markdown-link-completions.ts) / [`markdown-link-completions.test.ts`](./markdown-link-completions.test.ts) | 相対Markdownリンク補完とテスト。 |
| [`markdown.ts`](./markdown.ts) / [`markdown.test.ts`](./markdown.test.ts) | Markdown変換・リンク処理・Jira wiki markupへのコピー変換とテスト。 |
| [`prism-langs.ts`](./prism-langs.ts) | Prism対応言語の登録。 |
| [`routes.ts`](./routes.ts) | base pathとプロジェクトURLの構築。 |
