package provider

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Provider defines the interface for an MCP Server implementation.
type Provider interface {
	Name() string
	Description() string
	NewServer() *mcp.Server
}
