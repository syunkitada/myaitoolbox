package application

import (
	"context"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

func SearchTools(ctx context.Context, discovery domain.ToolDiscovery, prof *domain.Profile, query string) ([]domain.ToolEntry, error) {
	return discovery.SearchTools(ctx, prof, query)
}
