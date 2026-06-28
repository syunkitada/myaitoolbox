package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func (p *githubProvider) NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "github", Version: "0.0.1"}, nil)

	// Mock tools
	s.AddTool(&mcp.Tool{
		Name:        "search_repositories",
		Description: "Search GitHub repositories",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
			},
			"required": []string{"query"},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Mock result for search_repositories"},
			},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "create_issue",
		Description: "Create a new issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"title"},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Mock result for create_issue"},
			},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "get_pull_request",
		Description: "Get pull request details",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pr_number": map[string]interface{}{
					"type": "number",
				},
			},
			"required": []string{"pr_number"},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Mock result for get_pull_request"},
			},
		}, nil
	})

	return s
}
