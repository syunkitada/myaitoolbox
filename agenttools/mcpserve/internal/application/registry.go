package application

import (
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/infrastructure"
)

// List returns a list of all registered providers.
func List() []domain.Provider {
	return infrastructure.List()
}

// Get returns the provider by name.
func Get(name string) (domain.Provider, bool) {
	return infrastructure.Get(name)
}
