package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for a tool",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]

		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		entries, err := discovery.SearchTools(context.Background(), p, query)
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
	RootCmd.AddCommand(searchCmd)
}
