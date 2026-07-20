package entrypoint

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

var infoCmd = &cobra.Command{
	Use:   "info [server/tool]",
	Short: "Show detailed info about a tool",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		toolPath := args[0]
		serverName, toolName, err := domain.ParseToolName(toolPath)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entry, err := application.GetToolInfo(context.Background(), discovery, p, serverName, toolName)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("Name:")
		fmt.Printf("  %s/%s\n\n", entry.ServerName, entry.Tool.Name)
		fmt.Println("Description:")
		fmt.Printf("  %s\n\n", entry.Tool.Description)

		fmt.Println("Parameters:")
		fmt.Println(application.FormatParamList(entry))

		fmt.Println("Examples:")
		fmt.Printf("\n  mcpctl call %s/%s \\\n", entry.ServerName, entry.Tool.Name)
		fmt.Printf("    --params '{\"key\":\"value\"}'\n")
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}
