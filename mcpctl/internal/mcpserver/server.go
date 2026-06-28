package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/runtime"
)

func NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "mcpctl",
		Version: "1.0.0",
	}, nil)

	// Tool: list
	s.AddTool(&mcp.Tool{
		Name:        "list",
		Description: "List available tools across configured MCP servers",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile": map[string]interface{}{
					"type":        "string",
					"description": "Optional profile name to use for listing. If not provided, default profile is used.",
				},
			},
		},
	}, listHandler)

	// Tool: search
	s.AddTool(&mcp.Tool{
		Name:        "search",
		Description: "Search for tools by keyword in name or description",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile": map[string]interface{}{
					"type":        "string",
					"description": "Optional profile name to use",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Keyword to search for",
				},
			},
			"required": []string{"query"},
		},
	}, searchHandler)

	// Tool: info
	s.AddTool(&mcp.Tool{
		Name:        "info",
		Description: "Get detailed information about a specific tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile": map[string]interface{}{
					"type":        "string",
					"description": "Optional profile name to use",
				},
				"tool": map[string]interface{}{
					"type":        "string",
					"description": "Tool name in format <server>/<tool>",
				},
			},
			"required": []string{"tool"},
		},
	}, infoHandler)

	// Tool: call
	s.AddTool(&mcp.Tool{
		Name:        "call",
		Description: "Execute a specific tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"profile": map[string]interface{}{
					"type":        "string",
					"description": "Optional profile name to use",
				},
				"tool": map[string]interface{}{
					"type":        "string",
					"description": "Tool name in format <server>/<tool>",
				},
				"params": map[string]interface{}{
					"type":        "object",
					"description": "Parameters to pass to the tool. Must be a JSON object with string keys.",
				},
			},
			"required": []string{"tool"},
		},
	}, callHandler)

	return s
}

func parseArgs(request *mcp.CallToolRequest) (map[string]interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, err
	}
	return args, nil
}

func toolResultError(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func toolResultText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func listHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := parseArgs(request)
	profName, _ := args["profile"].(string)

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := discovery.ListTools(ctx, p, "")
	if err != nil && len(entries) == 0 {
		return toolResultError(fmt.Sprintf("Failed to list tools: %v", err)), nil
	}

	var out string
	for _, entry := range entries {
		out += fmt.Sprintf("%s/%s\n", entry.ServerName, entry.Tool.Name)
	}
	if out == "" {
		out = "No tools found."
	}
	return toolResultText(out), nil
}

func searchHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := parseArgs(request)
	profName, _ := args["profile"].(string)
	query, _ := args["query"].(string)

	if query == "" {
		return toolResultError("Query is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := discovery.SearchTools(ctx, p, query)
	if err != nil && len(entries) == 0 {
		return toolResultError(fmt.Sprintf("Failed to search tools: %v", err)), nil
	}

	var out string
	for _, entry := range entries {
		out += fmt.Sprintf("%s/%s\n", entry.ServerName, entry.Tool.Name)
	}
	if out == "" {
		out = "No matching tools found."
	}
	return toolResultText(out), nil
}

func infoHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := parseArgs(request)
	profName, _ := args["profile"].(string)
	toolPath, _ := args["tool"].(string)

	if toolPath == "" {
		return toolResultError("Tool name is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := discovery.ParseToolName(toolPath)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	entry, err := discovery.GetToolInfo(ctx, p, serverName, toolName)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	out := fmt.Sprintf("Name:\n  %s/%s\n\nDescription:\n  %s\n", entry.ServerName, entry.Tool.Name, entry.Tool.Description)
	return toolResultText(out), nil
}

func callHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := parseArgs(request)
	profName, _ := args["profile"].(string)
	toolPath, _ := args["tool"].(string)
	params := make(map[string]interface{})
	if prm, ok := args["params"].(map[string]interface{}); ok {
		params = prm
	}

	if toolPath == "" {
		return toolResultError("Tool name is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := discovery.ParseToolName(toolPath)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	return runtime.CallTool(ctx, p, serverName, toolName, params)
}
