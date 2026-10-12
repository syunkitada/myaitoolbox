package infrastructure

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mymcpserve/internal/domain"
)

type serverImpl struct {
	inner *mcp.Server
}

func NewMCServer(impl *mcp.Implementation, opts *mcp.ServerOptions) domain.Server {
	return &serverImpl{inner: mcp.NewServer(impl, opts)}
}

func (s *serverImpl) AddTool(tool *mcp.Tool, handler func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error)) {
	tool.OutputSchema = responseSchema(tool.OutputSchema)
	s.inner.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		requestedAt := time.Now().UTC()
		data, meta, err := handler(ctx, req)
		if err != nil {
			code, retryable := describeToolError(err)
			return &mcp.CallToolResult{
				IsError:           true,
				Content:           []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				StructuredContent: newErrorEnvelope(tool.Name, meta, code, err.Error(), retryable, requestedAt),
			}, nil
		}
		structured := newSuccessResult(tool.Name, data, meta, requestedAt)
		if structured["error"] != nil {
			return &mcp.CallToolResult{
				IsError:           true,
				Content:           []mcp.Content{&mcp.TextContent{Text: "failed to encode tool result"}},
				StructuredContent: structured,
			}, nil
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{},
			StructuredContent: structured,
		}, nil
	})
}

func (s *serverImpl) Run(ctx context.Context, transport mcp.Transport) error {
	return s.inner.Run(ctx, transport)
}

func (s *serverImpl) MCP() *mcp.Server {
	return s.inner
}
