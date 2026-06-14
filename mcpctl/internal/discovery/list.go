package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpclient "github.com/syunkitada/myaitoolbox/mcpctl/internal/mcpclient"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

type ToolEntry struct {
	ServerName string
	Tool       mcp.Tool
}

// ListTools retrieves all tools from the given profile.
// If serverFilter is not empty, only tools from that server are returned.
func ListTools(ctx context.Context, prof *profile.Profile, serverFilter string) ([]ToolEntry, error) {
	var entries []ToolEntry
	var mu sync.Mutex
	var wg sync.WaitGroup

	errCh := make(chan error, len(prof.Servers))

	for srvName, srvConfig := range prof.Servers {
		if serverFilter != "" && srvName != serverFilter {
			continue
		}

		wg.Add(1)
		go func(name string, config profile.ServerConfig) {
			defer wg.Done()
			
			client, err := mcpclient.NewClient(ctx, config)
			if err != nil {
				errCh <- fmt.Errorf("server %s: failed to connect: %w", name, err)
				return
			}
			defer client.Close()

			res, err := client.ListTools(ctx, mcp.ListToolsRequest{})
			if err != nil {
				errCh <- fmt.Errorf("server %s: failed to list tools: %w", name, err)
				return
			}

			mu.Lock()
			for _, t := range res.Tools {
				entries = append(entries, ToolEntry{
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

	// Sort alphabetically by "ServerName/ToolName"
	sort.Slice(entries, func(i, j int) bool {
		nameI := fmt.Sprintf("%s/%s", entries[i].ServerName, entries[i].Tool.Name)
		nameJ := fmt.Sprintf("%s/%s", entries[j].ServerName, entries[j].Tool.Name)
		return nameI < nameJ
	})

	return entries, nil
}
