# mcpctl AIエージェントガイドライン

このドキュメントは、AIエージェント向けに `mcpctl` を効率的に使用するための手順と例を提供します。

## ツール検索

ツール名または説明からツールを検索します。

例:
```bash
mcpctl search issue
```

## ツール情報確認

実行前にツール情報とスキーマパラメータを確認します。

例:
```bash
mcpctl info github/create_issue
```

## ツール実行

`info` コマンドでパラメータを確認した後にツールを実行します。

例:
```bash
mcpctl call github/create_issue \
  --title "Bug"
```

JSON文字列で引数を渡すこともできます:
```bash
mcpctl call github/create_issue \
  --params '{"title":"Bug"}'
```

## Profile管理

プロファイル一覧:
```bash
mcpctl profiles
```

現在のプロファイル:
```bash
mcpctl profiles current
```

プロファイル切替:
```bash
mcpctl profiles use prod
```

## ルール

- **ツール名が不明な場合は必ず検索**すること。
- **実行前に必ずツール情報を確認**すること。
- **パラメータ名を推測しない**こと。
- **再試行はツール情報を再確認した後のみ**行うこと。
- **`search` → `info` → `call` のワークフローを優先**すること。
- **指示がない限りデフォルトプロファイルを使用**すること。
- **機能追加・変更時には**、対応するREADME.md、docs/* 内のファイルを参照し、必要に応じて更新すること。
