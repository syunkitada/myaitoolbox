package infrastructure

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewMCServer(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.MCP() == nil {
		t.Fatal("expected non-nil MCP server")
	}
}

func TestAddToolAndCall(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	tool := &mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		return "test_data", "test_meta", nil
	}

	s.AddTool(tool, handler)

	// Create client and connect via in-memory transport
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- s.Run(context.Background(), serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	// Call the tool
	params := &mcp.CallToolParams{
		Name:      "test_tool",
		Arguments: map[string]any{"name": "world"},
	}
	result, err := session.CallTool(context.Background(), params)
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}

	// Verify result
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if textContent.Text != `"test_data"` {
		t.Errorf("expected text to be \"test_data\", got %s", textContent.Text)
	}

	// Verify structured content
	if result.StructuredContent == nil {
		t.Fatal("expected structured content")
	}
	sc, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result.StructuredContent)
	}
	if sc["meta"] != "test_meta" {
		t.Errorf("expected meta to be test_meta, got %v", sc["meta"])
	}
	if sc["data"] != "test_data" {
		t.Errorf("expected data to be test_data, got %v", sc["data"])
	}
}

func TestAddToolError(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	tool := &mcp.Tool{
		Name:        "error_tool",
		Description: "A tool that returns error",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		return nil, nil, &testError{"test error"}
	}

	s.AddTool(tool, handler)

	// Create client and connect via in-memory transport
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- s.Run(context.Background(), serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	// Call the tool
	params := &mcp.CallToolParams{
		Name:      "error_tool",
		Arguments: map[string]any{},
	}
	result, err := session.CallTool(context.Background(), params)
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}

	// Verify error result
	if !result.IsError {
		t.Error("expected IsError to be true")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if textContent.Text != "test error" {
		t.Errorf("expected error text to be 'test error', got %s", textContent.Text)
	}

	// Structured content should be nil on error
	if result.StructuredContent != nil {
		t.Error("expected structured content to be nil on error")
	}
}

func TestMCPServer(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcpServer := s.MCP()
	if mcpServer == nil {
		t.Fatal("expected non-nil MCP server")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
