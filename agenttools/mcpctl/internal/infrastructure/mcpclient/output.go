package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/syunkitada/myaitoolbox/mcpctl/internal/domain"
)

type OrderedMap []OrderedMapEntry

type OrderedMapEntry struct {
	Key   string
	Value interface{}
}

func (om OrderedMap) Keys() []string {
	ks := make([]string, len(om))
	for i, e := range om {
		ks[i] = e.Key
	}
	return ks
}

func (om OrderedMap) Get(key string) interface{} {
	for _, e := range om {
		if e.Key == key {
			return e.Value
		}
	}
	return nil
}

func DecodeJSON(b []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	value, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected data after JSON value")
		}
		return nil, err
	}
	return value, nil
}

func NormalizeJSONValue(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case json.RawMessage:
		return DecodeJSON(value)
	case []byte:
		return DecodeJSON(value)
	default:
		return data, nil
	}
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
			om := OrderedMap{}
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
				om = append(om, OrderedMapEntry{Key: key, Value: val})
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

func OrderedKeys(obj map[string]interface{}) []string {
	ks := make([]string, 0, len(obj))
	for k := range obj {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func ExtractDataArray(data interface{}) interface{} {
	if normalized, err := NormalizeJSONValue(data); err == nil {
		data = normalized
	}
	if obj, ok := data.(OrderedMap); ok {
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

func PrintTSV(data interface{}) {
	data = ExtractDataArray(data)
	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		if row, ok := v[0].(OrderedMap); ok {
			headers := row.Keys()
			fmt.Println(strings.Join(escapeHeaders(headers), "\t"))
			for _, item := range v {
				if obj, ok := item.(OrderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = formatCell(obj.Get(h))
					}
					fmt.Println(strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := OrderedKeys(row)
			fmt.Println(strings.Join(escapeHeaders(headers), "\t"))
			for _, item := range v {
				if obj, ok := item.(map[string]interface{}); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = formatCell(obj[h])
					}
					fmt.Println(strings.Join(vals, "\t"))
				}
			}
		} else {
			for _, item := range v {
				fmt.Println(formatCell(item))
			}
		}
	case OrderedMap:
		for _, e := range v {
			fmt.Printf("%s\t%s\n", escapeTSV(e.Key), formatCell(e.Value))
		}
	case map[string]interface{}:
		for _, k := range OrderedKeys(v) {
			fmt.Printf("%s\t%s\n", escapeTSV(k), formatCell(v[k]))
		}
	default:
		fmt.Println(formatCell(v))
	}
}

func PrintTable(data interface{}) {
	data = ExtractDataArray(data)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()

	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		if row, ok := v[0].(OrderedMap); ok {
			headers := row.Keys()
			_, _ = fmt.Fprintln(w, strings.Join(escapeHeaders(headers), "\t"))
			seps := make([]string, len(headers))
			for i, h := range headers {
				seps[i] = strings.Repeat("-", len(h))
			}
			_, _ = fmt.Fprintln(w, strings.Join(seps, "\t"))
			for _, item := range v {
				if obj, ok := item.(OrderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = formatCell(obj.Get(h))
					}
					_, _ = fmt.Fprintln(w, strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := OrderedKeys(row)
			_, _ = fmt.Fprintln(w, strings.Join(escapeHeaders(headers), "\t"))
			seps := make([]string, len(headers))
			for i, h := range headers {
				seps[i] = strings.Repeat("-", len(h))
			}
			_, _ = fmt.Fprintln(w, strings.Join(seps, "\t"))
			for _, item := range v {
				if obj, ok := item.(map[string]interface{}); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = formatCell(obj[h])
					}
					_, _ = fmt.Fprintln(w, strings.Join(vals, "\t"))
				}
			}
		} else {
			for _, item := range v {
				_, _ = fmt.Fprintln(w, formatCell(item))
			}
		}
	case OrderedMap:
		_, _ = fmt.Fprintln(w, "KEY\tVALUE")
		_, _ = fmt.Fprintln(w, "---\t-----")
		for _, e := range v {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", escapeTSV(e.Key), formatCell(e.Value))
		}
	case map[string]interface{}:
		_, _ = fmt.Fprintln(w, "KEY\tVALUE")
		_, _ = fmt.Fprintln(w, "---\t-----")
		for _, k := range OrderedKeys(v) {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", escapeTSV(k), formatCell(v[k]))
		}
	default:
		_, _ = fmt.Fprintln(w, formatCell(v))
	}
}

func escapeHeaders(headers []string) []string {
	escaped := make([]string, len(headers))
	for i, header := range headers {
		escaped[i] = escapeTSV(header)
	}
	return escaped
}

func escapeTSV(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\t", "\\t")
	value = strings.ReplaceAll(value, "\r", "\\r")
	return strings.ReplaceAll(value, "\n", "\\n")
}

func formatCell(value interface{}) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return escapeTSV(value)
	case json.Number:
		return value.String()
	case bool, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(value)
	default:
		b, err := json.Marshal(toJSONValue(value))
		if err == nil {
			return escapeTSV(string(b))
		}
		return escapeTSV(fmt.Sprint(value))
	}
}

func toJSONValue(value interface{}) interface{} {
	switch value := value.(type) {
	case OrderedMap:
		obj := make(map[string]interface{}, len(value))
		for _, entry := range value {
			obj[entry.Key] = toJSONValue(entry.Value)
		}
		return obj
	case []interface{}:
		array := make([]interface{}, len(value))
		for i, item := range value {
			array[i] = toJSONValue(item)
		}
		return array
	case map[string]interface{}:
		obj := make(map[string]interface{}, len(value))
		for key, item := range value {
			obj[key] = toJSONValue(item)
		}
		return obj
	default:
		return value
	}
}

func PrintText(text string, format string) {
	parsed, err := DecodeJSON([]byte(text))
	if err != nil {
		fmt.Println(text)
		return
	}

	switch format {
	case "tsv":
		PrintTSV(parsed)
	case "table":
		PrintTable(parsed)
	}
}

func ParseArrayArg(val string, existing interface{}) []interface{} {
	var result []interface{}
	if existing != nil {
		if arr, ok := existing.([]interface{}); ok {
			result = arr
		}
	}

	if strings.HasPrefix(val, "[") {
		var arr []interface{}
		if err := json.Unmarshal([]byte(val), &arr); err == nil {
			result = append(result, arr...)
			return result
		}
	}

	parts := strings.Split(val, ",")
	for _, p := range parts {
		result = append(result, p)
	}
	return result
}

func GetParamTypes(prof *domain.Profile, serverName, toolName string, discovery domain.ToolDiscovery) map[string]string {
	return GetParamTypesContext(context.Background(), prof, serverName, toolName, discovery)
}

func GetParamTypesContext(ctx context.Context, prof *domain.Profile, serverName, toolName string, discovery domain.ToolDiscovery) map[string]string {
	if prof == nil || discovery == nil {
		return nil
	}

	entry, err := discovery.GetToolInfo(ctx, prof, serverName, toolName)
	if err != nil {
		return nil
	}

	schema, ok := entry.Tool.InputSchema.(map[string]interface{})
	if !ok {
		return nil
	}

	props, _ := schema["properties"].(map[string]interface{})
	if len(props) == 0 {
		return nil
	}

	paramTypes := make(map[string]string)
	for name, propRaw := range props {
		prop, _ := propRaw.(map[string]interface{})
		typ, _ := prop["type"].(string)
		if typ != "" {
			paramTypes[name] = typ
		}
	}
	return paramTypes
}
