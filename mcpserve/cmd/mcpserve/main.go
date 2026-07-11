package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"

	_ "github.com/syunkitada/myaitoolbox/mcpserve/internal/application"
)

var (
	transport = flag.String("transport", "stdio", "transport to use: stdio or http")
	host      = flag.String("host", "localhost", "host to listen on (for http transport)")
	port      = flag.String("port", "8080", "port to listen on (for http transport)")
	version   = flag.Bool("v", false, "print version")
	versionL  = flag.Bool("version", false, "print version")
)

func printGlobalHelp() {
	fmt.Println("mcpserve - MCP Server Runtime")
	fmt.Println("\nUsage:")
	fmt.Println("  mcpserve [options] <server>")
	fmt.Println("\nAvailable servers:")
	providers := domain.List()
	for _, p := range providers {
		fmt.Printf("  %-11s %s\n", p.Name(), p.Description())
	}
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
}

func printServerHelp(name string) {
	p, exists := domain.Get(name)
	if !exists {
		fmt.Printf("Error: server %q not found\n", name)
		os.Exit(1)
	}
	fmt.Printf("%s\n\n", p.Description())
	fmt.Println("Description:")
	fmt.Printf("  %s\n\n", p.Description())
	fmt.Println("Available tools are configured in the server.")
}

func main() {
	// Initialize default slog handler to output JSON to stderr
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	if err := godotenv.Load(); err != nil {
		// .env file not found or error reading it; proceed with existing env
	}

	flag.Usage = printGlobalHelp
	flag.Parse()

	if *version || *versionL {
		fmt.Println("mcpserve version 0.0.1")
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		printGlobalHelp()
		os.Exit(0)
	}

	serverName := args[0]
	if serverName == "help" {
		if len(args) > 1 {
			printServerHelp(args[1])
		} else {
			printGlobalHelp()
		}
		os.Exit(0)
	}

	p, exists := domain.Get(serverName)
	if !exists {
		fmt.Printf("Error: server %q not found\n", serverName)
		os.Exit(1)
	}

	srv := p.NewServer()
	fmt.Printf("Starting server %q with transport %q\n", serverName, *transport)

	if *transport == "http" {
		addr := fmt.Sprintf("%s:%s", *host, *port)
		handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return srv.MCP()
		}, nil)
		slog.Info("MCP HTTP server listening", "addr", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	} else {
		// stdio
		if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}
