package jira

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

type jiraProvider struct{}

func New() provider.Provider {
	return &jiraProvider{}
}

func (p *jiraProvider) Name() string {
	return "jira"
}

func (p *jiraProvider) Description() string {
	return "Jira integration for MCP."
}

func (p *jiraProvider) NewServer() *server.MCPServer {
	s := server.NewMCPServer("jira", "0.0.1")

	s.AddTool(mcp.Tool{
		Name:        "get_issue",
		Description: "Get Jira issue details",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"issue_key": map[string]interface{}{
					"type": "string",
				},
			},
			Required: []string{"issue_key"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Mock result for get_issue"), nil
	})

	return s
}
