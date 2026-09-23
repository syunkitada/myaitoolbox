package entrypoint

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var infoCmd = &cobra.Command{
	Use:   "info [server/tool]",
	Short: "Show detailed info about a tool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		toolPath := args[0]
		serverName, toolName, err := domain.ParseToolName(toolPath)
		if err != nil {
			return err
		}

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			return err
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entry, err := application.GetToolInfo(cmd.Context(), discovery, p, serverName, toolName)
		if err != nil {
			return err
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
		return nil
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}
