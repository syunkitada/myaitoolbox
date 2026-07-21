package application

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func WrapTool(handler func(context.Context, *mcp.CallToolRequest) (data, meta interface{}, err error)) func(context.Context, *mcp.CallToolRequest) (data, meta interface{}, err error) {
	return func(ctx context.Context, request *mcp.CallToolRequest) (data, meta interface{}, err error) {
		var toolName string
		var args any
		if request != nil && request.Params != nil {
			toolName = request.Params.Name
			if len(request.Params.Arguments) > 0 {
				_ = json.Unmarshal(request.Params.Arguments, &args)
			}
		}

		slog.Info("MCP tool called",
			slog.String("tool", toolName),
			slog.Any("parameters", args),
		)

		data, meta, err = handler(ctx, request)
		if err != nil {
			slog.Error("MCP tool error",
				slog.String("tool", toolName),
				slog.Any("error", err),
			)
		} else {
			slog.Info("MCP tool execution completed",
				slog.String("tool", toolName),
			)
		}
		return data, meta, err
	}
}
