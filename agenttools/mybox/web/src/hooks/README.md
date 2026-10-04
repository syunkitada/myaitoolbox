# `web/src/hooks/`

複数の画面で共有するReact hooksを置きます。

## Index

| パス | 役割 |
| --- | --- |
| [`use-agent-sidebar.ts`](./use-agent-sidebar.ts) / [`use-agent-sidebar.test.tsx`](./use-agent-sidebar.test.tsx) | プロジェクトごとのデスクトップ / モバイル別Agentサイドバー開閉状態、選択Agent、出力表示モードのlocalStorage保存・復元。 |
| [`use-agent-favicon.ts`](./use-agent-favicon.ts) / [`use-agent-favicon.test.ts`](./use-agent-favicon.test.ts) | Herdr状態に応じたfavicon更新とテスト。 |
| [`use-escape-key.ts`](./use-escape-key.ts) / [`use-escape-key.test.tsx`](./use-escape-key.test.tsx) | Escapeキー購読とテスト。 |
| [`use-herdr.ts`](./use-herdr.ts) | Herdr概要の定期取得と、working後の未確認完了状態をlocalStorageで表示保持。 |
| [`use-mobile.ts`](./use-mobile.ts) | モバイル表示判定。 |
| [`use-resizable-height.ts`](./use-resizable-height.ts) | AgentサイドバーとFileAgentの出力・プロンプト欄の縦サイズ変更をlocalStorageへ保存・復元。 |
| [`use-resizable-width.ts`](./use-resizable-width.ts) | ドラッグ・キーボード操作に対応した幅変更とlocalStorage保存。 |
