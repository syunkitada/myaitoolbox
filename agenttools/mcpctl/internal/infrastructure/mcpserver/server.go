package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

type Server struct {
	mcp       *mcp.Server
	discovery domain.ToolDiscovery
	executor  domain.ToolExecutor
	resolver  domain.ProfileResolver
}

func NewServer(discovery domain.ToolDiscovery, executor domain.ToolExecutor, resolver domain.ProfileResolver) *Server {
	s := &Server{
		discovery: discovery,
		executor:  executor,
		resolver:  resolver,
	}

	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    "mcpctl",
		Version: domain.Version,
	}, nil)

	s.registerTools()
	return s
}

func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.mcp.Run(ctx, transport)
}

func (s *Server) registerTools() {
	s.mcp.AddTool(&mcp.Tool{
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
	}, s.listHandler)

	s.mcp.AddTool(&mcp.Tool{
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
	}, s.searchHandler)

	s.mcp.AddTool(&mcp.Tool{
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
	}, s.infoHandler)

	s.mcp.AddTool(&mcp.Tool{
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
	}, s.callHandler)
}

func (s *Server) parseArgs(request *mcp.CallToolRequest) (map[string]interface{}, error) {
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

func (s *Server) listHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := s.parseArgs(request)
	profName, _ := args["profile"].(string)

	p, err := s.resolver.Resolve("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := s.discovery.ListTools(ctx, p, "")
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

func (s *Server) searchHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := s.parseArgs(request)
	profName, _ := args["profile"].(string)
	query, _ := args["query"].(string)

	if query == "" {
		return toolResultError("Query is required"), nil
	}

	p, err := s.resolver.Resolve("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	entries, err := s.discovery.SearchTools(ctx, p, query)
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

func (s *Server) infoHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := s.parseArgs(request)
	profName, _ := args["profile"].(string)
	toolPath, _ := args["tool"].(string)

	if toolPath == "" {
		return toolResultError("Tool name is required"), nil
	}

	p, err := s.resolver.Resolve("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := domain.ParseToolName(toolPath)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	entry, err := s.discovery.GetToolInfo(ctx, p, serverName, toolName)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	out := fmt.Sprintf("Name:\n  %s/%s\n\nDescription:\n  %s\n", entry.ServerName, entry.Tool.Name, entry.Tool.Description)

	if schema, ok := entry.Tool.InputSchema.(map[string]interface{}); ok {
		if schemaType, _ := schema["type"].(string); schemaType == "object" {
			props, _ := schema["properties"].(map[string]interface{})
			requiredRaw, _ := schema["required"].([]interface{})
			requiredSet := make(map[string]bool)
			for _, r := range requiredRaw {
				if s, ok := r.(string); ok {
					requiredSet[s] = true
				}
			}

			if len(props) > 0 {
				out += "\nParameters:\n"
				for paramName, paramSchemaRaw := range props {
					paramSchema, ok := paramSchemaRaw.(map[string]interface{})
					req := ""
					if requiredSet[paramName] {
						req = " (required)"
					}
					if ok {
						typ, _ := paramSchema["type"].(string)
						if typ == "" {
							typ = "any"
						}
						if typ == "array" {
							if items, ok := paramSchema["items"].(map[string]interface{}); ok {
								if itemType, ok := items["type"].(string); ok {
									typ = "array[" + itemType + "]"
								}
							}
						}
						out += fmt.Sprintf("  %s: %s%s\n", paramName, typ, req)
					} else {
						out += fmt.Sprintf("  %s%s\n", paramName, req)
					}
				}
			}
		}
	}

	return toolResultText(out), nil
}

func (s *Server) callHandler(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := s.parseArgs(request)
	profName, _ := args["profile"].(string)
	toolPath, _ := args["tool"].(string)
	params := make(map[string]interface{})
	if prm, ok := args["params"].(map[string]interface{}); ok {
		params = prm
	}

	if toolPath == "" {
		return toolResultError("Tool name is required"), nil
	}

	p, err := s.resolver.Resolve("", profName)
	if err != nil {
		return toolResultError(fmt.Sprintf("Failed to resolve profile: %v", err)), nil
	}

	serverName, toolName, err := domain.ParseToolName(toolPath)
	if err != nil {
		return toolResultError(err.Error()), nil
	}

	return s.executor.CallTool(ctx, p, serverName, toolName, params)
}
