package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

// __list_tools: prints all available "server/tool" entries, one per line.
// Used internally by the zsh completion script.
var listToolsHelperCmd = &cobra.Command{
	Use:    "__list_tools",
	Short:  "List tools for shell completion (internal)",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			return
		}

		entries, err := discovery.ListTools(context.Background(), p, "")
		if err != nil && len(entries) == 0 {
			return
		}

		for _, entry := range entries {
			tool := entry.Tool
			desc := tool.Description
			// Trim description to a single line for zsh display
			for i, c := range desc {
				if c == '\n' {
					desc = desc[:i]
					break
				}
			}
			if desc != "" {
				fmt.Printf("%s/%s:%s\n", entry.ServerName, tool.Name, desc)
			} else {
				fmt.Printf("%s/%s\n", entry.ServerName, tool.Name)
			}
		}
	},
}

// __list_params: prints parameter names for a given "server/tool", one per line.
// Used internally by the zsh completion script.
var listParamsHelperCmd = &cobra.Command{
	Use:    "__list_params <server/tool>",
	Short:  "List params for shell completion (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		toolPath := args[0]
		serverName, toolName, err := discovery.ParseToolName(toolPath)
		if err != nil {
			return
		}

		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			return
		}

		entry, err := discovery.GetToolInfo(context.Background(), p, serverName, toolName)
		if err != nil {
			return
		}

		schema, ok := entry.Tool.InputSchema.(map[string]interface{})
		if !ok {
			return
		}

		schemaType, _ := schema["type"].(string)
		if schemaType != "object" {
			return
		}

		props, _ := schema["properties"].(map[string]interface{})
		requiredRaw, _ := schema["required"].([]interface{})
		requiredSet := make(map[string]bool)
		for _, r := range requiredRaw {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}

		for paramName, paramSchemaRaw := range props {
			paramSchema, ok := paramSchemaRaw.(map[string]interface{})
			desc := ""
			if ok {
				if d, ok := paramSchema["description"].(string); ok {
					// single line
					for i, c := range d {
						if c == '\n' {
							d = d[:i]
							break
						}
					}
					desc = d
				}
				// If enum, show possible values in description
				if enum, ok := paramSchema["enum"].([]interface{}); ok {
					b, _ := json.Marshal(enum)
					desc = fmt.Sprintf("one of %s", string(b))
				}
			}
			suffix := ""
			if requiredSet[paramName] {
				suffix = " (required)"
			}
			if desc != "" {
				fmt.Printf("--%s:%s%s\n", paramName, desc, suffix)
			} else {
				fmt.Printf("--%s\n", paramName)
			}
		}

		// Always offer -o and -p
		fmt.Println("-o:output format (raw, tsv, table)")
		fmt.Println("-p:profile to use")
	},
}

func init() {
	RootCmd.AddCommand(listToolsHelperCmd)
	RootCmd.AddCommand(listParamsHelperCmd)
}
