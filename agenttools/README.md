# `agenttools/`

AIエージェントやMCPを利用するためのツール群をまとめたディレクトリです。

## Index

| パス | 役割 |
| --- | --- |
| [`mcpctl/`](./mcpctl/) | MCPツールの検索・情報表示・実行を行うCLI。 |
| [`mcpserve/`](./mcpserve/) | 複数のMCPサーバ実装を束ねて起動するGoランタイム。 |
| [`mybox/`](./mybox/) | ローカルプロジェクトのタスク・ファイル・Git・AIエージェントなどを管理するワークスペースツール。 |
| [`mygit/`](./mygit/) | Git repositoryをmanifestとlockfileで管理する仕様・ツール。詳細は `mygit/docs/README.md` を参照。 |
| [`myntfy/`](./myntfy/) | 保存済みtopicでntfy通知を送受信するCLI。 |
