package entrypoint

import (
	"context"

	"github.com/spf13/cobra"
)

var profileFlag string

var RootCmd = &cobra.Command{
	Use:   "mymcp",
	Short: "A CLI tool for interacting with MCP servers",
	Long: `mymcp is a CLI that helps humans and AI interact with other MCP servers.
	It allows listing, searching, getting info about, and calling tools on remote MCP servers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return ExecuteContext(context.Background())
}

func ExecuteContext(ctx context.Context) error {
	return RootCmd.ExecuteContext(ctx)
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "profile to use")
}
