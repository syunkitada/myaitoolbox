package entrypoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var callCmd = &cobra.Command{
	Use:                "call <server/tool> [flags]",
	Short:              "Call a tool",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: mcpctl call <server/tool> [flags]")
		}

		humanFlag := ""
		if len(args) >= 1 && (args[len(args)-1] == "-l" || args[len(args)-1] == "-h") {
			humanFlag = args[len(args)-1]
		}
		if humanFlag != "" {
			target, selectedProfile, err := application.ParseCallTarget(args)
			if err != nil {
				return err
			}
			if selectedProfile == "" {
				selectedProfile = profileFlag
			}
			if target == "" {
				return runList(cmd.Context(), selectedProfile, "")
			} else if strings.Contains(target, "/") {
				return printParamList(cmd.Context(), target, selectedProfile)
			} else {
				return runList(cmd.Context(), selectedProfile, target)
			}
		}

		resolver := infraProfile.NewResolver()
		_, profName, err := application.ParseCallTarget(args)
		if err != nil {
			return err
		}
		if profName == "" {
			profName = profileFlag
		}

		cfg, err := resolver.LoadConfig()
		if err != nil {
			return err
		}
		p, err := resolver.Resolve(profName, "")
		if err != nil {
			return err
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		defaultOutputFormat := cfg.Output.Format
		if defaultOutputFormat == "" {
			defaultOutputFormat = "table"
		}
		toolPath, params, outputFormat, _, err := application.ParseCallArgsContext(cmd.Context(), args, discovery, p, defaultOutputFormat)
		if err != nil {
			return err
		}

		if err := application.ValidateOutputFormat(outputFormat); err != nil {
			return err
		}

		executor := mcpclientInfra.NewToolExecutor()
		res, err := application.CallTool(cmd.Context(), executor, discovery, p, toolPath, params, outputFormat)
		if err != nil {
			return err
		}

		return application.FormatOutput(res, outputFormat)
	},
}

func printParamList(ctx context.Context, toolPath, selectedProfile string) error {
	serverName, toolName, err := domain.ParseToolName(toolPath)
	if err != nil {
		return err
	}

	resolver := infraProfile.NewResolver()
	p, err := resolver.Resolve(selectedProfile, "")
	if err != nil {
		return err
	}

	discovery := mcpclientInfra.NewToolDiscovery()
	entry, err := application.GetToolInfo(ctx, discovery, p, serverName, toolName)
	if err != nil {
		return err
	}

	fmt.Print(application.FormatParamList(entry))
	return nil
}

func init() {
	RootCmd.AddCommand(callCmd)
}
