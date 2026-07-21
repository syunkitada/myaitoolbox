package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/entrypoint"
)

var (
	transport string
	host      string
	port      string
	logLevel  string
)

var registry = entrypoint.NewRegistryWithProviders()

var rootCmd = &cobra.Command{
	Use:     "mcpserve <server>",
	Short:   "MCP Server Runtime",
	Long:    "mcpserve is a runtime for MCP (Model Context Protocol) servers.",
	Args:    cobra.ExactArgs(1),
	Version: "0.0.1",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return registry.ListNames(), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: runServer,
}

func initLogger() {
	var lvl slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}

func runServer(cmd *cobra.Command, args []string) error {
	return entrypoint.Run(registry, args[0], transport, host, port)
}

func main() {
	rootCmd.PersistentFlags().StringVar(&transport, "transport", "stdio", "transport to use: stdio or http")
	rootCmd.PersistentFlags().StringVar(&host, "host", "localhost", "host to listen on (for http transport)")
	rootCmd.PersistentFlags().StringVar(&port, "port", "8080", "port to listen on (for http transport)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")

	if err := godotenv.Load(); err != nil {
		// .env file not found or error reading it; proceed with existing env
	}

	initLogger()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
