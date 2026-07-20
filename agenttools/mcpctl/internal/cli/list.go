package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

var listCmd = &cobra.Command{
	Use:   "list [server]",
	Short: "List available tools",
	Run: func(cmd *cobra.Command, args []string) {
		serverFilter := ""
		if len(args) > 0 {
			serverFilter = args[0]
		}

		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		entries, err := discovery.ListTools(context.Background(), p, serverFilter)
		if err != nil {
			fmt.Println("Error:", err)
			// Continue to show what we have
		}

		for _, entry := range entries {
			fmt.Printf("%s/%s\n", entry.ServerName, entry.Tool.Name)
		}
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
