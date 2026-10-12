package infrastructure

import (
	"context"
	"math"
	"testing"
	"time"

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
		return "test_data", map[string]interface{}{
			"request_id":  "req-1",
			"data_status": "live",
			"warnings":    nil,
		}, nil
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

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].OutputSchema == nil {
		t.Fatal("expected the common output schema to be advertised")
	}
	outputSchema, ok := listed.Tools[0].OutputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("expected output schema object, got %T", listed.Tools[0].OutputSchema)
	}
	properties, ok := outputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected output schema properties, got %T", outputSchema["properties"])
	}
	for _, name := range []string{"data", "meta", "error"} {
		if _, ok := properties[name]; !ok {
			t.Errorf("expected output schema property %q", name)
		}
	}

	// Call the tool
	params := &mcp.CallToolParams{
		Name:      "test_tool",
		Arguments: map[string]any{"name": "world"},
	}
	result, err := session.CallTool(context.Background(), params)
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}

	// StructuredContent is the canonical machine-readable result. Successful
	// calls do not duplicate it in unstructured content.
	if len(result.Content) != 0 {
		text, _ := result.Content[0].(*mcp.TextContent)
		t.Fatalf("expected no content in successful result, got %d items: %#v", len(result.Content), text)
	}

	// Verify structured content
	if result.StructuredContent == nil {
		t.Fatal("expected structured content")
	}
	sc, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result.StructuredContent)
	}
	meta, ok := sc["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta object, got %T", sc["meta"])
	}
	if meta["request_id"] != "req-1" {
		t.Errorf("expected request_id to be req-1, got %v", meta["request_id"])
	}
	if meta["schema_version"] != "1" {
		t.Errorf("expected schema_version to be 1, got %v", meta["schema_version"])
	}
	if meta["tool"] != "test_tool" {
		t.Errorf("expected tool to be test_tool, got %v", meta["tool"])
	}
	if meta["data_status"] != "live" {
		t.Errorf("expected data_status to be preserved as live, got %v", meta["data_status"])
	}
	if meta["execution_status"] != "success" {
		t.Errorf("expected execution_status to be success, got %v", meta["execution_status"])
	}
	if _, ok := meta["requested_at"].(string); !ok {
		t.Errorf("expected requested_at string, got %T", meta["requested_at"])
	}
	if _, ok := meta["warnings"].([]interface{}); !ok {
		t.Errorf("expected warnings array, got %T", meta["warnings"])
	}
	if sc["data"] != "test_data" {
		t.Errorf("expected data to be test_data, got %v", sc["data"])
	}
	if sc["error"] != nil {
		t.Errorf("expected error to be null, got %v", sc["error"])
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

	// Errors use the same envelope so callers can handle success and failure
	// without parsing two different response shapes.
	if result.StructuredContent == nil {
		t.Fatal("expected structured content on error")
	}
	sc, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result.StructuredContent)
	}
	if sc["data"] != nil {
		t.Errorf("expected data to be null, got %v", sc["data"])
	}
	if sc["meta"].(map[string]interface{})["data_status"] != "error" {
		t.Errorf("expected error data_status, got %v", sc["meta"])
	}
	if sc["meta"].(map[string]interface{})["execution_status"] != "error" {
		t.Errorf("expected error execution_status, got %v", sc["meta"])
	}
	errorObject, ok := sc["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got %T", sc["error"])
	}
	if errorObject["code"] != "TOOL_ERROR" {
		t.Errorf("expected TOOL_ERROR, got %v", errorObject["code"])
	}
	if errorObject["message"] != "test error" {
		t.Errorf("expected test error message, got %v", errorObject["message"])
	}
	if errorObject["retryable"] != false {
		t.Errorf("expected retryable=false, got %v", errorObject["retryable"])
	}
}

func TestAddToolUnencodableResult(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	s.AddTool(&mcp.Tool{
		Name:        "nan_tool",
		Description: "A tool that returns a non-JSON number",
		InputSchema: map[string]interface{}{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
		return math.NaN(), nil, nil
	})

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

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "nan_tool", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for an unencodable result")
	}
	if result.StructuredContent == nil {
		t.Fatal("expected structured content for an unencodable result")
	}
	sc, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result.StructuredContent)
	}
	errorObject, ok := sc["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got %T", sc["error"])
	}
	if errorObject["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR, got %v", errorObject["code"])
	}
}

func TestMCPServer(t *testing.T) {
	s := NewMCServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcpServer := s.MCP()
	if mcpServer == nil {
		t.Fatal("expected non-nil MCP server")
	}
}

func TestDescribeToolError(t *testing.T) {
	code, retryable := describeToolError(&describedTestError{})
	if code != "UPSTREAM_UNAVAILABLE" {
		t.Errorf("expected UPSTREAM_UNAVAILABLE, got %s", code)
	}
	if !retryable {
		t.Error("expected retryable=true")
	}
}

func TestBuildResponseMetaUsesRequestTime(t *testing.T) {
	requestedAt := time.Date(2026, time.October, 12, 9, 0, 0, 123456789, time.UTC)
	meta, err := buildResponseMeta("test_tool", nil, "success", requestedAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta["requested_at"] != requestedAt.Format(time.RFC3339Nano) {
		t.Errorf("expected requested_at %q, got %v", requestedAt.Format(time.RFC3339Nano), meta["requested_at"])
	}
	if meta["data_status"] != "unknown" {
		t.Errorf("expected default data_status unknown, got %v", meta["data_status"])
	}
	if meta["execution_status"] != "success" {
		t.Errorf("expected execution_status success, got %v", meta["execution_status"])
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

type describedTestError struct{}

func (e *describedTestError) Error() string {
	return "upstream unavailable"
}

func (e *describedTestError) Code() string {
	return "UPSTREAM_UNAVAILABLE"
}

func (e *describedTestError) Retryable() bool {
	return true
}
