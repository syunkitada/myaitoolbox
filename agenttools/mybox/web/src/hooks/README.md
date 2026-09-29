# `web/src/hooks/`

複数の画面で共有するReact hooksを置きます。

## Index

| パス | 役割 |
| --- | --- |
| [`use-agent-favicon.ts`](./use-agent-favicon.ts) / [`use-agent-favicon.test.ts`](./use-agent-favicon.test.ts) | Herdr状態に応じたfavicon更新とテスト。 |
| [`use-escape-key.ts`](./use-escape-key.ts) / [`use-escape-key.test.tsx`](./use-escape-key.test.tsx) | Escapeキー購読とテスト。 |
| [`use-herdr.ts`](./use-herdr.ts) | Herdr概要の定期取得と、mybox未確認の完了状態の表示保持。 |
| [`use-mobile.ts`](./use-mobile.ts) | モバイル表示判定。 |
| [`use-resizable-width.ts`](./use-resizable-width.ts) | ドラッグ・キーボード操作に対応した幅変更とlocalStorage保存。 |
