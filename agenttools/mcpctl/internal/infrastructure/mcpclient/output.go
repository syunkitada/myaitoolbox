package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
			fmt.Println(strings.Join(headers, "\t"))
			for _, item := range v {
				if obj, ok := item.(OrderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj.Get(h))
					}
					fmt.Println(strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := OrderedKeys(row)
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
			for _, item := range v {
				fmt.Printf("%v\n", item)
			}
		}
	case OrderedMap:
		for _, e := range v {
			fmt.Printf("%s\t%v\n", e.Key, e.Value)
		}
	case map[string]interface{}:
		for _, k := range OrderedKeys(v) {
			fmt.Printf("%s\t%v\n", k, v[k])
		}
	default:
		fmt.Println(v)
	}
}

func PrintTable(data interface{}) {
	data = ExtractDataArray(data)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return
		}
		if row, ok := v[0].(OrderedMap); ok {
			headers := row.Keys()
			fmt.Fprintln(w, strings.Join(headers, "\t"))
			seps := make([]string, len(headers))
			for i, h := range headers {
				seps[i] = strings.Repeat("-", len(h))
			}
			fmt.Fprintln(w, strings.Join(seps, "\t"))
			for _, item := range v {
				if obj, ok := item.(OrderedMap); ok {
					vals := make([]string, len(headers))
					for i, h := range headers {
						vals[i] = fmt.Sprintf("%v", obj.Get(h))
					}
					fmt.Fprintln(w, strings.Join(vals, "\t"))
				}
			}
		} else if row, ok := v[0].(map[string]interface{}); ok {
			headers := OrderedKeys(row)
			fmt.Fprintln(w, strings.Join(headers, "\t"))
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
			for _, item := range v {
				fmt.Fprintf(w, "%v\n", item)
			}
		}
	case OrderedMap:
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for _, e := range v {
			fmt.Fprintf(w, "%s\t%v\n", e.Key, e.Value)
		}
	case map[string]interface{}:
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
		for _, k := range OrderedKeys(v) {
			fmt.Fprintf(w, "%s\t%v\n", k, v[k])
		}
	default:
		fmt.Fprintln(w, v)
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
	if prof == nil || discovery == nil {
		return nil
	}

	entry, err := discovery.GetToolInfo(context.Background(), prof, serverName, toolName)
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
