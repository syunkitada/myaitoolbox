package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestQueryMetricHistory(t *testing.T) {
	// Single-query mode: columns are legend values (server-a, server-b)
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

		resp := PrometheusResponse{Status: "success"}
		resp.Data.ResultType = "matrix"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		}{
			{
				Metric: map[string]string{"host": "server-a"},
				Values: [][]interface{}{
					{1783270800.0, "10"},
					{1783270860.0, "20"},
				},
			},
			{
				Metric: map[string]string{"host": "server-b"},
				Values: [][]interface{}{
					{1783270800.0, "100"},
					{1783270860.0, "200"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewGrafanaClient(mockServer.URL, "test-token", "prom-123")

	origLocal := time.Local
	time.Local = time.UTC
	defer func() {
		time.Local = origLocal
	}()

	res, err := client.QueryMetricHistory(
		context.Background(),
		[]string{"cpu_usage"},
		nil,
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
	)

	if err != nil {
		t.Fatalf("QueryMetricHistory failed: %v", err)
	}

	// Single-query: meta.legend should be empty
	if res.Meta.Legend != "" {
		t.Errorf("Expected meta.legend to be empty for single-query mode, got %q", res.Meta.Legend)
	}

	if len(res.Data) != 2 {
		t.Fatalf("Expected 2 data points, got %d", len(res.Data))
	}

	dp1 := res.Data[0]
	t1, _ := dp1.Get("time")
	if t1 != "17:00" {
		t.Errorf("Expected first time to be 17:00, got %v", t1)
	}
	sa1, _ := dp1.Get("server-a")
	if sa1 != 10.0 {
		t.Errorf("Expected server-a to be 10, got %v", sa1)
	}
	sb1, _ := dp1.Get("server-b")
	if sb1 != 100.0 {
		t.Errorf("Expected server-b to be 100, got %v", sb1)
	}

	// Verify that the time key is serialized first in JSON output
	dp1JSON, err := json.Marshal(dp1)
	if err != nil {
		t.Fatalf("Failed to marshal dp1: %v", err)
	}
	if !strings.HasPrefix(string(dp1JSON), `{"time":"17:00"`) {
		t.Errorf("Expected time to be the first key in JSON, got: %s", string(dp1JSON))
	}

	dp2 := res.Data[1]
	t2, _ := dp2.Get("time")
	if t2 != "17:01" {
		t.Errorf("Expected second time to be 17:01, got %v", t2)
	}
	sa2, _ := dp2.Get("server-a")
	if sa2 != 20.0 {
		t.Errorf("Expected server-a to be 20, got %v", sa2)
	}
	sb2, _ := dp2.Get("server-b")
	if sb2 != 200.0 {
		t.Errorf("Expected server-b to be 200, got %v", sb2)
	}

	if res.Meta.TimeFrom != "2026-07-05T17:00:00Z" {
		t.Errorf("Expected Meta.TimeFrom 2026-07-05T17:00:00Z, got %s", res.Meta.TimeFrom)
	}
	if res.Meta.TimeTo != "2026-07-05T18:00:00Z" {
		t.Errorf("Expected Meta.TimeTo 2026-07-05T18:00:00Z, got %s", res.Meta.TimeTo)
	}
	if res.Meta.GrafanaExplorerURL == "" {
		t.Errorf("Expected GrafanaExplorerURL to be populated")
	}
}

func TestQueryMetricHistoryMultiQuery(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		resp := PrometheusResponse{Status: "success"}
		resp.Data.ResultType = "matrix"

		switch query {
		case "cpu_usage":
			resp.Data.Result = []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			}{
				{
					Metric: map[string]string{"host": "host1"},
					Values: [][]interface{}{
						{1783270800.0, "10"},
						{1783270860.0, "11"},
					},
				},
			}
		case "disk_usage":
			resp.Data.Result = []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			}{
				{
					Metric: map[string]string{"host": "host1"},
					Values: [][]interface{}{
						{1783270800.0, "20"},
						{1783270860.0, "21"},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewGrafanaClient(mockServer.URL, "test-token", "prom-123")

	origLocal := time.Local
	time.Local = time.UTC
	defer func() { time.Local = origLocal }()

	res, err := client.QueryMetricHistory(
		context.Background(),
		[]string{"cpu_usage", "disk_usage"},
		nil,
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
	)
	if err != nil {
		t.Fatalf("QueryMetricHistory failed: %v", err)
	}

	if res.Meta.Legend != "host1" {
		t.Errorf("Expected meta.legend to be host1, got %q", res.Meta.Legend)
	}
	if len(res.Data) != 2 {
		t.Fatalf("Expected 2 data points, got %d", len(res.Data))
	}

	dp1 := res.Data[0]

	// Verify time is first
	dp1JSON, err := json.Marshal(dp1)
	if err != nil {
		t.Fatalf("Failed to marshal dp1: %v", err)
	}
	if !strings.HasPrefix(string(dp1JSON), `{"time":"17:00"`) {
		t.Errorf("Expected time to be first key, got: %s", string(dp1JSON))
	}

	// Verify query columns are present in query order
	if dp1[0].Key != "time" {
		t.Errorf("Expected first key to be time, got %q", dp1[0].Key)
	}
	if dp1[1].Key != "cpu_usage" {
		t.Errorf("Expected second key to be cpu_usage, got %q", dp1[1].Key)
	}
	if dp1[1].Value != 10.0 {
		t.Errorf("Expected cpu_usage to be 10, got %v", dp1[1].Value)
	}
	if dp1[2].Key != "disk_usage" {
		t.Errorf("Expected third key to be disk_usage, got %q", dp1[2].Key)
	}
	if dp1[2].Value != 20.0 {
		t.Errorf("Expected disk_usage to be 20, got %v", dp1[2].Value)
	}
}

func TestQueryMetricHistoryMultiQueryMultipleLegends(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		resp := PrometheusResponse{Status: "success"}
		resp.Data.ResultType = "matrix"

		// Return two different hosts -> legend is not unique
		if query == "cpu_usage" {
			resp.Data.Result = []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			}{
				{
					Metric: map[string]string{"host": "host1"},
					Values: [][]interface{}{{1783270800.0, "10"}},
				},
				{
					Metric: map[string]string{"host": "host2"},
					Values: [][]interface{}{{1783270800.0, "20"}},
				},
			}
		} else {
			resp.Data.Result = []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewGrafanaClient(mockServer.URL, "test-token", "prom-123")

	_, err := client.QueryMetricHistory(
		context.Background(),
		[]string{"cpu_usage", "disk_usage"},
		nil,
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
	)
	if err == nil {
		t.Fatal("Expected error when multiple legends are resolved in multi-query mode, got nil")
	}
	if !strings.Contains(err.Error(), "multi-query requires legend to resolve to exactly one value") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
