package entrypoint

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/infrastructure"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring"
)

func NewRegistryWithProviders() *Registry {
	registry := NewRegistry()
	registry.Register(monitoring.New())
	return registry
}

func Run(registry *Registry, serverName, transport, host, port string) error {
	p, exists := registry.Get(serverName)
	if !exists {
		return fmt.Errorf("server %q not found", serverName)
	}

	server := infrastructure.NewMCServer(&mcp.Implementation{Name: serverName, Version: "0.0.1"}, nil)
	p.RegisterTools(server)

	slog.Info("Starting server", "server", serverName, "transport", transport)

	if transport == "http" {
		addr := fmt.Sprintf("%s:%s", host, port)
		handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return server.MCP()
		}, nil)
		slog.Info("MCP HTTP server listening", "addr", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			slog.Error("HTTP server error", "error", err)
			return err
		}
	} else {
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			slog.Error("Server error", "error", err)
			return err
		}
	}
	return nil
}
