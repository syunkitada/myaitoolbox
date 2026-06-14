package mcpclient

import (
	"context"
	"fmt"
	"strings"
	"os"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// NewClient creates a new MCP client for the given server configuration.
func NewClient(ctx context.Context, srvConfig profile.ServerConfig) (*client.Client, error) {
	var mcpClient *client.Client
	var err error

	switch srvConfig.Transport {
	case "stdio":
		mcpClient, err = newStdioClient(ctx, srvConfig)
	case "streamable-http":
		mcpClient, err = newStreamableHTTPClient(ctx, srvConfig)
	case "sse":
		mcpClient, err = newSSEClient(ctx, srvConfig)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", srvConfig.Transport)
	}

	if err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "mcpctl",
		Version: "1.0.0",
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	return mcpClient, nil
}

func newStdioClient(ctx context.Context, srvConfig profile.ServerConfig) (*client.Client, error) {
	parts := strings.Fields(srvConfig.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command is empty")
	}

	cmd := parts[0]
	args := parts[1:]

	mcpClient, err := client.NewStdioMCPClient(cmd, os.Environ(), args...)
	if err != nil {
		return nil, err
	}
	
	// Start the client.
	// We need an init request to establish the protocol session.
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start stdio client: %w", err)
	}

	return mcpClient, nil
}

func newStreamableHTTPClient(ctx context.Context, srvConfig profile.ServerConfig) (*client.Client, error) {
	mcpClient, err := client.NewStreamableHttpClient(srvConfig.URL)
	if err != nil {
		return nil, err
	}
	
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start streamable-http client: %w", err)
	}

	return mcpClient, nil
}

func newSSEClient(ctx context.Context, srvConfig profile.ServerConfig) (*client.Client, error) {
	mcpClient, err := client.NewSSEMCPClient(srvConfig.URL)
	if err != nil {
		return nil, err
	}
	
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start sse client: %w", err)
	}

	return mcpClient, nil
}
