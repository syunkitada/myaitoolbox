package markdown

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TemplateRenderer struct {
	projectRoot string
	defaultRoot string
}

func NewTemplateRenderer(projectRoot string, defaultRoot string) *TemplateRenderer {
	return &TemplateRenderer{projectRoot: projectRoot, defaultRoot: defaultRoot}
}

func (r *TemplateRenderer) RenderTask(data domain.TaskTemplateData) (string, error) {
	return r.render("task/task.md", data)
}

func (r *TemplateRenderer) RenderKnowledge(data domain.KnowledgeTemplateData) (string, error) {
	return r.render("knowledge/knowledge.md", data)
}

func (r *TemplateRenderer) render(rel string, data any) (string, error) {
	source, err := r.loadTemplate(rel)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("mybox").Funcs(template.FuncMap{
		"yamlq": yamlQuote,
	}).Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// yamlQuote returns s as a single YAML scalar, quoting it when necessary so
// the value cannot break frontmatter parsing (e.g. colons followed by spaces).
func yamlQuote(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimRight(string(b), "\n")
}

func (r *TemplateRenderer) loadTemplate(rel string) (string, error) {
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "templates", rel),
		filepath.Join(r.projectRoot, "templates", rel),
		filepath.Join(r.defaultRoot, "templates", rel),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if b, err := os.ReadFile(c); err == nil {
			return string(b), nil
		}
	}
	return builtinTemplate(rel)
}

func builtinTemplate(rel string) (string, error) {
	switch rel {
	case "task/task.md":
		return builtinTaskTemplate, nil
	case "knowledge/knowledge.md":
		return builtinKnowledgeTemplate, nil
	}
	return "", os.ErrNotExist
}

const builtinTaskTemplate = `---
title: {{.Name | yamlq}}
status: todo
priority: medium
assignee: ""
due: ""
tags: []
---

# {{.Name}}

## 目的

## やること

- [ ]
`

const builtinKnowledgeTemplate = `---
title: {{.Name | yamlq}}
aliases: []
tags: []
created: {{.Created.Format "2006-01-02T15:04:05Z07:00"}}
lastmod: {{.Created.Format "2006-01-02T15:04:05Z07:00"}}
---

# {{.Name}}
`
