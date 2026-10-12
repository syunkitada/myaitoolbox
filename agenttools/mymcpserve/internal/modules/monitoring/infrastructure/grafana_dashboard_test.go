package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchDashboards(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		q := r.URL.Query()
		if q.Get("type") != "dash-db" {
			t.Errorf("expected type dash-db, got %q", q.Get("type"))
		}
		if q.Get("query") != "cpu" {
			t.Errorf("expected query cpu, got %q", q.Get("query"))
		}
		if q.Get("folderUIDs") != "folder-1" {
			t.Errorf("expected folderUIDs folder-1, got %q", q.Get("folderUIDs"))
		}
		if got := q["tag"]; len(got) != 2 || got[0] != "prod" || got[1] != "core" {
			t.Errorf("expected tags [prod core], got %#v", got)
		}
		if q.Get("limit") != "10" {
			t.Errorf("expected limit 10, got %q", q.Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"uid":         "abc",
				"title":       "CPU Overview",
				"url":         "/d/abc/cpu-overview",
				"type":        "dash-db",
				"folderUid":   "folder-1",
				"folderTitle": "Prod",
				"tags":        []string{"prod"},
				"isStarred":   true,
			},
			{
				"uid":         "def",
				"title":       "DB Overview",
				"url":         "/d/def/db-overview",
				"type":        "dash-db",
				"folderUid":   "folder-2",
				"folderTitle": "Other",
				"tags":        []string{},
				"isStarred":   false,
			},
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	ctx := context.Background()
	summaries, err := client.SearchDashboards(ctx, "cpu", "folder-1", []string{"prod", "core"}, 10)
	if err != nil {
		t.Fatalf("SearchDashboards failed: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected 1 dashboard after folder filter, got %d", len(summaries))
	}
	if summaries[0].UID != "abc" || summaries[0].Title != "CPU Overview" {
		t.Errorf("unexpected first summary: %#v", summaries[0])
	}
	if summaries[0].FolderUID != "folder-1" {
		t.Errorf("expected folderUid folder-1, got %q", summaries[0].FolderUID)
	}
	if !summaries[0].IsStarred {
		t.Error("expected isStarred true")
	}
}

func TestSearchDashboards_FolderFilter(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"uid": "abc", "title": "A", "type": "dash-db", "folderUid": "folder-1"},
			{"uid": "def", "title": "B", "type": "dash-db", "folderUid": "folder-2"},
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	summaries, err := client.SearchDashboards(context.Background(), "", "folder-1", nil, 50)
	if err != nil {
		t.Fatalf("SearchDashboards failed: %v", err)
	}

	if len(summaries) != 1 || summaries[0].UID != "abc" {
		t.Fatalf("expected only folder-1 dashboard, got %#v", summaries)
	}
}

func TestSearchDashboards_Error(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	_, err := client.SearchDashboards(context.Background(), "", "", nil, 50)
	if err == nil || !strings.Contains(err.Error(), "non-OK status") {
		t.Fatalf("expected non-OK error, got %v", err)
	}
}

func TestGetDashboard(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/dashboards/uid/abc" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"meta": map[string]interface{}{
				"type":        "db",
				"url":         "/d/abc/cpu",
				"version":     3,
				"folderUid":   "folder-1",
				"folderTitle": "Prod",
				"isStarred":   false,
			},
			"dashboard": map[string]interface{}{
				"title": "CPU Overview",
				"uid":   "abc",
			},
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	dashboard, err := client.GetDashboard(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetDashboard failed: %v", err)
	}

	if dashboard.Meta.Version != 3 {
		t.Errorf("expected version 3, got %d", dashboard.Meta.Version)
	}
	if dashboard.Meta.FolderTitle != "Prod" {
		t.Errorf("expected folderTitle Prod, got %q", dashboard.Meta.FolderTitle)
	}
	if dashboard.Model["title"] != "CPU Overview" {
		t.Errorf("expected model title, got %#v", dashboard.Model)
	}
}

func TestGetDashboard_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Dashboard not found"}`))
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	_, err := client.GetDashboard(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "non-OK status") {
		t.Fatalf("expected non-OK error, got %v", err)
	}
}

func TestSaveDashboard(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/dashboards/db" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("expected JSON content type, got %q", r.Header.Get("Content-Type"))
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if _, ok := payload["dashboard"]; !ok {
			t.Error("expected dashboard key in payload")
		}
		if payload["folderUid"] != "folder-1" {
			t.Errorf("expected folderUid folder-1, got %#v", payload["folderUid"])
		}
		if payload["overwrite"] != true {
			t.Errorf("expected overwrite true, got %#v", payload["overwrite"])
		}
		if payload["message"] != "initial save" {
			t.Errorf("expected message, got %#v", payload["message"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      1,
			"uid":     "new-1",
			"url":     "/d/new-1/t",
			"status":  "success",
			"version": 2,
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	result, err := client.SaveDashboard(context.Background(),
		map[string]interface{}{"title": "New Dashboard"},
		"folder-1",
		true,
		"initial save",
	)
	if err != nil {
		t.Fatalf("SaveDashboard failed: %v", err)
	}

	if result.UID != "new-1" || result.Version != 2 || result.Status != "success" {
		t.Errorf("unexpected save result: %#v", result)
	}
}

func TestDeleteDashboard(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/dashboards/uid/abc" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title":   "CPU Overview",
			"message": "Dashboard deleted",
			"uid":     "abc",
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	result, err := client.DeleteDashboard(context.Background(), "abc")
	if err != nil {
		t.Fatalf("DeleteDashboard failed: %v", err)
	}

	if result.UID != "abc" || result.Title != "CPU Overview" {
		t.Errorf("unexpected delete result: %#v", result)
	}
}

func TestListFolders(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/folders" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "uid": "folder-1", "title": "Prod", "url": "/dashboards/f/folder-1/prod"},
		})
	}))
	defer mockServer.Close()

	client := NewGrafanaDashboardClient(mockServer.URL, "test-token")

	folders, err := client.ListFolders(context.Background())
	if err != nil {
		t.Fatalf("ListFolders failed: %v", err)
	}

	if len(folders) != 1 || folders[0].UID != "folder-1" || folders[0].Title != "Prod" {
		t.Errorf("unexpected folders: %#v", folders)
	}
}
