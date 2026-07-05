package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
		humanFlag := ""
		if len(args) >= 1 && (args[len(args)-1] == "-l" || args[len(args)-1] == "-h") {
			humanFlag = args[len(args)-1]
		}
		if humanFlag != "" {
			target := ""
			if len(args) >= 2 {
				target = args[0]
			}
			if target == "" {
				listCmd.Run(listCmd, []string{})
			} else if strings.Contains(target, "/") {
				printParamList(target)
			} else {
				listCmd.Run(listCmd, []string{target})
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
		outputFormat := "tsv"
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

// printParamList prints parameter names with type and required info.
func printParamList(toolPath string) {
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

	schema, ok := entry.Tool.InputSchema.(map[string]interface{})
	if !ok {
		return
	}

	props, _ := schema["properties"].(map[string]interface{})
	if len(props) == 0 {
		fmt.Println("(no parameters)")
		return
	}

	requiredRaw, _ := schema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, r := range requiredRaw {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}

	for name, propRaw := range props {
		prop, _ := propRaw.(map[string]interface{})
		typ, _ := prop["type"].(string)
		if typ == "" {
			typ = "any"
		}
		req := ""
		if requiredSet[name] {
			req = " (required)"
		}
		fmt.Printf("  %s: %s%s\n", name, typ, req)
	}
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
	parsed, err := decodeJSON([]byte(text))
	if err != nil {
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
	data = extractDataArray(data)
	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		// array of objects: print header row then data rows
		if row, ok := v[0].(orderedMap); ok {
			headers := row.keys()
			fmt.Println(strings.Join(headers, "\t"))
			for _, item := range v {
				if obj, ok := item.(orderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj.get(h))
					}
					fmt.Println(strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := orderedKeys(row)
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
	case orderedMap:
		// single object: print key\tvalue rows
		for _, e := range v {
			fmt.Printf("%s\t%v\n", e.Key, e.Value)
		}
	case map[string]interface{}:
		// single object: print key\tvalue rows
		for _, k := range orderedKeys(v) {
			fmt.Printf("%s\t%v\n", k, v[k])
		}
	default:
		// scalar
		fmt.Println(v)
	}
}

// printTable prints JSON data as an aligned table using tabwriter.
// Supports: array of objects, array of scalars, single object, scalar value.
func printTable(data interface{}) {
	data = extractDataArray(data)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		// array of objects: print header row then data rows
		if row, ok := v[0].(orderedMap); ok {
			headers := row.keys()
			fmt.Fprintln(w, strings.Join(headers, "\t"))
			// separator line
			seps := make([]string, len(headers))
			for i, h := range headers {
				seps[i] = strings.Repeat("-", len(h))
			}
			fmt.Fprintln(w, strings.Join(seps, "\t"))
			for _, item := range v {
				if obj, ok := item.(orderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj.get(h))
					}
					fmt.Fprintln(w, strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := orderedKeys(row)
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
	case orderedMap:
		// single object: print KEY\tVALUE table
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for _, e := range v {
			fmt.Fprintf(w, "%s\t%v\n", e.Key, e.Value)
		}
	case map[string]interface{}:
		// single object: print KEY\tVALUE table
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for _, k := range orderedKeys(v) {
			fmt.Fprintf(w, "%s\t%v\n", k, v[k])
		}
	default:
		fmt.Fprintln(w, v)
	}
}

// extractDataArray unwraps a response object: if the data is a map with a
// "data" key whose value is an array, it returns that array. Otherwise it
// returns the original data unchanged.
func extractDataArray(data interface{}) interface{} {
	if obj, ok := data.(orderedMap); ok {
		for _, e := range obj {
			if e.Key == "data" {
				if arr, ok := e.Value.([]interface{}); ok {
					return arr
				}
			}
		}
	}
	if obj, ok := data.(map[string]interface{}); ok {
		if arr, ok := obj["data"].([]interface{}); ok {
			return arr
		}
	}
	return data
}

// orderedMap preserves JSON object key order.
type orderedMap []orderedMapEntry

type orderedMapEntry struct {
	Key   string
	Value interface{}
}

func (om orderedMap) keys() []string {
	ks := make([]string, len(om))
	for i, e := range om {
		ks[i] = e.Key
	}
	return ks
}

func (om orderedMap) get(key string) interface{} {
	for _, e := range om {
		if e.Key == key {
			return e.Value
		}
	}
	return nil
}

// decodeJSON decodes JSON into an ordered representation.
// Objects become orderedMap (preserving key order), arrays become []interface{},
// and scalars are represented as their natural Go types.
func decodeJSON(b []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return decodeValue(dec)
}

func decodeValue(dec *json.Decoder) (interface{}, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch delim := t.(type) {
	case json.Delim:
		switch delim {
		case '{':
			om := orderedMap{}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key := keyToken.(string)
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				om = append(om, orderedMapEntry{Key: key, Value: val})
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return om, nil
		case '[':
			arr := []interface{}{}
			for dec.More() {
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
	}

	return t, nil
}

// orderedKeys returns map keys sorted alphabetically (fallback when no orderedMap available).
func orderedKeys(obj map[string]interface{}) []string {
	ks := make([]string, 0, len(obj))
	for k := range obj {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func init() {
	RootCmd.AddCommand(callCmd)
}
