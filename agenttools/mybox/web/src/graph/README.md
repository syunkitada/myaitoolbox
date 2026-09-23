# `web/src/graph/`

プロジェクト内のMarkdownファイルとリンクを、UIで扱うノード・エッジ・ディレクトリフレームへ変換するロジックです。

## Index

| パス | 役割 |
| --- | --- |
| [`links.ts`](./links.ts) | Markdownリンクを解析し、グラフ用リンクを作成。 |
| [`nodeId.ts`](./nodeId.ts) | ファイル、ディレクトリ、外部リンクのノードID変換。 |
| [`projection.ts`](./projection.ts) / [`projection.test.ts`](./projection.test.ts) | グラフの表示対象投影とテスト。 |
| [`reconcile.ts`](./reconcile.ts) / [`reconcile.test.ts`](./reconcile.test.ts) | Graphologyグラフの差分反映と表示属性、テスト。 |
| [`tree.ts`](./tree.ts) / [`tree.test.ts`](./tree.test.ts) | ファイルツリー構築と表示範囲計算、テスト。 |
| [`types.ts`](./types.ts) | グラフ・ツリーの型定義。 |

