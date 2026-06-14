package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/runtime"
)

func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"mcpctl",
		"1.0.0",
	)

	// Tool: list
	listTool := mcp.NewTool("list",
		mcp.WithDescription("List available tools across configured MCP servers"),
		mcp.WithString("profile", mcp.Description("Optional profile name to use for listing. If not provided, default profile is used.")),
	)
	s.AddTool(listTool, listHandler)

	// Tool: search
	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Search for tools by keyword in name or description"),
		mcp.WithString("profile", mcp.Description("Optional profile name to use")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keyword to search for")),
	)
	s.AddTool(searchTool, searchHandler)

	// Tool: info
	infoTool := mcp.NewTool("info",
		mcp.WithDescription("Get detailed information about a specific tool"),
		mcp.WithString("profile", mcp.Description("Optional profile name to use")),
		mcp.WithString("tool", mcp.Required(), mcp.Description("Tool name in format <server>/<tool>")),
	)
	s.AddTool(infoTool, infoHandler)

	// Tool: call
	callTool := mcp.NewTool("call",
		mcp.WithDescription("Execute a specific tool"),
		mcp.WithString("profile", mcp.Description("Optional profile name to use")),
		mcp.WithString("tool", mcp.Required(), mcp.Description("Tool name in format <server>/<tool>")),
		mcp.WithObject("params", mcp.Description("Parameters to pass to the tool. Must be a JSON object with string keys.")),
	)
	s.AddTool(callTool, callHandler)

	return s
}

func listHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profName := ""
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if p, ok := args["profile"].(string); ok {
			profName = p
		}
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := discovery.ListTools(ctx, p, "")
	if err != nil && len(entries) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list tools: %v", err)), nil
	}

	var out string
	for _, entry := range entries {
		out += fmt.Sprintf("%s/%s\n", entry.ServerName, entry.Tool.Name)
	}

	if out == "" {
		out = "No tools found."
	}
	return mcp.NewToolResultText(out), nil
}

func searchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profName := ""
	var query string
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if p, ok := args["profile"].(string); ok {
			profName = p
		}
		if q, ok := args["query"].(string); ok {
			query = q
		}
	}
	if query == "" {
		return mcp.NewToolResultError("Query is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := discovery.SearchTools(ctx, p, query)
	if err != nil && len(entries) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search tools: %v", err)), nil
	}

	var out string
	for _, entry := range entries {
		out += fmt.Sprintf("%s/%s\n", entry.ServerName, entry.Tool.Name)
	}
	
	if out == "" {
		out = "No matching tools found."
	}
	return mcp.NewToolResultText(out), nil
}

func infoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profName := ""
	var toolPath string
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if p, ok := args["profile"].(string); ok {
			profName = p
		}
		if t, ok := args["tool"].(string); ok {
			toolPath = t
		}
	}
	if toolPath == "" {
		return mcp.NewToolResultError("Tool name is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := discovery.ParseToolName(toolPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	entry, err := discovery.GetToolInfo(ctx, p, serverName, toolName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	out := fmt.Sprintf("Name:\n  %s/%s\n\nDescription:\n  %s\n", entry.ServerName, entry.Tool.Name, entry.Tool.Description)
	return mcp.NewToolResultText(out), nil
}

func callHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profName := ""
	var toolPath string
	params := make(map[string]interface{})
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if p, ok := args["profile"].(string); ok {
			profName = p
		}
		if t, ok := args["tool"].(string); ok {
			toolPath = t
		}
		if prm, ok := args["params"].(map[string]interface{}); ok {
			params = prm
		}
	}

	if toolPath == "" {
		return mcp.NewToolResultError("Tool name is required"), nil
	}

	p, err := profile.ResolveProfile("", profName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := discovery.ParseToolName(toolPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return runtime.CallTool(ctx, p, serverName, toolName, params)
}
