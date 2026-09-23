package entrypoint

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for a tool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			return err
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entries, err := application.SearchTools(cmd.Context(), discovery, p, query)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			fmt.Printf("%s/%s\n", entry.ServerName, entry.Tool.Name)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(searchCmd)
}
