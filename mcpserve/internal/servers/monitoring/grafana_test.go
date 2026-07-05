package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseFlexibleTime(t *testing.T) {
	baseTime := time.Date(2026, 7, 5, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timeStr  string
		isFrom   bool
		expected time.Time
		wantErr  bool
	}{
		{"empty from", "", true, baseTime.Add(-1 * time.Hour), false},
		{"empty to", "", false, baseTime, false},
		{"now", "now", false, baseTime, false},
		{"now-1h", "now-1h", false, baseTime.Add(-1 * time.Hour), false},
		{"5m", "5m", false, baseTime.Add(-5 * time.Minute), false},
		{"1d", "1d", false, baseTime.Add(-24 * time.Hour), false},
		{"-1h", "-1h", false, baseTime.Add(-1 * time.Hour), false},
		{"RFC3339", "2026-07-05T17:00:00Z", false, time.Date(2026, 7, 5, 17, 0, 0, 0, time.UTC), false},
		{"invalid", "invalid", false, time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlexibleTime(tt.timeStr, baseTime, tt.isFrom)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFlexibleTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.expected) {
				t.Errorf("parseFlexibleTime() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExpandVariables(t *testing.T) {
	vars := map[string]string{
		"host": "server-a",
		"env":  "production",
	}

	tests := []struct {
		query    string
		expected string
	}{
		{"cpu_usage{host=\"$host\"}", "cpu_usage{host=\"server-a\"}"},
		{"cpu_usage{host=\"${host}\", env=\"[[env]]\"}", "cpu_usage{host=\"server-a\", env=\"production\"}"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := expandVariables(tt.query, vars)
			if got != tt.expected {
				t.Errorf("expandVariables() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatLegend(t *testing.T) {
	metric := map[string]string{
		"host": "server-a",
		"job":  "node",
	}

	tests := []struct {
		template string
		expected string
	}{
		{"", "{host=\"server-a\",job=\"node\"}"},
		{"{{host}}", "server-a"},
		{"{{host}}-{{job}}", "server-a-node"},
		{"{{missing}}", ""},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			got := formatLegend(tt.template, metric)
			if got != tt.expected {
				t.Errorf("formatLegend() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestQueryMetricSummary(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/datasources/proxy/uid/prom-123/api/v1/query_range" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := PrometheusResponse{
			Status: "success",
		}
		resp.Data.ResultType = "matrix"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		}{
			{
				Metric: map[string]string{
					"host": "server-a",
				},
				Values: [][]interface{}{
					{1688547600.0, "10"},
					{1688547615.0, "20"},
					{1688547630.0, "30"},
					{1688547645.0, "40"},
					{1688547660.0, "50"},
				},
			},
			{
				Metric: map[string]string{
					"host": "server-b",
				},
				Values: [][]interface{}{
					{1688547600.0, "5"},
					{1688547615.0, "15"},
					{1688547630.0, "25"},
					{1688547645.0, "35"},
					{1688547660.0, "45"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewGrafanaClient(mockServer.URL, "test-token", "prom-123")

	// Save original local location and set to UTC for test consistency
	origLocal := time.Local
	time.Local = time.UTC
	defer func() {
		time.Local = origLocal
	}()

	res, err := client.QueryMetricSummary(
		context.Background(),
		"cpu_usage{host=\"$host\"}",
		map[string]string{"host": "server-*"},
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
		"p99",
		true, // reverse (server-a: 50 > server-b: 45, so server-a should come first)
		10,
		0,
	)

	if err != nil {
		t.Fatalf("QueryMetricSummary failed: %v", err)
	}

	if len(res.Data) != 2 {
		t.Fatalf("Expected 2 series, got %d", len(res.Data))
	}

	if res.Data[0].Legend != "server-a" {
		t.Errorf("Expected first series to be server-a, got %s", res.Data[0].Legend)
	}
	if res.Data[1].Legend != "server-b" {
		t.Errorf("Expected second series to be server-b, got %s", res.Data[1].Legend)
	}

	sa := res.Data[0]
	if sa.Samples != 5 {
		t.Errorf("Expected 5 samples, got %d", sa.Samples)
	}
	if sa.Min != 10 {
		t.Errorf("Expected min 10, got %v", sa.Min)
	}
	if sa.Max != 50 {
		t.Errorf("Expected max 50, got %v", sa.Max)
	}
	if sa.P50 != 30 {
		t.Errorf("Expected p50 30, got %v", sa.P50)
	}
	if sa.P99 != 50 {
		t.Errorf("Expected p99 50, got %v", sa.P99)
	}
	if sa.Last != 50 {
		t.Errorf("Expected last 50, got %v", sa.Last)
	}

	if res.Meta.From != "2026-07-05T17:00:00Z" {
		t.Errorf("Expected Meta.From 2026-07-05T17:00:00Z, got %s", res.Meta.From)
	}
	if res.Meta.To != "2026-07-05T18:00:00Z" {
		t.Errorf("Expected Meta.To 2026-07-05T18:00:00Z, got %s", res.Meta.To)
	}
	if res.Meta.GrafanaExplorerURL == "" {
		t.Errorf("Expected GrafanaExplorerURL to be populated")
	}
}
