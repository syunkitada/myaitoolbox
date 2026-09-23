package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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

	command := exec.Command(cmd, args...)
	if len(srvConfig.Env) > 0 {
		command.Env = mergeEnvironment(os.Environ(), srvConfig.Env)
	}

	return &mcp.CommandTransport{Command: command}, nil
}

func mergeEnvironment(base, overrides []string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, entry := range append(append([]string(nil), base...), overrides...) {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		if _, exists := values[parts[0]]; !exists {
			order = append(order, parts[0])
		}
		values[parts[0]] = parts[1]
	}

	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+values[key])
	}
	return result
}
