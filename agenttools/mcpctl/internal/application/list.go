package application

import (
	"context"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func ListTools(ctx context.Context, discovery domain.ToolDiscovery, prof *domain.Profile, serverFilter string) ([]domain.ToolEntry, error) {
	return discovery.ListTools(ctx, prof, serverFilter)
}
