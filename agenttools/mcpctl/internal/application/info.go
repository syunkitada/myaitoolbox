package application

import (
	"context"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func GetToolInfo(ctx context.Context, discovery domain.ToolDiscovery, prof *domain.Profile, serverName, toolName string) (*domain.ToolEntry, error) {
	return discovery.GetToolInfo(ctx, prof, serverName, toolName)
}
