# `web/src/components/GraphView/`

Markdownリンクから作ったグラフのデータをFilesのフォルダ展開状態から組み立て、Sigmaで描画します。カメラ位置の保存やノード選択、コンテキストメニューも担当します。

## Index

| パス | 役割 |
| --- | --- |
| [`GraphWorkspace.tsx`](./GraphWorkspace.tsx) / [`GraphWorkspace.test.tsx`](./GraphWorkspace.test.tsx) | Filesの共有展開状態に基づくグラフ構築、Markdown取得、操作連携とテスト。 |
| [`GraphView.tsx`](./GraphView.tsx) | Sigmaによるグラフの描画、操作、選択状態の連携。 |

