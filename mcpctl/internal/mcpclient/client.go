package mcpclient

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// NewClient creates a new MCP client for the given server configuration.
func NewClient(ctx context.Context, srvConfig profile.ServerConfig) (*mcp.ClientSession, error) {
	var transport mcp.Transport
	var err error

	switch srvConfig.Transport {
	case "stdio":
		transport, err = newStdioTransport(srvConfig)
	case "streamable-http":
		transport = &mcp.StreamableClientTransport{Endpoint: srvConfig.URL}
	case "sse":
		transport = &mcp.SSEClientTransport{Endpoint: srvConfig.URL}
	default:
		return nil, fmt.Errorf("unsupported transport: %s", srvConfig.Transport)
	}

	if err != nil {
		return nil, err
	}

	impl := mcp.Implementation{
		Name:    "mcpctl",
		Version: "1.0.0",
	}

	client := mcp.NewClient(&impl, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect client: %w", err)
	}

	return session, nil
}

func newStdioTransport(srvConfig profile.ServerConfig) (mcp.Transport, error) {
	parts := strings.Fields(srvConfig.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command is empty")
	}

	cmd := parts[0]
	args := parts[1:]

	return &mcp.CommandTransport{Command: exec.Command(cmd, args...)}, nil
}
