# `skills/myherdr/`

Codex に myherdr 固有の Herdr session、SSH 許可 host、ブラウザー引き継ぎの運用を指示します。Herdr pane の一般的な操作手順は重複せず、Herdr が提供する skill に任せます。

## 配置

`SKILL.md` と `references/` を一緒に Codex の skills ディレクトリへ配置します。例:

```bash
mkdir -p "$CODEX_HOME/skills"
ln -s /path/to/myaitoolbox/agenttools/myherdr/skills/myherdr "$CODEX_HOME/skills/myherdr"
```

## Index

| パス | 役割 |
| --- | --- |
| [`README.md`](./README.md) | skill の目的と配置方法。 |
| [`SKILL.md`](./SKILL.md) | Codex の myherdr 利用手順。 |
| [`references/`](./references/) | SSH policy とユーザー引き継ぎの詳細。詳細は [`references/README.md`](./references/README.md) を参照。 |
