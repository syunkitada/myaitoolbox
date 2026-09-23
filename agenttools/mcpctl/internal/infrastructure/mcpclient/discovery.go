package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

type ToolDiscovery struct{}

func NewToolDiscovery() *ToolDiscovery {
	return &ToolDiscovery{}
}

func (d *ToolDiscovery) ListTools(ctx context.Context, prof *domain.Profile, serverFilter string) ([]domain.ToolEntry, error) {
	if prof == nil {
		return nil, fmt.Errorf("profile is required")
	}
	slog.Debug("listing tools", "profile", prof.Name, "server_filter", serverFilter)
	if serverFilter != "" {
		if _, ok := prof.Servers[serverFilter]; !ok {
			return nil, fmt.Errorf("server %s not found in profile %s", serverFilter, prof.Name)
		}
	}

	var entries []domain.ToolEntry
	var mu sync.Mutex
	var wg sync.WaitGroup

	errCh := make(chan error, len(prof.Servers))

	for srvName, srvConfig := range prof.Servers {
		if serverFilter != "" && srvName != serverFilter {
			continue
		}

		wg.Add(1)
		go func(name string, config domain.ServerConfig) {
			defer wg.Done()

			client, err := NewClient(ctx, config)
			if err != nil {
				errCh <- fmt.Errorf("server %s: failed to connect: %w", name, err)
				return
			}
			defer func() { _ = client.Close() }()

			for t, err := range client.Tools(ctx, &mcp.ListToolsParams{}) {
				if err != nil {
					errCh <- fmt.Errorf("server %s: failed to list tools: %w", name, err)
					return
				}
				mu.Lock()
				entries = append(entries, domain.ToolEntry{
					ServerName: name,
					Tool:       t,
				})
				mu.Unlock()
			}
		}(srvName, srvConfig)
	}

	wg.Wait()
	close(errCh)

	var errors []string
	for err := range errCh {
		errors = append(errors, err.Error())
	}
	sort.Strings(errors)
	sort.Slice(entries, func(i, j int) bool {
		nameI := fmt.Sprintf("%s/%s", entries[i].ServerName, entries[i].Tool.Name)
		nameJ := fmt.Sprintf("%s/%s", entries[j].ServerName, entries[j].Tool.Name)
		return nameI < nameJ
	})

	if len(errors) > 0 {
		return entries, fmt.Errorf("some servers failed: %s", strings.Join(errors, "; "))
	}

	return entries, nil
}

func (d *ToolDiscovery) GetToolInfo(ctx context.Context, prof *domain.Profile, serverName, toolName string) (*domain.ToolEntry, error) {
	entries, err := d.ListTools(ctx, prof, serverName)
	if err != nil && len(entries) == 0 {
		return nil, err
	}

	for _, entry := range entries {
		if entry.ServerName == serverName && entry.Tool.Name == toolName {
			return &entry, nil
		}
	}

	return nil, fmt.Errorf("tool %s/%s not found", serverName, toolName)
}

func (d *ToolDiscovery) SearchTools(ctx context.Context, prof *domain.Profile, query string) ([]domain.ToolEntry, error) {
	entries, err := d.ListTools(ctx, prof, "")
	if err != nil && len(entries) == 0 {
		return nil, err
	}

	var results []domain.ToolEntry
	lowerQuery := strings.ToLower(query)

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Tool.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(entry.Tool.Description), lowerQuery) {
			results = append(results, entry)
		}
	}

	return results, err
}
