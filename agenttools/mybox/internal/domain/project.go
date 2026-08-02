package domain

type Project struct {
	Name string
	Path string
}

type Config struct {
	Projects       []Project
	DefaultProject string
}
