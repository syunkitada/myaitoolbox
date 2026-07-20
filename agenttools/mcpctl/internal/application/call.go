package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

func CallTool(ctx context.Context, executor domain.ToolExecutor, discovery domain.ToolDiscovery, prof *domain.Profile, toolPath string, params map[string]interface{}, outputFormat string) (*mcp.CallToolResult, error) {
	serverName, toolName, err := domain.ParseToolName(toolPath)
	if err != nil {
		return nil, err
	}

	return executor.CallTool(ctx, prof, serverName, toolName, params)
}

func FormatOutput(res *mcp.CallToolResult, outputFormat string) {
	if res.IsError {
		fmt.Println("Tool execution returned an error:")
	}

	if outputFormat == "raw" {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else if res.StructuredContent != nil && (outputFormat == "tsv" || outputFormat == "table") {
		switch outputFormat {
		case "tsv":
			mcpclient.PrintTSV(res.StructuredContent)
		case "table":
			mcpclient.PrintTable(res.StructuredContent)
		}
		if m, ok := res.StructuredContent.(map[string]interface{}); ok {
			if meta, ok := m["meta"].(map[string]interface{}); ok {
				if outputs, ok := meta["outputs"].([]interface{}); ok {
					fmt.Fprintln(os.Stderr)
					for _, o := range outputs {
						if key, ok := o.(string); ok {
							if val, ok := meta[key]; ok {
								b, _ := json.Marshal(val)
								fmt.Fprintf(os.Stderr, "%s: %s\n", key, string(b))
							} else {
								fmt.Fprintf(os.Stderr, "Warning: key %q specified in outputs not found in meta\n", key)
							}
						}
					}
				}
			}
		}
	} else {
		for _, c := range res.Content {
			if txt, ok := c.(*mcp.TextContent); ok {
				mcpclient.PrintText(txt.Text, outputFormat)
			} else if im, ok := c.(*mcp.ImageContent); ok {
				fmt.Printf("[Image %s]\n", im.MIMEType)
			} else {
				b, _ := json.MarshalIndent(c, "", "  ")
				fmt.Println(string(b))
			}
		}
	}
}

func ParseCallArgs(args []string, discovery domain.ToolDiscovery, prof *domain.Profile) (toolPath string, params map[string]interface{}, outputFormat string, err error) {
	outputFormat = "tsv"

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--output" || arg == "-o" {
			if i+1 < len(args) {
				outputFormat = args[i+1]
				i++
			}
			continue
		}

		if arg == "--params" && i+1 < len(args) {
			val := args[i+1]
			i++
			params = make(map[string]interface{})
			if strings.HasPrefix(val, "{") {
				if err := json.Unmarshal([]byte(val), &params); err != nil {
					return "", nil, "", fmt.Errorf("parsing params JSON: %w", err)
				}
			} else {
				data, err := os.ReadFile(val)
				if err != nil {
					return "", nil, "", fmt.Errorf("reading params file: %w", err)
				}
				if err := json.Unmarshal(data, &params); err != nil {
					return "", nil, "", fmt.Errorf("parsing params JSON from file: %w", err)
				}
			}
			continue
		}

		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if key == "profile" || key == "p" {
				if i+1 < len(args) {
					i++
				}
				continue
			}
			if params == nil {
				params = make(map[string]interface{})
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				val := args[i+1]
				paramTypes := mcpclient.GetParamTypes(prof, "", "", discovery)
				if paramTypes[key] == "array" {
					params[key] = mcpclient.ParseArrayArg(val, params[key])
				} else {
					params[key] = val
				}
				i++
			} else {
				params[key] = true
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			key := strings.TrimPrefix(arg, "-")
			if key == "o" || key == "p" {
				if i+1 < len(args) {
					if key == "o" {
						outputFormat = args[i+1]
					}
					i++
				}
				continue
			}
			if params == nil {
				params = make(map[string]interface{})
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				val := args[i+1]
				paramTypes := mcpclient.GetParamTypes(prof, "", "", discovery)
				if paramTypes[key] == "array" {
					params[key] = mcpclient.ParseArrayArg(val, params[key])
				} else {
					params[key] = val
				}
				i++
			} else {
				params[key] = true
			}
		} else if toolPath == "" && !strings.HasPrefix(arg, "-") {
			toolPath = arg
		}
	}

	if params == nil {
		params = make(map[string]interface{})
	}

	return toolPath, params, outputFormat, nil
}

func ValidateOutputFormat(format string) error {
	switch format {
	case "raw", "tsv", "table":
		return nil
	default:
		return fmt.Errorf("unsupported output format %q. Supported: raw, tsv, table", format)
	}
}

func FormatParamList(entry *domain.ToolEntry) string {
	schema, ok := entry.Tool.InputSchema.(map[string]interface{})
	if !ok {
		return "(no parameters)"
	}

	props, _ := schema["properties"].(map[string]interface{})
	if len(props) == 0 {
		return "(no parameters)"
	}

	requiredRaw, _ := schema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, r := range requiredRaw {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}

	var out string
	for name, propRaw := range props {
		prop, _ := propRaw.(map[string]interface{})
		typ, _ := prop["type"].(string)
		if typ == "" {
			typ = "any"
		}
		if typ == "array" {
			if items, ok := prop["items"].(map[string]interface{}); ok {
				if itemType, ok := items["type"].(string); ok {
					typ = "array[" + itemType + "]"
				}
			}
		}
		req := ""
		if requiredSet[name] {
			req = " (required)"
		}
		out += fmt.Sprintf("  %s: %s%s\n", name, typ, req)
	}
	return out
}
