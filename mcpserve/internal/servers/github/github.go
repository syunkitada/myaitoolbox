package github

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

type githubProvider struct{}

func New() provider.Provider {
	return &githubProvider{}
}

func (p *githubProvider) Name() string {
	return "github"
}

func (p *githubProvider) Description() string {
	return "GitHub integration for MCP."
}

func (p *githubProvider) NewServer() *server.MCPServer {
	s := server.NewMCPServer("github", "0.0.1")

	// Mock tools
	s.AddTool(mcp.Tool{
		Name:        "search_repositories",
		Description: "Search GitHub repositories",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Mock result for search_repositories"), nil
	})

	s.AddTool(mcp.Tool{
		Name:        "create_issue",
		Description: "Create a new issue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"title": map[string]interface{}{
					"type": "string",
				},
			},
			Required: []string{"title"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Mock result for create_issue"), nil
	})

	s.AddTool(mcp.Tool{
		Name:        "get_pull_request",
		Description: "Get pull request details",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pr_number": map[string]interface{}{
					"type": "number",
				},
			},
			Required: []string{"pr_number"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Mock result for get_pull_request"), nil
	})

	return s
}
