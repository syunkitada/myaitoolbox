package infrastructure

import (
	"testing"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Description() string {
	return "mock provider"
}

func (m *mockProvider) NewServer() domain.Server {
	return nil
}

func TestRegisterAndGet(t *testing.T) {
	// Clear the registry for test isolation
	providers = make(map[string]domain.Provider)

	p := &mockProvider{name: "test-provider"}
	Register(p)

	got, exists := Get("test-provider")
	if !exists {
		t.Fatal("expected provider to exist")
	}
	if got.Name() != "test-provider" {
		t.Errorf("expected name test-provider, got %s", got.Name())
	}
}

func TestRegisterDuplicate(t *testing.T) {
	// Clear the registry for test isolation
	providers = make(map[string]domain.Provider)

	p1 := &mockProvider{name: "duplicate"}
	Register(p1)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate registration")
		}
	}()

	p2 := &mockProvider{name: "duplicate"}
	Register(p2)
}

func TestGetNonexistent(t *testing.T) {
	// Clear the registry for test isolation
	providers = make(map[string]domain.Provider)

	_, exists := Get("nonexistent")
	if exists {
		t.Error("expected provider to not exist")
	}
}

func TestList(t *testing.T) {
	// Clear the registry for test isolation
	providers = make(map[string]domain.Provider)

	p1 := &mockProvider{name: "provider1"}
	p2 := &mockProvider{name: "provider2"}
	Register(p1)
	Register(p2)

	list := List()
	if len(list) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(list))
	}

	names := make(map[string]bool)
	for _, p := range list {
		names[p.Name()] = true
	}
	if !names["provider1"] || !names["provider2"] {
		t.Errorf("expected both providers in list, got %v", names)
	}
}

func TestListEmpty(t *testing.T) {
	// Clear the registry for test isolation
	providers = make(map[string]domain.Provider)

	list := List()
	if len(list) != 0 {
		t.Errorf("expected 0 providers, got %d", len(list))
	}
}
