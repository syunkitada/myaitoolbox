# `web/src/state/`

ブラウザ内で保持するFiles・Graphの表示状態と、画面をまたいで共有する実行状態を管理します。

## Index

| パス | 役割 |
| --- | --- |
| [`fileExecution.tsx`](./fileExecution.tsx) / [`fileExecution.test.tsx`](./fileExecution.test.tsx) | サーバー管理の実行可能ファイルジョブを再接続し、実行状態、出力、停止・再表示・Dismiss、直近20件の完了済み履歴を管理するReact contextとテスト。 |
| [`graphViewState.ts`](./graphViewState.ts) / [`graphViewState.test.ts`](./graphViewState.test.ts) | Graphの表示モード、Filesと共有するフォルダ展開、カメラ位置、ノード配置のlocalStorage永続化とテスト。 |
