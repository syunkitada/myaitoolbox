package entrypoint

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
	infraMcpserver "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpserver"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server",
	Run: func(cmd *cobra.Command, args []string) {
		discovery := mcpclient.NewToolDiscovery()
		executor := mcpclient.NewToolExecutor()
		resolver := infraProfile.NewResolver()

		s := infraMcpserver.NewServer(discovery, executor, resolver)
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}
