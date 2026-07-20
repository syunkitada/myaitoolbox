package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/application"
)

var (
	transport string
	host      string
	port      string
	logLevel  string
)

var rootCmd = &cobra.Command{
	Use:     "mcpserve <server>",
	Short:   "MCP Server Runtime",
	Long:    "mcpserve is a runtime for MCP (Model Context Protocol) servers.",
	Args:    cobra.ExactArgs(1),
	Version: "0.0.1",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var names []string
		for _, p := range application.List() {
			names = append(names, p.Name())
		}
		return names, cobra.ShellCompDirectiveNoFileComp
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
	serverName := args[0]
	p, exists := application.Get(serverName)
	if !exists {
		return fmt.Errorf("server %q not found", serverName)
	}

	srv := p.NewServer()
	slog.Info("Starting server", "server", serverName, "transport", transport)

	if transport == "http" {
		addr := fmt.Sprintf("%s:%s", host, port)
		handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return srv.MCP()
		}, nil)
		slog.Info("MCP HTTP server listening", "addr", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			slog.Error("HTTP server error", "error", err)
			return err
		}
	} else {
		if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			slog.Error("Server error", "error", err)
			return err
		}
	}
	return nil
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
