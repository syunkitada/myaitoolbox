package entrypoint

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
	mcpclientInfra "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

var listToolsHelperCmd = &cobra.Command{
	Use:    "__list_tools",
	Short:  "List tools for shell completion (internal)",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entries, err := discovery.ListTools(context.Background(), p, "")
		if err != nil && len(entries) == 0 {
			return
		}

		for _, entry := range entries {
			tool := entry.Tool
			desc := tool.Description
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

var listParamsHelperCmd = &cobra.Command{
	Use:    "__list_params <server/tool>",
	Short:  "List params for shell completion (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		toolPath := args[0]
		serverName, toolName, err := domain.ParseToolName(toolPath)
		if err != nil {
			return
		}

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
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
					for i, c := range d {
						if c == '\n' {
							d = d[:i]
							break
						}
					}
					desc = d
				}
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

		fmt.Println("-o:output format (raw, tsv, table)")
		fmt.Println("-p:profile to use")
	},
}

var listParamValuesHelperCmd = &cobra.Command{
	Use:    "__list_param_values <server/tool> <paramName>",
	Short:  "List param values for shell completion (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		toolPath := args[0]
		paramName := args[1]
		serverName, toolName, err := domain.ParseToolName(toolPath)
		if err != nil {
			return
		}

		resolver := infraProfile.NewResolver()
		p, err := resolver.Resolve(profileFlag, "")
		if err != nil {
			return
		}

		discovery := mcpclientInfra.NewToolDiscovery()
		entry, err := discovery.GetToolInfo(context.Background(), p, serverName, toolName)
		if err != nil {
			return
		}

		schema, ok := entry.Tool.InputSchema.(map[string]interface{})
		if !ok {
			return
		}

		props, _ := schema["properties"].(map[string]interface{})
		paramSchemaRaw, ok := props[paramName]
		if !ok {
			return
		}

		paramSchema, ok := paramSchemaRaw.(map[string]interface{})
		if !ok {
			return
		}

		enum, ok := paramSchema["enum"].([]interface{})
		if !ok {
			return
		}

		for _, v := range enum {
			if s, ok := v.(string); ok {
				fmt.Println(s)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(listToolsHelperCmd)
	RootCmd.AddCommand(listParamsHelperCmd)
	RootCmd.AddCommand(listParamValuesHelperCmd)
}
