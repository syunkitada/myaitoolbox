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
	slog.Debug("listing tools", "profile", prof.Name, "server_filter", serverFilter)

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
			defer client.Close()

			res, err := client.ListTools(ctx, &mcp.ListToolsParams{})
			if err != nil {
				errCh <- fmt.Errorf("server %s: failed to list tools: %w", name, err)
				return
			}

			mu.Lock()
			for _, t := range res.Tools {
				entries = append(entries, domain.ToolEntry{
					ServerName: name,
					Tool:       t,
				})
			}
			mu.Unlock()
		}(srvName, srvConfig)
	}

	wg.Wait()
	close(errCh)

	var errors []string
	for err := range errCh {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return entries, fmt.Errorf("some servers failed: %s", strings.Join(errors, "; "))
	}

	sort.Slice(entries, func(i, j int) bool {
		nameI := fmt.Sprintf("%s/%s", entries[i].ServerName, entries[i].Tool.Name)
		nameJ := fmt.Sprintf("%s/%s", entries[j].ServerName, entries[j].Tool.Name)
		return nameI < nameJ
	})

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
