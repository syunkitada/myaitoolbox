// Package templates embeds the builtin templates (frontmatter YAML + Markdown body).
// プロジェクトの templates/ に同名の .md を置くと runtime で上書きされる。
package templates

import "embed"

//go:embed task knowledge
var FS embed.FS
