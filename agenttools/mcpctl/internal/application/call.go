package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/mcpclient"
)

func CallTool(ctx context.Context, executor domain.ToolExecutor, discovery domain.ToolDiscovery, prof *domain.Profile, toolPath string, params map[string]interface{}, outputFormat string) (*mcp.CallToolResult, error) {
	_ = discovery
	_ = outputFormat
	serverName, toolName, err := domain.ParseToolName(toolPath)
	if err != nil {
		return nil, err
	}

	return executor.CallTool(ctx, prof, serverName, toolName, params)
}

func FormatOutput(res *mcp.CallToolResult, outputFormat string) error {
	if res == nil {
		return fmt.Errorf("tool returned an empty result")
	}

	if res.IsError {
		fmt.Fprintln(os.Stderr, "Tool execution returned an error:")
	}

	if outputFormat == "raw" {
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting raw output: %w", err)
		}
		_, err = fmt.Println(string(b))
		return err
	} else if res.StructuredContent != nil && (outputFormat == "tsv" || outputFormat == "table") {
		switch outputFormat {
		case "tsv":
			mcpclient.PrintTSV(res.StructuredContent)
		case "table":
			mcpclient.PrintTable(res.StructuredContent)
		}
		printMetaOutputs(res.StructuredContent)
	} else {
		for _, c := range res.Content {
			if txt, ok := c.(*mcp.TextContent); ok {
				mcpclient.PrintText(txt.Text, outputFormat)
			} else if im, ok := c.(*mcp.ImageContent); ok {
				fmt.Printf("[Image %s]\n", im.MIMEType)
			} else {
				b, err := json.MarshalIndent(c, "", "  ")
				if err != nil {
					return fmt.Errorf("formatting content output: %w", err)
				}
				fmt.Println(string(b))
			}
		}
	}
	return nil
}

type rawParamValue struct {
	value    string
	hasValue bool
}

// ParseCallTarget extracts the positional tool path and profile from a call
// command without connecting to an MCP server. It is used by human shortcuts.
func ParseCallTarget(args []string) (toolPath, profileName string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--profile" || arg == "-p" || arg == "--output" || arg == "-o" || arg == "--params" {
			value, next, valueErr := requiredFlagValue(args, i, arg)
			if valueErr != nil {
				return "", "", valueErr
			}
			if arg == "--profile" || arg == "-p" {
				profileName = value
			}
			i = next
			continue
		}
		if _, ok := inlineFlagValue(arg, "--profile"); ok {
			profileName, _ = inlineFlagValue(arg, "--profile")
			if profileName == "" {
				return "", "", fmt.Errorf("flag %s requires a value", arg)
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			_, value, hasValue := parseLongParam(arg)
			if !hasValue && i+1 < len(args) && canConsumeParamValue(args[i+1]) {
				i++
			}
			_ = value
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			key, value, hasValue := parseShortParam(arg)
			if hasValue && (key == "o" || key == "p") && value == "" {
				return "", "", fmt.Errorf("flag %s requires a value", arg)
			}
			if hasValue && key == "p" {
				profileName = value
			}
			if !hasValue && i+1 < len(args) && canConsumeParamValue(args[i+1]) {
				i++
			}
			continue
		}
		if toolPath != "" {
			return "", "", fmt.Errorf("unexpected positional argument %q", arg)
		}
		toolPath = arg
	}
	return toolPath, profileName, nil
}

// ParseCallArgs parses the complete call command, including the tool path.
// Schema-dependent conversion is performed once after the tool path is known.
func ParseCallArgs(args []string, discovery domain.ToolDiscovery, prof *domain.Profile) (toolPath string, params map[string]interface{}, outputFormat string, err error) {
	toolPath, params, outputFormat, _, err = ParseCallArgsContext(context.Background(), args, discovery, prof, "tsv")
	return
}

func ParseCallArgsContext(ctx context.Context, args []string, discovery domain.ToolDiscovery, prof *domain.Profile, defaultOutputFormat string) (toolPath string, params map[string]interface{}, outputFormat string, profileName string, err error) {
	if defaultOutputFormat == "" {
		defaultOutputFormat = "tsv"
	}
	outputFormat = defaultOutputFormat
	params = make(map[string]interface{})
	rawParams := make(map[string][]rawParamValue)
	afterTerminator := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if afterTerminator {
			if toolPath != "" {
				return "", nil, "", "", fmt.Errorf("unexpected positional argument %q", arg)
			}
			toolPath = arg
			continue
		}
		if arg == "--" {
			afterTerminator = true
			continue
		}

		if arg == "--output" || arg == "-o" {
			value, next, err := requiredFlagValue(args, i, arg)
			if err != nil {
				return "", nil, "", "", err
			}
			outputFormat = value
			i = next
			continue
		}
		if value, ok := inlineFlagValue(arg, "--output"); ok {
			if value == "" {
				return "", nil, "", "", fmt.Errorf("flag %s requires a value", arg)
			}
			outputFormat = value
			continue
		}
		if value, ok := inlineFlagValue(arg, "--params"); ok {
			if err := mergeParams(params, value); err != nil {
				return "", nil, "", "", err
			}
			continue
		}
		if value, ok := inlineFlagValue(arg, "--profile"); ok {
			if value == "" {
				return "", nil, "", "", fmt.Errorf("flag %s requires a value", arg)
			}
			profileName = value
			continue
		}

		if arg == "--params" {
			value, next, err := requiredFlagValue(args, i, arg)
			if err != nil {
				return "", nil, "", "", err
			}
			if err := mergeParams(params, value); err != nil {
				return "", nil, "", "", err
			}
			i = next
			continue
		}

		if arg == "--profile" || arg == "-p" {
			value, next, err := requiredFlagValue(args, i, arg)
			if err != nil {
				return "", nil, "", "", err
			}
			profileName = value
			i = next
			continue
		}

		if strings.HasPrefix(arg, "--") {
			key, value, hasValue := parseLongParam(arg)
			if key == "" {
				return "", nil, "", "", fmt.Errorf("invalid parameter flag %q", arg)
			}
			if !hasValue && i+1 < len(args) && canConsumeParamValue(args[i+1]) {
				value = args[i+1]
				hasValue = true
				i++
			}
			rawParams[key] = append(rawParams[key], rawParamValue{value: value, hasValue: hasValue})
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			key, value, hasValue := parseShortParam(arg)
			if key == "o" || key == "p" {
				if hasValue && value == "" {
					return "", nil, "", "", fmt.Errorf("flag %s requires a value", arg)
				}
				if !hasValue {
					var next int
					value, next, err = requiredFlagValue(args, i, arg)
					if err != nil {
						return "", nil, "", "", err
					}
					i = next
				}
				if key == "o" {
					outputFormat = value
				} else {
					profileName = value
				}
				continue
			}
			if !hasValue && i+1 < len(args) && canConsumeParamValue(args[i+1]) {
				value = args[i+1]
				hasValue = true
				i++
			}
			rawParams[key] = append(rawParams[key], rawParamValue{value: value, hasValue: hasValue})
			continue
		}

		if toolPath != "" {
			return "", nil, "", "", fmt.Errorf("unexpected positional argument %q", arg)
		}
		toolPath = arg
	}

	if toolPath == "" {
		return "", nil, "", "", fmt.Errorf("tool path is required; expected <server>/<tool>")
	}

	paramTypes := map[string]string(nil)
	if prof != nil && discovery != nil {
		serverName, toolName, parseErr := domain.ParseToolName(toolPath)
		if parseErr != nil {
			return "", nil, "", "", parseErr
		}
		paramTypes = mcpclient.GetParamTypesContext(ctx, prof, serverName, toolName, discovery)
	}

	for key, values := range rawParams {
		for _, raw := range values {
			value, convertErr := convertParamValue(key, raw, paramTypes[key])
			if convertErr != nil {
				return "", nil, "", "", convertErr
			}
			params[key] = appendParamValue(params[key], value, paramTypes[key])
		}
	}

	return toolPath, params, outputFormat, profileName, nil
}

func requiredFlagValue(args []string, index int, flag string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("flag %s requires a value", flag)
	}
	if args[index+1] == "--" || strings.HasPrefix(args[index+1], "--") {
		return "", index, fmt.Errorf("flag %s requires a value", flag)
	}
	return args[index+1], index + 1, nil
}

func inlineFlagValue(arg, flag string) (string, bool) {
	prefix := flag + "="
	if strings.HasPrefix(arg, prefix) {
		return strings.TrimPrefix(arg, prefix), true
	}
	return "", false
}

func parseLongParam(arg string) (key, value string, hasValue bool) {
	arg = strings.TrimPrefix(arg, "--")
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", false
}

func parseShortParam(arg string) (key, value string, hasValue bool) {
	arg = strings.TrimPrefix(arg, "-")
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", false
}

func canConsumeParamValue(value string) bool {
	if value == "--" || strings.HasPrefix(value, "--") {
		return false
	}
	if !strings.HasPrefix(value, "-") {
		return true
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func mergeParams(params map[string]interface{}, source string) error {
	data := []byte(source)
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "{") {
		var err error
		data, err = os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("reading params file: %w", err)
		}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parsing params JSON: %w", err)
	}
	for key, value := range parsed {
		params[key] = value
	}
	return nil
}

func convertParamValue(key string, raw rawParamValue, paramType string) (interface{}, error) {
	if !raw.hasValue {
		if paramType != "" && paramType != "boolean" {
			return nil, fmt.Errorf("parameter --%s requires a value", key)
		}
		return true, nil
	}

	switch paramType {
	case "boolean":
		value, err := strconv.ParseBool(raw.value)
		if err != nil {
			return nil, fmt.Errorf("parameter --%s expects a boolean, got %q", key, raw.value)
		}
		return value, nil
	case "integer":
		if _, err := strconv.ParseInt(raw.value, 10, 64); err != nil {
			return nil, fmt.Errorf("parameter --%s expects an integer, got %q", key, raw.value)
		}
		return json.Number(raw.value), nil
	case "number":
		if _, err := strconv.ParseFloat(raw.value, 64); err != nil {
			return nil, fmt.Errorf("parameter --%s expects a number, got %q", key, raw.value)
		}
		return json.Number(raw.value), nil
	case "array", "object":
		var value interface{}
		if err := json.Unmarshal([]byte(raw.value), &value); err != nil {
			if paramType == "array" {
				return mcpclient.ParseArrayArg(raw.value, nil), nil
			}
			return nil, fmt.Errorf("parameter --%s expects JSON %s, got %q", key, paramType, raw.value)
		}
		if paramType == "array" {
			if array, ok := value.([]interface{}); ok {
				return array, nil
			}
		} else if _, ok := value.(map[string]interface{}); ok {
			return value, nil
		}
		return nil, fmt.Errorf("parameter --%s expects JSON %s, got %q", key, paramType, raw.value)
	default:
		return raw.value, nil
	}
}

func appendParamValue(existing, value interface{}, paramType string) interface{} {
	if paramType != "array" {
		return value
	}
	result, _ := existing.([]interface{})
	if values, ok := value.([]interface{}); ok {
		return append(result, values...)
	}
	return append(result, value)
}

func printMetaOutputs(data interface{}) {
	normalized, err := mcpclient.NormalizeJSONValue(data)
	if err != nil {
		return
	}
	metaValue, ok := objectValue(normalized, "meta")
	if !ok {
		return
	}
	outputsValue, ok := objectValue(metaValue, "outputs")
	if !ok {
		return
	}
	normalizedOutputs, err := mcpclient.NormalizeJSONValue(outputsValue)
	if err != nil {
		return
	}
	outputs, ok := normalizedOutputs.([]interface{})
	if !ok {
		return
	}
	fmt.Fprintln(os.Stderr)
	for _, output := range outputs {
		key, ok := output.(string)
		if !ok {
			continue
		}
		value, ok := objectValue(metaValue, key)
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: key %q specified in outputs not found in meta\n", key)
			continue
		}
		b, err := json.Marshal(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", key, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", key, string(b))
	}
}

func objectValue(data interface{}, key string) (interface{}, bool) {
	switch object := data.(type) {
	case map[string]interface{}:
		value, ok := object[key]
		return value, ok
	case mcpclient.OrderedMap:
		for _, entry := range object {
			if entry.Key == key {
				return entry.Value, true
			}
		}
	}
	return nil, false
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

	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	var out string
	for _, name := range names {
		propRaw := props[name]
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
