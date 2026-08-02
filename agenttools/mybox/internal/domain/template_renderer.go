package domain

import "time"

type TaskTemplateData struct {
	ID      string
	Name    string
	Project string
	Created time.Time
}

type KnowledgeTemplateData struct {
	Path    string
	Name    string
	Created time.Time
}

type TemplateRenderer interface {
	RenderTask(data TaskTemplateData) (string, error)
	RenderKnowledge(data KnowledgeTemplateData) (string, error)
}
