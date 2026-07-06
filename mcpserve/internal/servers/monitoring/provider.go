package monitoring

import (
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/provider"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/monitoring/domain"
	"github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/monitoring/infrastructure"
)

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

func (p *monitoringProvider) NewServer() provider.Server {
	s := provider.NewMCServer(&mcp.Implementation{Name: "monitoring", Version: "0.0.1"}, nil)

	amURL := os.Getenv("ALERTMANAGER_URL")
	if amURL == "" {
		amURL = "http://127.0.0.1:9093"
	}
	alertRepo := infrastructure.NewAlertmanagerClient(amURL)
	silenceRepo := alertRepo.(domain.SilenceRepository)

	var metricRepo domain.MetricRepository
	if grafanaURL := os.Getenv("GRAFANA_URL"); grafanaURL != "" {
		if apiToken := os.Getenv("GRAFANA_API_TOKEN"); apiToken != "" {
			if dsUID := os.Getenv("GRAFANA_DATASOURCE_UID"); dsUID != "" {
				metricRepo = infrastructure.NewGrafanaClient(grafanaURL, apiToken, dsUID)
			}
		}
	}

	app := NewApp(alertRepo, silenceRepo, metricRepo)

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
	}, wrapTool(app.ListAlerts))

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
	}, wrapTool(app.CreateSilence))

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
	}, wrapTool(app.ListSilences))

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
	}, wrapTool(app.DeleteSilence))

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
				"var": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Key=value pairs, e.g. host=server1, env=prod",
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
	}, wrapTool(app.QueryMetricSummary))

	s.AddTool(&mcp.Tool{
		Name:        "query_metric_history",
		Description: "Query prometheus metrics via Grafana API and return time-aligned data points",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "PromQL query or queries to run",
				},
				"var": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Key=value pairs, e.g. host=server1, env=prod",
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
	}, wrapTool(app.QueryMetricHistory))

	return s
}
