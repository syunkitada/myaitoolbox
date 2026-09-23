package entrypoint

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var listCmd = &cobra.Command{
	Use:   "list [server]",
	Short: "List available tools",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverFilter := ""
		if len(args) > 0 {
			serverFilter = args[0]
		}
		return runList(cmd.Context(), profileFlag, serverFilter)
	},
}

func runList(ctx context.Context, selectedProfile, serverFilter string) error {
	resolver := infraProfile.NewResolver()
	p, err := resolver.Resolve(selectedProfile, "")
	if err != nil {
		return err
	}

	discovery := mcpclientInfra.NewToolDiscovery()
	entries, err := application.ListTools(ctx, discovery, p, serverFilter)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Printf("%s/%s\n", entry.ServerName, entry.Tool.Name)
	}
	return nil
}

func init() {
	RootCmd.AddCommand(listCmd)
}
