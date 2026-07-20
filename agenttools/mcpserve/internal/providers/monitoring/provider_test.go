package monitoring

import (
	"testing"
)

func TestProviderName(t *testing.T) {
	p := New()
	if p.Name() != "monitoring" {
		t.Errorf("expected name monitoring, got %s", p.Name())
	}
}

func TestNewServer(t *testing.T) {
	p := New()
	s := p.NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
