package entrypoint

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

var listCmd = &cobra.Command{
	Use:   "list [server]",
	Short: "List available tools",
	Run: func(cmd *cobra.Command, args []string) {
		serverFilter := ""
		if len(args) > 0 {
			serverFilter = args[0]
		}

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entries, err := application.ListTools(context.Background(), discovery, p, serverFilter)
		if err != nil {
			fmt.Println("Error:", err)
		}

		for _, entry := range entries {
			fmt.Printf("%s/%s\n", entry.ServerName, entry.Tool.Name)
		}
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
