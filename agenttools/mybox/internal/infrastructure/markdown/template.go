package markdown

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

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
	tmpl, err := template.New("mybox").Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
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
id: {{.ID}}
title: {{.Name}}
status: todo
priority: medium
assignee: ""
due: ""
tags: []
project: {{.Project}}
created: {{.Created.Format "2006-01-02T15:04:05Z07:00"}}
---

# {{.Name}}

## 目的

## やること

- [ ]
`

const builtinKnowledgeTemplate = `---
title: {{.Name}}
aliases: []
tags: []
created: {{.Created.Format "2006-01-02T15:04:05Z07:00"}}
lastmod: {{.Created.Format "2006-01-02T15:04:05Z07:00"}}
---

# {{.Name}}
`
