package monitoring

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWrapTool(t *testing.T) {
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		return "ok", nil, nil
	}

	wrapped := wrapTool(handler)
	req := &mcp.CallToolRequest{}
	req.Params = &mcp.CallToolParamsRaw{
		Name:      "test_tool",
		Arguments: json.RawMessage(`{"param1": "value1"}`),
	}

	data, _, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != "ok" {
		t.Errorf("expected data to be ok, got %v", data)
	}
}
