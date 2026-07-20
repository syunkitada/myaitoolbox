package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

type ToolExecutor struct{}

func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{}
}

func (e *ToolExecutor) CallTool(ctx context.Context, prof *domain.Profile, serverName, toolName string, params map[string]interface{}) (*mcp.CallToolResult, error) {
	slog.Debug("calling tool", "server", serverName, "tool", toolName, "profile", prof.Name)

	srvConfig, ok := prof.Servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found in profile %s", serverName, prof.Name)
	}

	session, err := NewClient(ctx, srvConfig)
	if err != nil {
		return nil, fmt.Errorf("server %s: failed to connect: %w", serverName, err)
	}
	defer session.Close()

	argsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: json.RawMessage(argsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("server %s: failed to call tool %s: %w", serverName, toolName, err)
	}

	return res, nil
}
