package domain

type TaskTemplateData struct {
	Name string
}

type TemplateRenderer interface {
	RenderTask(data TaskTemplateData) (string, error)
}
