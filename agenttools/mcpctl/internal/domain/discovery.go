package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "1.0.0"

type ToolEntry struct {
	ServerName string
	Tool       *mcp.Tool
}

type ToolDiscovery interface {
	ListTools(ctx context.Context, prof *Profile, serverFilter string) ([]ToolEntry, error)
	GetToolInfo(ctx context.Context, prof *Profile, serverName, toolName string) (*ToolEntry, error)
	SearchTools(ctx context.Context, prof *Profile, query string) ([]ToolEntry, error)
}

type ToolExecutor interface {
	CallTool(ctx context.Context, prof *Profile, serverName, toolName string, params map[string]interface{}) (*mcp.CallToolResult, error)
}

type ProfileResolver interface {
	Resolve(flagProfile, mcpProfile string) (*Profile, error)
	LoadConfig() (*Config, error)
	SaveConfig(cfg *Config) error
	ListProfiles() ([]string, error)
}

func ParseToolName(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tool name format, expected <server>/<tool>")
	}
	return parts[0], parts[1], nil
}
