package provider

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

type serverImpl struct {
	inner *mcp.Server
}

func NewMCServer(impl *mcp.Implementation, opts *mcp.ServerOptions) Server {
	return &serverImpl{inner: mcp.NewServer(impl, opts)}
}

func (s *serverImpl) AddTool(tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error)) {
	s.inner.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, meta, err := handler(ctx, req)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}
		text := ""
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			text = string(b)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: map[string]interface{}{
				"meta": meta,
				"data": data,
			},
		}, nil
	})
}

func (s *serverImpl) Run(ctx context.Context, transport mcp.Transport) error {
	return s.inner.Run(ctx, transport)
}

func (s *serverImpl) MCP() *mcp.Server {
	return s.inner
}
