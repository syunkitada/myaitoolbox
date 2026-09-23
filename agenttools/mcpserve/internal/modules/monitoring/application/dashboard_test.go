package application

import (
	"context"
	"testing"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/domain"
)

type mockDashboardRepo struct {
	summaries       []domain.DashboardSummary
	dashboard       domain.Dashboard
	saveResult      domain.DashboardSaveResult
	deleteResult    domain.DashboardDeleteResult
	folders         []domain.Folder
	err             error
	searchQuery     string
	searchFolderUID string
	searchTags      []string
	searchLimit     int
	savedModel      map[string]interface{}
	savedFolderUID  string
	savedOverwrite  bool
	savedMessage    string
	deletedUID      string
}

func (m *mockDashboardRepo) SearchDashboards(ctx context.Context, query, folderUID string, tags []string, limit int) ([]domain.DashboardSummary, error) {
	m.searchQuery = query
	m.searchFolderUID = folderUID
	m.searchTags = tags
	m.searchLimit = limit
	return m.summaries, m.err
}

func (m *mockDashboardRepo) GetDashboard(ctx context.Context, uid string) (domain.Dashboard, error) {
	m.deletedUID = uid
	return m.dashboard, m.err
}

func (m *mockDashboardRepo) SaveDashboard(ctx context.Context, model map[string]interface{}, folderUID string, overwrite bool, message string) (domain.DashboardSaveResult, error) {
	m.savedModel = model
	m.savedFolderUID = folderUID
	m.savedOverwrite = overwrite
	m.savedMessage = message
	return m.saveResult, m.err
}

func (m *mockDashboardRepo) DeleteDashboard(ctx context.Context, uid string) (domain.DashboardDeleteResult, error) {
	m.deletedUID = uid
	return m.deleteResult, m.err
}

func (m *mockDashboardRepo) ListFolders(ctx context.Context) ([]domain.Folder, error) {
	return m.folders, m.err
}

func TestListDashboards_Success(t *testing.T) {
	repo := &mockDashboardRepo{
		summaries: []domain.DashboardSummary{
			{UID: "abc", Title: "CPU Overview", FolderUID: "folder-1"},
		},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, meta, err := app.ListDashboards(context.Background(), callToolReq(t, map[string]interface{}{
		"query":      "cpu",
		"folder_uid": "folder-1",
		"tag":        []interface{}{"prod", "core"},
		"limit":      float64(10),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.searchQuery != "cpu" {
		t.Errorf("expected query cpu, got %q", repo.searchQuery)
	}
	if repo.searchFolderUID != "folder-1" {
		t.Errorf("expected folder_uid folder-1, got %q", repo.searchFolderUID)
	}
	if len(repo.searchTags) != 2 || repo.searchTags[0] != "prod" {
		t.Errorf("expected tags [prod core], got %#v", repo.searchTags)
	}
	if repo.searchLimit != 10 {
		t.Errorf("expected limit 10, got %d", repo.searchLimit)
	}

	items, ok := data.([]domain.DashboardSummary)
	if !ok {
		t.Fatalf("expected []domain.DashboardSummary, got %T", data)
	}
	if len(items) != 1 || items[0].UID != "abc" {
		t.Errorf("unexpected items: %#v", items)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["count"] != 1 {
		t.Errorf("expected count 1, got %v", metaMap["count"])
	}
}

func TestListDashboards_Empty(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{})

	data, meta, err := app.ListDashboards(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["count"] != 0 {
		t.Errorf("expected count 0, got %v", metaMap["count"])
	}

	items, ok := data.([]interface{})
	if !ok || len(items) != 0 {
		t.Errorf("expected empty slice, got %T %v", data, data)
	}
}

func TestListDashboards_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{err: assertError("search failed")})

	_, _, err := app.ListDashboards(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil || err.Error() != "failed to search dashboards: search failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListDashboards_NilRepo(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, nil)

	_, _, err := app.ListDashboards(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil || err.Error() != "dashboard repository not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetDashboard_Compact(t *testing.T) {
	repo := &mockDashboardRepo{
		dashboard: domain.Dashboard{
			Meta: domain.DashboardMeta{
				URL:         "/d/abc/cpu",
				Version:     3,
				FolderUID:   "folder-1",
				FolderTitle: "Prod",
				Created:     "2026-01-01T00:00:00Z",
			},
			Model: map[string]interface{}{
				"title": "CPU Overview",
				"tags":  []interface{}{"prod"},
				"panels": []interface{}{
					map[string]interface{}{"type": "timeseries"},
					map[string]interface{}{"type": "stat"},
				},
			},
		},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, meta, err := app.GetDashboard(context.Background(), callToolReq(t, map[string]interface{}{"uid": "abc"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	compact, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", data)
	}
	if compact["uid"] != "abc" {
		t.Errorf("expected uid abc, got %v", compact["uid"])
	}
	if compact["title"] != "CPU Overview" {
		t.Errorf("expected title, got %v", compact["title"])
	}
	if compact["panel_count"] != 2 {
		t.Errorf("expected panel_count 2, got %v", compact["panel_count"])
	}
	if compact["version"] != 3 {
		t.Errorf("expected version 3, got %v", compact["version"])
	}
	if compact["folder_title"] != "Prod" {
		t.Errorf("expected folder_title Prod, got %v", compact["folder_title"])
	}
	if _, hasModel := compact["title"]; !hasModel {
		t.Errorf("expected title key in compact data")
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["verbose"] != false {
		t.Errorf("expected verbose false, got %v", metaMap["verbose"])
	}
}

func TestGetDashboard_Verbose(t *testing.T) {
	repo := &mockDashboardRepo{
		dashboard: domain.Dashboard{
			Meta: domain.DashboardMeta{URL: "/d/abc/cpu"},
			Model: map[string]interface{}{
				"title": "CPU Overview",
			},
		},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, _, err := app.GetDashboard(context.Background(), callToolReq(t, map[string]interface{}{
		"uid":     "abc",
		"verbose": true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dashboard, ok := data.(domain.Dashboard)
	if !ok {
		t.Fatalf("expected domain.Dashboard, got %T", data)
	}
	if dashboard.Meta.URL != "/d/abc/cpu" {
		t.Errorf("expected meta url, got %v", dashboard.Meta.URL)
	}
	if dashboard.Model["title"] != "CPU Overview" {
		t.Errorf("expected model title, got %v", dashboard.Model["title"])
	}
}

func TestGetDashboard_MissingUID(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{})

	_, _, err := app.GetDashboard(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetDashboard_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{err: assertError("not found")})

	_, _, err := app.GetDashboard(context.Background(), callToolReq(t, map[string]interface{}{"uid": "abc"}))
	if err == nil || err.Error() != "failed to get dashboard: not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDashboard_FromTitle(t *testing.T) {
	repo := &mockDashboardRepo{
		saveResult: domain.DashboardSaveResult{UID: "created-1", URL: "/d/created-1/t", Status: "success", Version: 1},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, meta, err := app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{
		"title":      "New Dashboard",
		"folder_uid": "folder-1",
		"tags":       []interface{}{"prod"},
		"overwrite":  true,
		"message":    "initial save",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.savedModel["title"] != "New Dashboard" {
		t.Errorf("expected model title, got %#v", repo.savedModel)
	}
	if len(repo.savedModel["tags"].([]string)) != 1 {
		t.Errorf("expected model tags, got %#v", repo.savedModel["tags"])
	}
	if repo.savedFolderUID != "folder-1" {
		t.Errorf("expected folder_uid folder-1, got %q", repo.savedFolderUID)
	}
	if !repo.savedOverwrite {
		t.Error("expected overwrite true")
	}
	if repo.savedMessage != "initial save" {
		t.Errorf("expected message, got %q", repo.savedMessage)
	}

	result := data.(domain.DashboardSaveResult)
	if result.UID != "created-1" {
		t.Errorf("expected uid created-1, got %v", result.UID)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["folder_uid"] != "folder-1" {
		t.Errorf("expected folder_uid in meta, got %v", metaMap["folder_uid"])
	}
}

func TestCreateDashboard_WithModel(t *testing.T) {
	repo := &mockDashboardRepo{
		saveResult: domain.DashboardSaveResult{UID: "created-1", Status: "success"},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	dashboardModel := map[string]interface{}{
		"title":         "Model Dashboard",
		"schemaVersion": float64(39),
	}
	app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{
		"dashboard": dashboardModel,
	}))

	if repo.savedModel["title"] != "Model Dashboard" {
		t.Errorf("expected model title, got %#v", repo.savedModel)
	}
	if repo.savedModel["schemaVersion"] == nil {
		t.Errorf("expected schemaVersion preserved, got %#v", repo.savedModel)
	}
}

func TestCreateDashboard_TitleOverridesModel(t *testing.T) {
	repo := &mockDashboardRepo{
		saveResult: domain.DashboardSaveResult{UID: "created-1", Status: "success"},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{
		"title": "Override Title",
		"dashboard": map[string]interface{}{
			"title": "Original Title",
		},
	}))

	if repo.savedModel["title"] != "Override Title" {
		t.Errorf("expected title override, got %#v", repo.savedModel["title"])
	}
}

func TestCreateDashboard_MissingTitle(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{})

	_, _, err := app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateDashboard_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{err: assertError("save failed")})

	_, _, err := app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{"title": "T"}))
	if err == nil || err.Error() != "failed to save dashboard: save failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDashboard_NilRepo(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, nil)

	_, _, err := app.CreateDashboard(context.Background(), callToolReq(t, map[string]interface{}{"title": "T"}))
	if err == nil || err.Error() != "dashboard repository not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteDashboard_Success(t *testing.T) {
	repo := &mockDashboardRepo{
		deleteResult: domain.DashboardDeleteResult{Title: "CPU Overview", Message: "Dashboard deleted", UID: "abc"},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, meta, err := app.DeleteDashboard(context.Background(), callToolReq(t, map[string]interface{}{"uid": "abc"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.deletedUID != "abc" {
		t.Errorf("expected deleted uid abc, got %q", repo.deletedUID)
	}

	result := data.(domain.DashboardDeleteResult)
	if result.UID != "abc" {
		t.Errorf("expected result uid abc, got %v", result.UID)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["status"] != "deleted" {
		t.Errorf("expected status deleted, got %v", metaMap["status"])
	}
	if metaMap["uid"] != "abc" {
		t.Errorf("expected meta uid abc, got %v", metaMap["uid"])
	}
}

func TestDeleteDashboard_MissingUID(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{})

	_, _, err := app.DeleteDashboard(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteDashboard_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{err: assertError("delete failed")})

	_, _, err := app.DeleteDashboard(context.Background(), callToolReq(t, map[string]interface{}{"uid": "abc"}))
	if err == nil || err.Error() != "failed to delete dashboard: delete failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListFolders_Success(t *testing.T) {
	repo := &mockDashboardRepo{
		folders: []domain.Folder{
			{UID: "folder-1", Title: "Prod"},
		},
	}
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, repo)

	data, meta, err := app.ListFolders(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	folders, ok := data.([]domain.Folder)
	if !ok {
		t.Fatalf("expected []domain.Folder, got %T", data)
	}
	if len(folders) != 1 || folders[0].UID != "folder-1" {
		t.Errorf("unexpected folders: %#v", folders)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["count"] != 1 {
		t.Errorf("expected count 1, got %v", metaMap["count"])
	}
}

func TestListFolders_Empty(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{})

	data, meta, err := app.ListFolders(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["count"] != 0 {
		t.Errorf("expected count 0, got %v", metaMap["count"])
	}

	items, ok := data.([]interface{})
	if !ok || len(items) != 0 {
		t.Errorf("expected empty slice, got %T %v", data, data)
	}
}

func TestListFolders_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil, &mockDashboardRepo{err: assertError("folders failed")})

	_, _, err := app.ListFolders(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil || err.Error() != "failed to list folders: folders failed" {
		t.Errorf("unexpected error: %v", err)
	}
}
