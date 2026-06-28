package aws

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func (p *awsProvider) NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "aws", Version: "0.0.1"}, nil)

	s.AddTool(&mcp.Tool{
		Name:        "list_instances",
		Description: "List EC2 instances",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Mock result for list_instances",
				},
			},
		}, nil
	})

	return s
}
