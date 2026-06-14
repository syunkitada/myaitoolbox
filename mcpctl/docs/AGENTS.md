# mcpctl AI Agent Guidelines

This document provides instructions and examples for AI agents on how to use `mcpctl` efficiently.

## ツール検索 (Search for a tool)

Search for a tool by name or description.

Example:
```bash
mcpctl search issue
```

## ツール情報確認 (Inspect tool info)

Inspect tool information and schema parameters before execution.

Example:
```bash
mcpctl info github/create_issue
```

## ツール実行 (Execute a tool)

Execute the tool after confirming its parameters using the `info` command.

Example:
```bash
mcpctl call github/create_issue \
  --title "Bug"
```

You can also pass arguments via a JSON string:
```bash
mcpctl call github/create_issue \
  --params '{"title":"Bug"}'
```

## Profile 管理 (Profile management)

List profiles:
```bash
mcpctl profiles
```

Show current profile:
```bash
mcpctl profiles current
```

Switch profile:
```bash
mcpctl profiles use prod
```

## Rules

- **Always search** when tool names are unknown.
- **Always inspect** tool information before execution.
- **Never guess** parameter names.
- **Retry only after** reviewing tool information again.
- **Prefer** `search` → `info` → `call` workflow.
- **Use the default profile** unless instructed otherwise.
