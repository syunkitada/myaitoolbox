package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/runtime"
)

var callCmd = &cobra.Command{
	Use:   "call <server/tool> [flags]",
	Short: "Call a tool",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Usage: mcpctl call <server/tool> [flags]")
			return
		}

		// Handle Human Shortcut: `mcpctl call -l` or `mcpctl call server -l` or `mcpctl call server/tool -l`
		if len(args) == 1 && args[0] == "-l" {
			listCmd.Run(listCmd, []string{})
			return
		}

		if len(args) == 2 && args[1] == "-l" {
			if strings.Contains(args[0], "/") {
				infoCmd.Run(infoCmd, []string{args[0]})
			} else {
				listCmd.Run(listCmd, []string{args[0]})
			}
			return
		}

		toolPath := args[0]
		if strings.HasPrefix(toolPath, "--") || strings.HasPrefix(toolPath, "-") {
			fmt.Println("Usage: mcpctl call <server/tool> [flags]")
			return
		}

		serverName, toolName, err := discovery.ParseToolName(toolPath)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		params := make(map[string]interface{})

		// parse args
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if arg == "--params" && i+1 < len(args) {
				val := args[i+1]
				i++
				if strings.HasPrefix(val, "{") {
					if err := json.Unmarshal([]byte(val), &params); err != nil {
						fmt.Println("Error parsing params JSON:", err)
						return
					}
				} else {
					data, err := os.ReadFile(val)
					if err != nil {
						fmt.Println("Error reading params file:", err)
						return
					}
					if err := json.Unmarshal(data, &params); err != nil {
						fmt.Println("Error parsing params JSON from file:", err)
						return
					}
				}
				continue
			}

			if strings.HasPrefix(arg, "--") {
				key := strings.TrimPrefix(arg, "--")
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					params[key] = args[i+1]
					i++
				} else {
					params[key] = true
				}
			} else if strings.HasPrefix(arg, "-") {
				key := strings.TrimPrefix(arg, "-")
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					params[key] = args[i+1]
					i++
				} else {
					params[key] = true
				}
			}
		}

		// extract global flags
		profName := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--profile" || args[i] == "-p" {
				if i+1 < len(args) {
					profName = args[i+1]
					delete(params, "profile")
					delete(params, "p")
				}
			}
		}

		p, err := profile.ResolveProfile(profName, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		res, err := runtime.CallTool(context.Background(), p, serverName, toolName, params)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if res.IsError {
			fmt.Println("Tool execution returned an error:")
		}
		
		for _, c := range res.Content {
			if txt, ok := c.(mcp.TextContent); ok {
				fmt.Println(txt.Text)
			} else if im, ok := c.(mcp.ImageContent); ok {
				fmt.Printf("[Image %s]\n", im.MIMEType)
			} else {
				b, _ := json.MarshalIndent(c, "", "  ")
				fmt.Println(string(b))
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(callCmd)
}
