package monitoring

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/infrastructure"
)

func TestProviderName(t *testing.T) {
	p := New()
	if p.Name() != "monitoring" {
		t.Errorf("expected name monitoring, got %s", p.Name())
	}
}

func TestRegisterTools(t *testing.T) {
	p := New()
	server := infrastructure.NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	p.RegisterTools(server)
	if server.MCP() == nil {
		t.Fatal("expected non-nil MCP server after RegisterTools")
	}
}

func TestProviderInterface(t *testing.T) {
	var _ domain.Provider = New()
}
