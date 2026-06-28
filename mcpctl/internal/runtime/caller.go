package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpclient "github.com/syunkitada/myaitoolbox/mcpctl/internal/mcpclient"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// CallTool executes a tool on a specific server with the given arguments.
func CallTool(ctx context.Context, prof *profile.Profile, serverName, toolName string, params map[string]interface{}) (*mcp.CallToolResult, error) {
	srvConfig, ok := prof.Servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found in profile %s", serverName, prof.Name)
	}

	session, err := mcpclient.NewClient(ctx, srvConfig)
	if err != nil {
		return nil, fmt.Errorf("server %s: failed to connect: %w", serverName, err)
	}
	defer session.Close()

	// Marshal params to json.RawMessage for Arguments
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
