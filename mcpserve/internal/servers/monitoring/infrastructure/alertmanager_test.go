package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/servers/monitoring/domain"
)

func alertmanagerServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, domain.AlertRepository) {
	t.Helper()
	s := httptest.NewServer(handler)
	return s, NewAlertmanagerClient(s.URL)
}

func TestGetAlerts_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		alerts := []map[string]interface{}{
			{
				"labels":      map[string]string{"alertname": "CPU", "host": "server-a"},
				"annotations": map[string]string{"summary": "high CPU"},
				"status":      map[string]string{"state": "firing"},
			},
			{
				"labels":      map[string]string{"alertname": "MEM", "host": "server-b"},
				"annotations": map[string]string{},
				"status":      map[string]string{"state": "resolved"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	alerts, err := client.GetAlerts(context.Background())
	if err != nil {
		t.Fatalf("GetAlerts failed: %v", err)
	}

	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}

	if alerts[0].Labels["alertname"] != "CPU" {
		t.Errorf("expected alertname CPU, got %v", alerts[0].Labels["alertname"])
	}
	if alerts[0].Status != domain.AlertStatus("firing") {
		t.Errorf("expected status firing, got %v", alerts[0].Status)
	}
	if alerts[0].Annotations["summary"] != "high CPU" {
		t.Errorf("expected annotation summary, got %v", alerts[0].Annotations["summary"])
	}

	if alerts[1].Status != domain.AlertStatus("resolved") {
		t.Errorf("expected status resolved, got %v", alerts[1].Status)
	}
}

func TestGetAlerts_WithFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/alerts", func(w http.ResponseWriter, r *http.Request) {
		filters := r.URL.Query()["filter"]
		if len(filters) != 1 || filters[0] != `alertname="CPU"` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		alerts := []map[string]interface{}{
			{
				"labels": map[string]string{"alertname": "CPU"},
				"status": map[string]string{"state": "firing"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	alerts, err := client.GetAlerts(context.Background(), `alertname="CPU"`)
	if err != nil {
		t.Fatalf("GetAlerts failed: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestGetAlerts_NonOK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	_, err := client.GetAlerts(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListSilences_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/silences", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		now := time.Now()
		silences := []map[string]interface{}{
			{
				"id": "silence-1",
				"status": map[string]string{
					"state": "active",
				},
				"matchers": []map[string]interface{}{
					{"name": "alertname", "value": "CPU", "isRegex": false, "isEqual": true},
				},
				"startsAt":  now,
				"endsAt":    now.Add(1 * time.Hour),
				"updatedAt": now,
				"createdBy": "admin",
				"comment":   "test silence",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(silences)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	silenceRepo := client.(domain.SilenceRepository)
	silences, err := silenceRepo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(silences) != 1 {
		t.Fatalf("expected 1 silence, got %d", len(silences))
	}

	if silences[0].ID != "silence-1" {
		t.Errorf("expected id silence-1, got %s", silences[0].ID)
	}
	if silences[0].Comment != "test silence" {
		t.Errorf("expected comment 'test silence', got %s", silences[0].Comment)
	}
	if silences[0].CreatedBy != "admin" {
		t.Errorf("expected createdBy admin, got %s", silences[0].CreatedBy)
	}
	if len(silences[0].Matchers) != 1 {
		t.Fatalf("expected 1 matcher, got %d", len(silences[0].Matchers))
	}
	if silences[0].Matchers[0].Name != "alertname" || silences[0].Matchers[0].Value != "CPU" {
		t.Errorf("unexpected matcher: %+v", silences[0].Matchers[0])
	}
}

func TestCreateSilence_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/silences", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"silenceID": "new-silence-123"})
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	silenceRepo := client.(domain.SilenceRepository)
	id, err := silenceRepo.Create(context.Background(), domain.Silence{
		Matchers:  []domain.Matcher{{Name: "alertname", Value: "Test", IsEqual: true}},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		Comment:   "created",
		CreatedBy: "tester",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if id != "new-silence-123" {
		t.Errorf("expected id new-silence-123, got %s", id)
	}
}

func TestCreateSilence_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/silences", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid matcher"}`))
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	silenceRepo := client.(domain.SilenceRepository)
	_, err := silenceRepo.Create(context.Background(), domain.Silence{
		Matchers: []domain.Matcher{{Name: "alertname", Value: "", IsEqual: true}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid matcher") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteSilence_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/silence/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	silenceRepo := client.(domain.SilenceRepository)
	err := silenceRepo.Delete(context.Background(), "silence-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestDeleteSilence_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/silence/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	s, client := alertmanagerServer(t, mux.ServeHTTP)
	defer s.Close()

	silenceRepo := client.(domain.SilenceRepository)
	err := silenceRepo.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
