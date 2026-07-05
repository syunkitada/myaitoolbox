package monitoring

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMonitoringServerTools(t *testing.T) {
	provider := New()
	if provider.Name() != "monitoring" {
		t.Errorf("expected provider name to be monitoring, got %s", provider.Name())
	}

	server := provider.NewServer()
	if server == nil {
		t.Fatal("expected NewServer to not be nil")
	}

	// Verify wrapTool works as expected and logs without panic
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	}

	wrapped := wrapTool(handler)
	req := &mcp.CallToolRequest{}
	req.Params = &mcp.CallToolParamsRaw{
		Name:      "test_tool",
		Arguments: json.RawMessage(`{"param1": "value1"}`),
	}

	res, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatal("expected non-nil response content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if tc.Text != "ok" {
		t.Errorf("expected response text to be ok, got %s", tc.Text)
	}
}
