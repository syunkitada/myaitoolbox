package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mark3labs/mcp-go/server"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/mcpserver"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server",
	Run: func(cmd *cobra.Command, args []string) {
		s := mcpserver.NewServer()
		
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}
