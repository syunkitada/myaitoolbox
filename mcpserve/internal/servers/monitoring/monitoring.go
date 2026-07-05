package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/provider"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/registry"
)

func init() {
	registry.Register(New())
}

type monitoringProvider struct{}

func New() provider.Provider {
	return &monitoringProvider{}
}

func (p *monitoringProvider) Name() string {
	return "monitoring"
}

func (p *monitoringProvider) Description() string {
	return "Monitoring integration (Alertmanager) for MCP."
}

func (p *monitoringProvider) NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "monitoring", Version: "0.0.1"}, nil)
	client := NewAlertmanagerClient()

	s.AddTool(&mcp.Tool{
		Name:        "list_alerts",
		Description: "Get alerts from Alertmanager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type": "string",
					"enum": []string{"active", "silenced", "inhibited"},
				},
				"alertname": map[string]interface{}{
					"type": "string",
				},
				"host": map[string]interface{}{
					"type": "string",
				},
				"verbose": map[string]interface{}{
					"type": "boolean",
				},
			},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}
		status, _ := args["status"].(string)
		alertname, _ := args["alertname"].(string)
		host, _ := args["host"].(string)
		verbose, _ := args["verbose"].(bool)

		var amFilters []string
		if alertname != "" {
			amFilters = append(amFilters, fmt.Sprintf(`alertname="%s"`, alertname))
		}
		if host != "" {
			amFilters = append(amFilters, fmt.Sprintf(`host="%s"`, host))
		}

		alerts, err := client.GetAlerts(amFilters...)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to get alerts: %v", err)}}}, nil
		}

		var filtered []map[string]interface{}
		for _, a := range alerts {
			if status != "" && a.Status.State != status {
				continue
			}

			item := map[string]interface{}{
				"status": a.Status.State,
			}
			if verbose {
				item["labels"] = FormatLabels(a.Labels)
			} else {
				item["labels"] = FormatSelectedLabels(a.Labels, "alertname", "host")
			}
			filtered = append(filtered, item)
		}

		b, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to encode result: %v", err)}}}, nil
		}
		if len(filtered) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "[]"}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
	}))

	s.AddTool(&mcp.Tool{
		Name:        "create_silence",
		Description: "Create a new silence in Alertmanager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"startat": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative time (e.g. +1h). Default is now.",
				},
				"endat": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative time (e.g. +2h)",
				},
				"matchers": map[string]interface{}{
					"type":        "string",
					"description": "Comma separated key=value pairs, e.g. alertname=\"HighCPUUsage\",host=\"server1\"",
				},
				"comment": map[string]interface{}{
					"type": "string",
				},
				"created_by": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"endat", "matchers", "comment", "created_by"},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}
		startat, ok := args["startat"].(string)
		if !ok {
			startat = "now"
		}
		endat, _ := args["endat"].(string)
		matchersStr, _ := args["matchers"].(string)
		comment, _ := args["comment"].(string)
		createdBy, _ := args["created_by"].(string)

		now := time.Now()
		var startTime time.Time
		var err error

		if startat == "now" {
			startTime = now
		} else {
			startTime, err = ParseTime(startat, now)
			if err != nil {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Invalid startat: %v", err)}}}, nil
			}
		}

		endTime, err := ParseTime(endat, startTime)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Invalid endat: %v", err)}}}, nil
		}

		if !endTime.After(startTime) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "endat must be after startat"}}}, nil
		}

		matchers, err := ParseMatchers(matchersStr)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Invalid matchers: %v", err)}}}, nil
		}

		silence := Silence{
			Matchers:  matchers,
			StartsAt:  startTime,
			EndsAt:    endTime,
			Comment:   comment,
			CreatedBy: createdBy,
		}

		id, err := client.CreateSilence(silence)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to create silence: %v", err)}}}, nil
		}

		slog.Info("silence created",
			slog.String("id", id),
			slog.Any("matchers", matchers),
			slog.Time("start", startTime),
			slog.Time("end", endTime),
			slog.String("by", createdBy),
		)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Created silence with ID: %s", id)}}}, nil
	}))

	s.AddTool(&mcp.Tool{
		Name:        "list_silences",
		Description: "Get silences from Alertmanager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"alertname": map[string]interface{}{
					"type": "string",
				},
				"host": map[string]interface{}{
					"type": "string",
				},
				"verbose": map[string]interface{}{
					"type": "boolean",
				},
			},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}
		alertname, _ := args["alertname"].(string)
		host, _ := args["host"].(string)
		verbose, _ := args["verbose"].(bool)

		var amFilters []string
		if alertname != "" {
			amFilters = append(amFilters, fmt.Sprintf(`alertname="%s"`, alertname))
		}
		if host != "" {
			amFilters = append(amFilters, fmt.Sprintf(`host="%s"`, host))
		}

		silences, err := client.GetSilences(amFilters...)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to get silences: %v", err)}}}, nil
		}

		var filtered []map[string]interface{}
		for _, s := range silences {
			labelsMap := make(map[string]string)
			for _, m := range s.Matchers {
				labelsMap[m.Name] = m.Value
			}

			item := map[string]interface{}{
				"id": s.ID,
			}
			if verbose {
				item["labels"] = FormatLabels(labelsMap)
				item["comment"] = s.Comment
				item["author"] = s.CreatedBy
				item["start"] = s.StartsAt.Format(time.RFC3339)
				item["end"] = s.EndsAt.Format(time.RFC3339)
			} else {
				item["labels"] = FormatSelectedLabels(labelsMap, "alertname", "host")
			}
			filtered = append(filtered, item)
		}

		b, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to encode result: %v", err)}}}, nil
		}
		if len(filtered) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "[]"}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
	}))

	s.AddTool(&mcp.Tool{
		Name:        "delete_silence",
		Description: "Delete a silence in Alertmanager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"id"},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}
		id, _ := args["id"].(string)
		if id == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "id is required"}}}, nil
		}

		err := client.DeleteSilence(id)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to delete silence: %v", err)}}}, nil
		}

		slog.Info("silence deleted", slog.String("id", id))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Deleted silence %s", id)}}}, nil
	}))

	s.AddTool(&mcp.Tool{
		Name:        "query_metric_summary",
		Description: "Query prometheus metrics via Grafana API and return summary statistics",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "PromQL query to run",
				},
				"vars": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated key=value pairs, e.g. host=\"server1\",env=\"prod\"",
				},
				"legend": map[string]interface{}{
					"type":        "string",
					"description": "Legend template with labels in curly braces, e.g. {{host}}",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Field to sort by (legend, samples, min, p50, p90, p99, max, last). Default is p99.",
				},
				"reverse": map[string]interface{}{
					"type":        "boolean",
					"description": "Reverse sort order",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Max number of items to return. Default is 100.",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Number of items to skip",
				},
				"time_from": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative duration (e.g. now-1h, 1h, 1d). Default is now-1h.",
				},
				"time_to": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative duration (e.g. now, 5m). Default is now.",
				},
			},
			"required": []string{"query"},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}

		query, _ := args["query"].(string)
		if query == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "query is required"}}}, nil
		}

		varsStr, _ := args["vars"].(string)
		legend, _ := args["legend"].(string)
		sortField, _ := args["sort"].(string)
		reverse, _ := args["reverse"].(bool)

		limit := 100
		if val, ok := args["limit"].(float64); ok {
			limit = int(val)
		}

		offset := 0
		if val, ok := args["offset"].(float64); ok {
			offset = int(val)
		}

		timeFrom, _ := args["time_from"].(string)
		timeTo, _ := args["time_to"].(string)

		vars := make(map[string]string)
		if varsStr != "" {
			re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(?:"([^"]*)"|([^,\s]+))`)
			matches := re.FindAllStringSubmatch(varsStr, -1)
			for _, m := range matches {
				k := m[1]
				v := m[2]
				if v == "" {
					v = m[3]
				}
				vars[k] = v
			}
		}

		grafanaURL := os.Getenv("GRAFANA_URL")
		grafanaAPIToken := os.Getenv("GRAFANA_API_TOKEN")
		grafanaDatasourceUID := os.Getenv("GRAFANA_DATASOURCE_UID")

		if grafanaURL == "" || grafanaAPIToken == "" || grafanaDatasourceUID == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "GRAFANA_URL, GRAFANA_API_TOKEN, and GRAFANA_DATASOURCE_UID must be set in environment variables"}},
			}, nil
		}

		client := NewGrafanaClient(grafanaURL, grafanaAPIToken, grafanaDatasourceUID)
		res, err := client.QueryMetricSummary(ctx, query, vars, legend, timeFrom, timeTo, sortField, reverse, limit, offset)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to query metric summary: %v", err)}},
			}, nil
		}

		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to encode result: %v", err)}},
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil
	}))

	s.AddTool(&mcp.Tool{
		Name:        "query_metric_history",
		Description: "Query prometheus metrics via Grafana API and return time-aligned data points",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"oneOf": []interface{}{
						map[string]interface{}{"type": "string"},
						map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"description": "PromQL query or queries to run",
				},
				"vars": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated key=value pairs, e.g. host=\"server1\",env=\"prod\"",
				},
				"legend": map[string]interface{}{
					"type":        "string",
					"description": "Legend template with labels in curly braces, e.g. {{host}}",
				},
				"time_from": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative duration (e.g. now-1h, 1h, 1d). Default is now-1h.",
				},
				"time_to": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 or relative duration (e.g. now, 5m). Default is now.",
				},
			},
			"required": []string{"query"},
		},
	}, wrapTool(func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Invalid arguments format"}}}, nil
		}

		var queries []string
		if qRaw, ok := args["query"]; ok {
			if qStr, ok := qRaw.(string); ok {
				queries = []string{qStr}
			} else if qSlice, ok := qRaw.([]interface{}); ok {
				for _, item := range qSlice {
					if itemStr, ok := item.(string); ok {
						queries = append(queries, itemStr)
					}
				}
			}
		}

		if len(queries) == 0 {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "query is required"}}}, nil
		}

		varsStr, _ := args["vars"].(string)
		legend, _ := args["legend"].(string)
		timeFrom, _ := args["time_from"].(string)
		timeTo, _ := args["time_to"].(string)

		vars := make(map[string]string)
		if varsStr != "" {
			re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(?:"([^"]*)"|([^,\s]+))`)
			matches := re.FindAllStringSubmatch(varsStr, -1)
			for _, m := range matches {
				k := m[1]
				v := m[2]
				if v == "" {
					v = m[3]
				}
				vars[k] = v
			}
		}

		grafanaURL := os.Getenv("GRAFANA_URL")
		grafanaAPIToken := os.Getenv("GRAFANA_API_TOKEN")
		grafanaDatasourceUID := os.Getenv("GRAFANA_DATASOURCE_UID")

		if grafanaURL == "" || grafanaAPIToken == "" || grafanaDatasourceUID == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "GRAFANA_URL, GRAFANA_API_TOKEN, and GRAFANA_DATASOURCE_UID must be set in environment variables"}},
			}, nil
		}

		client := NewGrafanaClient(grafanaURL, grafanaAPIToken, grafanaDatasourceUID)
		res, err := client.QueryMetricHistory(ctx, queries, vars, legend, timeFrom, timeTo)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to query metric history: %v", err)}},
			}, nil
		}

		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed to encode result: %v", err)}},
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil
	}))

	return s
}

// wrapTool is a helper wrapper to log details of MCP tool execution using slog.
func wrapTool(handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var toolName string
		var args any
		if request != nil && request.Params != nil {
			toolName = request.Params.Name
			if len(request.Params.Arguments) > 0 {
				_ = json.Unmarshal(request.Params.Arguments, &args)
			}
		}

		slog.Info("MCP tool called",
			slog.String("tool", toolName),
			slog.Any("parameters", args),
		)

		res, err := handler(ctx, request)
		if err != nil {
			slog.Error("MCP tool error",
				slog.String("tool", toolName),
				slog.Any("error", err),
			)
		} else if res != nil && res.IsError {
			slog.Warn("MCP tool returned execution error",
				slog.String("tool", toolName),
				slog.Any("result", res),
			)
		} else {
			slog.Info("MCP tool execution completed",
				slog.String("tool", toolName),
			)
		}
		return res, err
	}
}
