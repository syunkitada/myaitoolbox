package provider

import "github.com/mark3labs/mcp-go/server"

// Provider defines the interface for an MCP Server implementation.
type Provider interface {
	Name() string
	Description() string
	NewServer() *server.MCPServer
}
