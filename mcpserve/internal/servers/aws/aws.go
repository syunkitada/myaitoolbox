package aws

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/provider"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/registry"
)

func init() {
	registry.Register(New())
}

type awsProvider struct{}

func New() provider.Provider {
	return &awsProvider{}
}

func (p *awsProvider) Name() string {
	return "aws"
}

func (p *awsProvider) Description() string {
	return "AWS integration for MCP."
}

func (p *awsProvider) NewServer() *server.MCPServer {
	s := server.NewMCPServer("aws", "0.0.1")

	s.AddTool(mcp.Tool{
		Name:        "list_instances",
		Description: "List EC2 instances",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Mock result for list_instances"), nil
	})

	return s
}
