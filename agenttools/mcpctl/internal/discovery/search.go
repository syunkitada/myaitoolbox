package discovery

import (
	"context"
	"strings"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// SearchTools searches for tools whose name or description matches the query.
func SearchTools(ctx context.Context, prof *profile.Profile, query string) ([]ToolEntry, error) {
	entries, err := ListTools(ctx, prof, "")
	if err != nil && len(entries) == 0 {
		return nil, err
	}

	var results []ToolEntry
	lowerQuery := strings.ToLower(query)

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Tool.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(entry.Tool.Description), lowerQuery) {
			results = append(results, entry)
		}
	}

	return results, err
}
