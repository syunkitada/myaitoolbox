package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"
)

type serverImpl struct {
	inner *mcp.Server
}

func NewMCServer(impl *mcp.Implementation, opts *mcp.ServerOptions) domain.Server {
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
