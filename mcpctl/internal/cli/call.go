package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/discovery"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/runtime"
)

var callCmd = &cobra.Command{
	Use:                "call <server/tool> [flags]",
	Short:              "Call a tool",
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

		// extract global flags (profile and output format)
		profName := ""
		outputFormat := "raw"
		for i := 1; i < len(args); i++ {
			if args[i] == "--profile" || args[i] == "-p" {
				if i+1 < len(args) {
					profName = args[i+1]
					delete(params, "profile")
					delete(params, "p")
				}
			}
			if args[i] == "--output" || args[i] == "-o" {
				if i+1 < len(args) {
					outputFormat = args[i+1]
					delete(params, "output")
					delete(params, "o")
				}
			}
		}

		// validate output format
		switch outputFormat {
		case "raw", "tsv", "table":
			// valid
		default:
			fmt.Printf("Error: unsupported output format %q. Supported: raw, tsv, table\n", outputFormat)
			return
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
			if txt, ok := c.(*mcp.TextContent); ok {
				printText(txt.Text, outputFormat)
			} else if im, ok := c.(*mcp.ImageContent); ok {
				fmt.Printf("[Image %s]\n", im.MIMEType)
			} else {
				b, _ := json.MarshalIndent(c, "", "  ")
				fmt.Println(string(b))
			}
		}
	},
}

// printText prints the text in the specified output format.
// It tries to parse the text as JSON to enable structured output (tsv, table).
// If JSON parsing fails, it falls back to printing the raw text.
func printText(text string, format string) {
	if format == "raw" {
		fmt.Println(text)
		return
	}

	// Try to parse as JSON for structured output
	var parsed interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		// Not JSON — fall back to raw output
		fmt.Println(text)
		return
	}

	switch format {
	case "tsv":
		printTSV(parsed)
	case "table":
		printTable(parsed)
	}
}

// printTSV prints JSON data as TSV (Tab-Separated Values).
// Supports: array of objects, array of scalars, single object, scalar value.
func printTSV(data interface{}) {
	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		// array of objects: print header row then data rows
		if row, ok := v[0].(map[string]interface{}); ok {
			headers := objectKeys(row)
			fmt.Println(strings.Join(headers, "\t"))
			for _, item := range v {
				if obj, ok := item.(map[string]interface{}); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj[h])
					}
					fmt.Println(strings.Join(vals, "\t"))
				}
			}
		} else {
			// array of scalars
			for _, item := range v {
				fmt.Printf("%v\n", item)
			}
		}
	case map[string]interface{}:
		// single object: print key\tvalue rows
		for k, val := range v {
			fmt.Printf("%s\t%v\n", k, val)
		}
	default:
		// scalar
		fmt.Println(v)
	}
}

// printTable prints JSON data as an aligned table using tabwriter.
// Supports: array of objects, array of scalars, single object, scalar value.
func printTable(data interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		// array of objects: print header row then data rows
		if row, ok := v[0].(map[string]interface{}); ok {
			headers := objectKeys(row)
			fmt.Fprintln(w, strings.Join(headers, "\t"))
			// separator line
			seps := make([]string, len(headers))
			for i, h := range headers {
				seps[i] = strings.Repeat("-", len(h))
			}
			fmt.Fprintln(w, strings.Join(seps, "\t"))
			for _, item := range v {
				if obj, ok := item.(map[string]interface{}); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj[h])
					}
					fmt.Fprintln(w, strings.Join(vals, "\t"))
				}
			}
		} else {
			// array of scalars
			for _, item := range v {
				fmt.Fprintf(w, "%v\n", item)
			}
		}
	case map[string]interface{}:
		// single object: print KEY\tVALUE table
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for k, val := range v {
			fmt.Fprintf(w, "%s\t%v\n", k, val)
		}
	default:
		fmt.Fprintln(w, v)
	}
}

// objectKeys returns the keys of a JSON object in a stable order.
// Keys are collected from the first occurrence, preserving insertion order where possible.
func objectKeys(obj map[string]interface{}) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

func init() {
	RootCmd.AddCommand(callCmd)
}
