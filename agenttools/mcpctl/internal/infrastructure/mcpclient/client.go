package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func NewClient(ctx context.Context, srvConfig domain.ServerConfig) (*mcp.ClientSession, error) {
	slog.Debug("creating MCP client", "transport", srvConfig.Transport)

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
		Version: domain.Version,
	}

	client := mcp.NewClient(&impl, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect client: %w", err)
	}

	return session, nil
}

func newStdioTransport(srvConfig domain.ServerConfig) (mcp.Transport, error) {
	if srvConfig.Command == "" {
		return nil, fmt.Errorf("command is empty")
	}

	var cmd string
	var args []string

	if len(srvConfig.Args) > 0 {
		cmd = srvConfig.Command
		args = srvConfig.Args
	} else {
		parts := strings.Fields(srvConfig.Command)
		if len(parts) == 0 {
			return nil, fmt.Errorf("command is empty")
		}
		cmd = parts[0]
		args = parts[1:]
	}

	return &mcp.CommandTransport{Command: exec.Command(cmd, args...)}, nil
}
