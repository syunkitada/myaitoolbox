package domain

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Provider defines the interface for an MCP Server implementation.
type Provider interface {
	Name() string
	Description() string
	NewServer() Server
}

// Server defines an MCP server with standardized response formatting.
type Server interface {
	// AddTool adds a tool. The handler returns (data, meta, err) which is
	// automatically formatted into {meta, data} StructuredContent.
	AddTool(tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error))
	// Run starts the server with the given transport.
	Run(ctx context.Context, transport mcp.Transport) error
	// MCP returns the underlying *mcp.Server for use with NewSSEHandler.
	MCP() *mcp.Server
}
