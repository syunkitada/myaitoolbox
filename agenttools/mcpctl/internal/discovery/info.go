package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// GetToolInfo retrieves a specific tool.
func GetToolInfo(ctx context.Context, prof *profile.Profile, serverName, toolName string) (*ToolEntry, error) {
	entries, err := ListTools(ctx, prof, serverName)
	if err != nil && len(entries) == 0 {
		return nil, err
	}

	for _, entry := range entries {
		if entry.ServerName == serverName && entry.Tool.Name == toolName {
			return &entry, nil
		}
	}

	// Maybe the user passed the tool without knowing the server name, or just to try to match something.
	// But according to spec, info gets `<server>/<tool>`.
	return nil, fmt.Errorf("tool %s/%s not found", serverName, toolName)
}

// ParseToolName splits <server>/<tool> into serverName and toolName.
func ParseToolName(name string) (string, string, error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tool name format, expected <server>/<tool>")
	}
	return parts[0], parts[1], nil
}
