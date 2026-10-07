# `web/src/state/`

ブラウザ内で保持するUI状態と、画面をまたいで共有する実行状態を管理します。

## Index

| パス | 役割 |
| --- | --- |
| [`explorerState.tsx`](./explorerState.tsx) | エクスプローラーの展開、選択、表示範囲などのReact context。 |
| [`fileExecution.tsx`](./fileExecution.tsx) / [`fileExecution.test.tsx`](./fileExecution.test.tsx) | サーバー管理の実行可能ファイルジョブを再接続し、実行状態、出力、停止・再表示・Dismiss、直近20件の完了済み履歴を管理するReact contextとテスト。 |
| [`graphViewState.ts`](./graphViewState.ts) | グラフのカメラ位置とノードレイアウトのlocalStorage永続化。 |
