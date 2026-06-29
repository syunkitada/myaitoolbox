package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	})

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
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		log.Printf("silence created: id=%s matchers=%v start=%s end=%s by=%s", id, matchers, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), createdBy)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Created silence with ID: %s", id)}}}, nil
	})

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
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	})

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
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		log.Printf("silence deleted: id=%s", id)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Deleted silence %s", id)}}}, nil
	})

	return s
}
