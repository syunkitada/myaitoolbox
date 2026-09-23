package monitoring

import (
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/domain"

	monApp "github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/application"
	monDomain "github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/domain"
	monInfra "github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/infrastructure"
)

type monitoringProvider struct{}

func New() domain.Provider {
	return &monitoringProvider{}
}

func (p *monitoringProvider) Name() string {
	return "monitoring"
}

func (p *monitoringProvider) Description() string {
	return "Monitoring integration (Alertmanager, Grafana) for MCP."
}

func (p *monitoringProvider) RegisterTools(server domain.Server) {
	amURL := os.Getenv("ALERTMANAGER_URL")
	if amURL == "" {
		amURL = "http://127.0.0.1:9093"
	}
	alertRepo := monInfra.NewAlertmanagerClient(amURL)
	silenceRepo := alertRepo.(monDomain.SilenceRepository)

	var dashboardRepo monDomain.DashboardRepository
	var metricRepo monDomain.MetricRepository
	if grafanaURL := os.Getenv("GRAFANA_URL"); grafanaURL != "" {
		if apiToken := os.Getenv("GRAFANA_API_TOKEN"); apiToken != "" {
			dashboardRepo = monInfra.NewGrafanaDashboardClient(grafanaURL, apiToken)
			if dsUID := os.Getenv("GRAFANA_DATASOURCE_UID"); dsUID != "" {
				metricRepo = monInfra.NewGrafanaClient(grafanaURL, apiToken, dsUID)
			}
		}
	}

	app := monApp.NewApp(alertRepo, silenceRepo, metricRepo, dashboardRepo)

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.ListAlerts))

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.CreateSilence))

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.ListSilences))

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.DeleteSilence))

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.QueryMetricSummary))

	server.AddTool(&mcp.Tool{
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
	}, monApp.WrapTool(app.QueryMetricHistory))

	server.AddTool(&mcp.Tool{
		Name:        "list_dashboards",
		Description: "Search Grafana dashboards",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search text matched against dashboard title",
				},
				"folder_uid": map[string]interface{}{
					"type":        "string",
					"description": "Restrict results to a folder UID",
				},
				"tag": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Dashboard tags to filter by",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Max number of dashboards to return. Default is 50.",
				},
			},
		},
	}, monApp.WrapTool(app.ListDashboards))

	server.AddTool(&mcp.Tool{
		Name:        "get_dashboard",
		Description: "Get a Grafana dashboard by UID",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uid": map[string]interface{}{
					"type": "string",
				},
				"verbose": map[string]interface{}{
					"type":        "boolean",
					"description": "Include the full dashboard model in the response",
				},
			},
			"required": []string{"uid"},
		},
	}, monApp.WrapTool(app.GetDashboard))

	server.AddTool(&mcp.Tool{
		Name:        "create_dashboard",
		Description: "Create or update a Grafana dashboard",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Dashboard title. Overrides dashboard.title when both are provided.",
				},
				"dashboard": map[string]interface{}{
					"type":        "object",
					"description": "Full dashboard model JSON. When omitted, a minimal dashboard is created from title and tags.",
				},
				"folder_uid": map[string]interface{}{
					"type":        "string",
					"description": "Destination folder UID. Omit for the General folder.",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Dashboard tags to set",
				},
				"overwrite": map[string]interface{}{
					"type":        "boolean",
					"description": "Overwrite an existing dashboard with the same UID. Default is false.",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Change message stored with the new version",
				},
			},
		},
	}, monApp.WrapTool(app.CreateDashboard))

	server.AddTool(&mcp.Tool{
		Name:        "delete_dashboard",
		Description: "Delete a Grafana dashboard by UID",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uid": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"uid"},
		},
	}, monApp.WrapTool(app.DeleteDashboard))

	server.AddTool(&mcp.Tool{
		Name:        "list_folders",
		Description: "List Grafana dashboard folders",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, monApp.WrapTool(app.ListFolders))
}
