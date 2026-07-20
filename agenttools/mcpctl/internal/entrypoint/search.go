package entrypoint

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for a tool",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entries, err := application.SearchTools(context.Background(), discovery, p, query)
		if err != nil {
			fmt.Println("Error:", err)
		}

		for _, entry := range entries {
			fmt.Printf("%s/%s\n", entry.ServerName, entry.Tool.Name)
		}
	},
}

func init() {
	RootCmd.AddCommand(searchCmd)
}
