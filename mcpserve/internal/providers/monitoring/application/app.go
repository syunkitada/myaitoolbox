package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

var varParseRegex = regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+)\s*$`)

type App struct {
	alertRepo   domain.AlertRepository
	silenceRepo domain.SilenceRepository
	metricRepo  domain.MetricRepository
}

func NewApp(alertRepo domain.AlertRepository, silenceRepo domain.SilenceRepository, metricRepo domain.MetricRepository) *App {
	return &App{
		alertRepo:   alertRepo,
		silenceRepo: silenceRepo,
		metricRepo:  metricRepo,
	}
}

func (a *App) ListAlerts(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
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

	alerts, err := a.alertRepo.GetAlerts(ctx, amFilters...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get alerts: %w", err)
	}

	var filtered []map[string]interface{}
	for _, al := range alerts {
		if status != "" && string(al.Status) != status {
			continue
		}
		item := map[string]interface{}{
			"status": al.Status,
		}
		if verbose {
			item["labels"] = domain.FormatLabels(al.Labels)
		} else {
			item["labels"] = domain.FormatSelectedLabels(al.Labels, "alertname", "host")
		}
		filtered = append(filtered, item)
	}

	lmeta := map[string]interface{}{
		"status":    status,
		"alertname": alertname,
		"host":      host,
		"count":     len(filtered),
	}
	if len(filtered) == 0 {
		return []interface{}{}, lmeta, nil
	}
	return filtered, lmeta, nil
}

func (a *App) CreateSilence(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
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

	if startat == "now" {
		startTime = now
	} else {
		startTime, err = domain.ParseTime(startat, now)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid startat: %w", err)
		}
	}

	endTime, err := domain.ParseTime(endat, startTime)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid endat: %w", err)
	}

	if !endTime.After(startTime) {
		return nil, nil, fmt.Errorf("endat must be after startat")
	}

	matchers, err := domain.ParseMatchers(matchersStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid matchers: %w", err)
	}

	silence := domain.Silence{
		Matchers:  matchers,
		StartsAt:  startTime,
		EndsAt:    endTime,
		Comment:   comment,
		CreatedBy: createdBy,
	}

	id, err := a.silenceRepo.Create(ctx, silence)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create silence: %w", err)
	}

	slog.Info("silence created",
		slog.String("id", id),
		slog.Any("matchers", matchers),
		slog.Time("start", startTime),
		slog.Time("end", endTime),
		slog.String("by", createdBy),
	)
	cmeta := map[string]interface{}{
		"matchers":   matchers,
		"start":      startTime.Format(time.RFC3339),
		"end":        endTime.Format(time.RFC3339),
		"created_by": createdBy,
	}
	return map[string]interface{}{"id": id}, cmeta, nil
}

func (a *App) ListSilences(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
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

	silences, err := a.silenceRepo.List(ctx, amFilters...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get silences: %w", err)
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
			item["labels"] = domain.FormatLabels(labelsMap)
			item["comment"] = s.Comment
			item["author"] = s.CreatedBy
			item["start"] = s.StartsAt.Format(time.RFC3339)
			item["end"] = s.EndsAt.Format(time.RFC3339)
		} else {
			item["labels"] = domain.FormatSelectedLabels(labelsMap, "alertname", "host")
		}
		filtered = append(filtered, item)
	}

	smeta := map[string]interface{}{
		"alertname": alertname,
		"host":      host,
		"count":     len(filtered),
	}
	if len(filtered) == 0 {
		return []interface{}{}, smeta, nil
	}
	return filtered, smeta, nil
}

func (a *App) DeleteSilence(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}
	id, _ := args["id"].(string)
	if id == "" {
		return nil, nil, fmt.Errorf("id is required")
	}

	if err := a.silenceRepo.Delete(ctx, id); err != nil {
		return nil, nil, fmt.Errorf("failed to delete silence: %w", err)
	}

	slog.Info("silence deleted", slog.String("id", id))
	dmeta := map[string]interface{}{
		"status": "deleted",
		"id":     id,
	}
	return nil, dmeta, nil
}

func (a *App) QueryMetricSummary(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}

	query, _ := args["query"].(string)
	if query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

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

	vars := parseVars(args)

	if a.metricRepo == nil {
		return nil, nil, fmt.Errorf("metric repository not available")
	}

	metrics, mmeta, err := a.metricRepo.QuerySummary(ctx, query, vars, legend, timeFrom, timeTo, sortField, reverse, limit, offset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query metric summary: %w", err)
	}

	return metrics, mmeta, nil
}

func (a *App) QueryMetricHistory(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}

	var queries []string
	if qSlice, ok := args["query"].([]interface{}); ok {
		for _, item := range qSlice {
			if itemStr, ok := item.(string); ok {
				queries = append(queries, itemStr)
			}
		}
	}

	if len(queries) == 0 {
		return nil, nil, fmt.Errorf("query is required")
	}

	legend, _ := args["legend"].(string)
	timeFrom, _ := args["time_from"].(string)
	timeTo, _ := args["time_to"].(string)

	vars := parseVars(args)

	if a.metricRepo == nil {
		return nil, nil, fmt.Errorf("metric repository not available")
	}

	history, mmeta, err := a.metricRepo.QueryHistory(ctx, queries, vars, legend, timeFrom, timeTo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query metric history: %w", err)
	}

	return history, mmeta, nil
}

func parseVars(args map[string]interface{}) map[string]string {
	vars := make(map[string]string)
	if varSlice, ok := args["var"].([]interface{}); ok {
		for _, item := range varSlice {
			itemStr, ok := item.(string)
			if !ok {
				continue
			}
			m := varParseRegex.FindStringSubmatch(itemStr)
			if m != nil {
				vars[m[1]] = strings.TrimSpace(m[2])
			}
		}
	}
	return vars
}
