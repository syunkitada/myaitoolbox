package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

var infoCmd = &cobra.Command{
	Use:   "info [server/tool]",
	Short: "Show detailed info about a tool",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		toolPath := args[0]
		serverName, toolName, err := discovery.ParseToolName(toolPath)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		entry, err := discovery.GetToolInfo(context.Background(), p, serverName, toolName)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("Name:")
		fmt.Printf("  %s/%s\n\n", entry.ServerName, entry.Tool.Name)
		fmt.Println("Description:")
		fmt.Printf("  %s\n\n", entry.Tool.Description)

		fmt.Println("Parameters:")
		if schema, ok := entry.Tool.InputSchema.(map[string]interface{}); ok {
			if schemaType, _ := schema["type"].(string); schemaType == "object" {
				props, _ := schema["properties"].(map[string]interface{})
				requiredRaw, _ := schema["required"].([]interface{})
				var requiredFields []string
				for _, r := range requiredRaw {
					if s, ok := r.(string); ok {
						requiredFields = append(requiredFields, s)
					}
				}

				isRequired := func(field string) bool {
					for _, r := range requiredFields {
						if r == field {
							return true
						}
					}
					return false
				}

				for paramName, paramSchemaRaw := range props {
					fmt.Printf("\n  %s\n", paramName)
					paramSchema, ok := paramSchemaRaw.(map[string]interface{})
					if ok {
						if typ, ok := paramSchema["type"]; ok {
							fmt.Printf("    Type: %v\n", typ)
						}
					}
					fmt.Printf("    Required: %t\n", isRequired(paramName))
				}
			}
		}

		fmt.Println("\nExamples:")
		fmt.Printf("\n  mcpctl call %s/%s \\\n", entry.ServerName, entry.Tool.Name)
		fmt.Printf("    --params '{\"key\":\"value\"}'\n")
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}
