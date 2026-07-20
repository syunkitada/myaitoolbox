package entrypoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

var callCmd = &cobra.Command{
	Use:                "call <server/tool> [flags]",
	Short:              "Call a tool",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Usage: mcpctl call <server/tool> [flags]")
			return
		}

		humanFlag := ""
		if len(args) >= 1 && (args[len(args)-1] == "-l" || args[len(args)-1] == "-h") {
			humanFlag = args[len(args)-1]
		}
		if humanFlag != "" {
			target := ""
			if len(args) >= 2 {
				target = args[0]
			}
			if target == "" {
				listCmd.Run(listCmd, []string{})
			} else if strings.Contains(target, "/") {
				printParamList(target)
			} else {
				listCmd.Run(listCmd, []string{target})
			}
			return
		}

		toolPath := args[0]
		if strings.HasPrefix(toolPath, "--") || strings.HasPrefix(toolPath, "-") {
			fmt.Println("Usage: mcpctl call <server/tool> [flags]")
			return
		}

		resolver := infraProfile.NewResolver()
		profName := ""
		for i := 1; i < len(args); i++ {
			if (args[i] == "--profile" || args[i] == "-p") && i+1 < len(args) {
				profName = args[i+1]
			}
		}

		p, err := resolver.Resolve(profName, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		parsedToolPath, params, outputFormat, err := application.ParseCallArgs(args[1:], discovery, p)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if parsedToolPath != "" {
			toolPath = parsedToolPath
		}

		if err := application.ValidateOutputFormat(outputFormat); err != nil {
			fmt.Println("Error:", err)
			return
		}

		executor := mcpclientInfra.NewToolExecutor()
		res, err := application.CallTool(context.Background(), executor, discovery, p, toolPath, params, outputFormat)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		application.FormatOutput(res, outputFormat)
	},
}

func printParamList(toolPath string) {
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

	fmt.Print(application.FormatParamList(entry))
}

func init() {
	RootCmd.AddCommand(callCmd)
}
