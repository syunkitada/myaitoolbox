package runtime

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpclient "github.com/syunkitada/myaitoolbox/mcpctl/internal/mcpclient"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// CallTool executes a tool on a specific server with the given arguments.
func CallTool(ctx context.Context, prof *profile.Profile, serverName, toolName string, params map[string]interface{}) (*mcp.CallToolResult, error) {
	srvConfig, ok := prof.Servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found in profile %s", serverName, prof.Name)
	}

	client, err := mcpclient.NewClient(ctx, srvConfig)
	if err != nil {
		return nil, fmt.Errorf("server %s: failed to connect: %w", serverName, err)
	}
	defer client.Close()

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = params

	res, err := client.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("server %s: failed to call tool %s: %w", serverName, toolName, err)
	}

	return res, nil
}
