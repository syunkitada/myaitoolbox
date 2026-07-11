package domain

import (
	"fmt"
)

var (
	providers = make(map[string]Provider)
)

// Register registers a new MCP provider to the registry.
func Register(p Provider) {
	if _, exists := providers[p.Name()]; exists {
		panic(fmt.Sprintf("provider %q already registered", p.Name()))
	}
	providers[p.Name()] = p
}

// Get returns the provider by name.
func Get(name string) (Provider, bool) {
	p, exists := providers[name]
	return p, exists
}

// List returns a list of all registered providers.
func List() []Provider {
	var list []Provider
	for _, p := range providers {
		list = append(list, p)
	}
	return list
}
