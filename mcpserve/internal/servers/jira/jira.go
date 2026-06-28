package jira

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func (p *jiraProvider) NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "jira", Version: "0.0.1"}, nil)

	s.AddTool(&mcp.Tool{
		Name:        "get_issue",
		Description: "Get Jira issue details",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issue_key": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"issue_key"},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Mock result for get_issue"},
			},
		}, nil
	})

	return s
}
