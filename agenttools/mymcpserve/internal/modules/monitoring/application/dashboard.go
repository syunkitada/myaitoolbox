package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultDashboardLimit = 50

func (a *App) ListDashboards(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}
	query, _ := args["query"].(string)
	folderUID, _ := args["folder_uid"].(string)
	limit := defaultDashboardLimit
	if val, ok := args["limit"].(float64); ok {
		limit = int(val)
	}
	var tags []string
	if tagSlice, ok := args["tag"].([]interface{}); ok {
		for _, item := range tagSlice {
			if s, ok := item.(string); ok && s != "" {
				tags = append(tags, s)
			}
		}
	}

	if a.dashboardRepo == nil {
		return nil, nil, fmt.Errorf("dashboard repository not available")
	}

	dashboards, err := a.dashboardRepo.SearchDashboards(ctx, query, folderUID, tags, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search dashboards: %w", err)
	}

	lmeta := map[string]interface{}{
		"query":      query,
		"folder_uid": folderUID,
		"tags":       tags,
		"count":      len(dashboards),
	}
	if len(dashboards) == 0 {
		return []interface{}{}, lmeta, nil
	}
	return dashboards, lmeta, nil
}

func (a *App) GetDashboard(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}
	uid, _ := args["uid"].(string)
	if uid == "" {
		return nil, nil, fmt.Errorf("uid is required")
	}
	verbose, _ := args["verbose"].(bool)

	if a.dashboardRepo == nil {
		return nil, nil, fmt.Errorf("dashboard repository not available")
	}

	dashboard, err := a.dashboardRepo.GetDashboard(ctx, uid)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get dashboard: %w", err)
	}

	gmeta := map[string]interface{}{
		"uid":     uid,
		"verbose": verbose,
	}
	if verbose {
		return dashboard, gmeta, nil
	}

	title, _ := dashboard.Model["title"].(string)
	var panelCount int
	if panels, ok := dashboard.Model["panels"].([]interface{}); ok {
		panelCount = len(panels)
	}
	var tags []string
	if rawTags, ok := dashboard.Model["tags"].([]interface{}); ok {
		for _, item := range rawTags {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}

	compact := map[string]interface{}{
		"uid":          uid,
		"title":        title,
		"tags":         tags,
		"panel_count":  panelCount,
		"url":          dashboard.Meta.URL,
		"version":      dashboard.Meta.Version,
		"folder_uid":   dashboard.Meta.FolderUID,
		"folder_title": dashboard.Meta.FolderTitle,
		"created":      dashboard.Meta.Created,
		"updated":      dashboard.Meta.Updated,
		"created_by":   dashboard.Meta.CreatedBy,
		"updated_by":   dashboard.Meta.UpdatedBy,
		"is_starred":   dashboard.Meta.IsStarred,
	}
	return compact, gmeta, nil
}

func (a *App) CreateDashboard(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}
	title, _ := args["title"].(string)
	folderUID, _ := args["folder_uid"].(string)
	message, _ := args["message"].(string)
	overwrite, _ := args["overwrite"].(bool)
	var tags []string
	if tagSlice, ok := args["tags"].([]interface{}); ok {
		for _, item := range tagSlice {
			if s, ok := item.(string); ok && s != "" {
				tags = append(tags, s)
			}
		}
	}

	var model map[string]interface{}
	if raw, ok := args["dashboard"].(map[string]interface{}); ok {
		model = raw
	} else {
		model = map[string]interface{}{}
	}
	if title != "" {
		model["title"] = title
	} else if t, ok := model["title"].(string); !ok || t == "" {
		return nil, nil, fmt.Errorf("title is required")
	}
	if len(tags) > 0 {
		model["tags"] = tags
	}

	if a.dashboardRepo == nil {
		return nil, nil, fmt.Errorf("dashboard repository not available")
	}

	result, err := a.dashboardRepo.SaveDashboard(ctx, model, folderUID, overwrite, message)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save dashboard: %w", err)
	}

	slog.Info("dashboard saved",
		slog.String("uid", result.UID),
		slog.String("url", result.URL),
		slog.Int("version", result.Version),
	)
	cmeta := map[string]interface{}{
		"folder_uid": folderUID,
		"overwrite":  overwrite,
	}
	return result, cmeta, nil
}

func (a *App) DeleteDashboard(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}
	uid, _ := args["uid"].(string)
	if uid == "" {
		return nil, nil, fmt.Errorf("uid is required")
	}

	if a.dashboardRepo == nil {
		return nil, nil, fmt.Errorf("dashboard repository not available")
	}

	result, err := a.dashboardRepo.DeleteDashboard(ctx, uid)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete dashboard: %w", err)
	}

	slog.Info("dashboard deleted",
		slog.String("uid", uid),
		slog.String("title", result.Title),
	)
	dmeta := map[string]interface{}{
		"status": "deleted",
		"uid":    uid,
	}
	return result, dmeta, nil
}

func (a *App) ListFolders(ctx context.Context, req *mcp.CallToolRequest) (data, meta interface{}, err error) {
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, nil, fmt.Errorf("invalid arguments format")
	}

	if a.dashboardRepo == nil {
		return nil, nil, fmt.Errorf("dashboard repository not available")
	}

	folders, err := a.dashboardRepo.ListFolders(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list folders: %w", err)
	}

	fmeta := map[string]interface{}{
		"count": len(folders),
	}
	if len(folders) == 0 {
		return []interface{}{}, fmeta, nil
	}
	return folders, fmeta, nil
}
