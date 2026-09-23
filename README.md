# myaitoolbox

AIエージェント向けのツール、Go製のMCP実装、個人ワークスペース管理ツールをまとめたリポジトリです。

## Index

| パス | 役割 |
| --- | --- |
| [`AGENTS.md`](./AGENTS.md) | リポジトリ全体のREADME整備と作業に関するガイドライン。 |
| [`agenttools/`](./agenttools/) | `mcpctl`、`mcpserve`、`mybox`などのエージェント関連ツール。 |
| [`docs/`](./docs/) | Goプロジェクトで共有する設計・構成・技術スタックの参考資料。 |
| [`inbox/`](./inbox/) | 作業中の検証環境、提案、メモ。READMEは必須ではない領域。 |
| [`scripts/`](./scripts/) | myboxやTailscaleの起動・再インストール用スクリプト。 |
| [`.gitignore`](./.gitignore) | リポジトリ共通の除外設定。 |
| [`README.md`](./README.md) | リポジトリ全体の入口。本文書。 |

## mybox

[`agenttools/mybox/`](./agenttools/mybox/) は、Markdownを唯一のデータソースとする個人ワークスペースツールです。プロジェクト管理、ファイル管理、タスク管理、AIエージェント連携、ターミナル、Web UIを提供します。

プライベートネットワーク内での利用を想定しており、認証機能はありません。スマートフォンなどから接続する場合は、TailscaleなどのVPN越しに利用してください。

myboxの更新と再インストールは次のスクリプトで行えます。

```bash
./scripts/reinstall-mybox.sh
```

起動時は、用途に応じたポート番号のスクリプトをtmux内で実行します。herdr内では起動しないでください。

```bash
./scripts/start-or-restart-mybox1111.sh
```

## 依存する実行環境

- `tmux`: インフラアプリケーションの実行基盤。
- `herdr`: AIエージェントの実行基盤。myboxのエージェント連携で利用。
- AIエージェント: `opencode`、`codex`、`antigravity`など。
