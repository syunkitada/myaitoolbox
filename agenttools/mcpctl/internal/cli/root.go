package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	profileFlag string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "mcpctl",
	Short: "A CLI tool for interacting with MCP servers",
	Long: `mcpctl is a CLI and MCP server that helps humans and AI interact with other MCP servers.
It allows listing, searching, getting info about, and calling tools on remote MCP servers.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "profile to use")
}
