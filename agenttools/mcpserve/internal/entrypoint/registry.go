package entrypoint

import (
	"fmt"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
)

type Registry struct {
	providers map[string]domain.Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]domain.Provider)}
}

func (r *Registry) Register(p domain.Provider) {
	if _, exists := r.providers[p.Name()]; exists {
		panic(fmt.Sprintf("provider %q already registered", p.Name()))
	}
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (domain.Provider, bool) {
	p, exists := r.providers[name]
	return p, exists
}

func (r *Registry) List() []domain.Provider {
	var list []domain.Provider
	for _, p := range r.providers {
		list = append(list, p)
	}
	return list
}

func (r *Registry) ListNames() []string {
	var names []string
	for _, p := range r.providers {
		names = append(names, p.Name())
	}
	return names
}
