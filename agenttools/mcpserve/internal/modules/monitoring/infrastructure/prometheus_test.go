package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

		resp := prometheusResponse{
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

	origLocal := time.Local
	time.Local = time.UTC
	defer func() {
		time.Local = origLocal
	}()

	data, meta, err := client.QuerySummary(
		context.Background(),
		"cpu_usage{host=\"$host\"}",
		map[string]string{"host": "server-*"},
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
		"p99",
		true,
		10,
		0,
	)

	if err != nil {
		t.Fatalf("QuerySummary failed: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("Expected 2 series, got %d", len(data))
	}

	if data[0].Legend != "server-a" {
		t.Errorf("Expected first series to be server-a, got %s", data[0].Legend)
	}
	if data[1].Legend != "server-b" {
		t.Errorf("Expected second series to be server-b, got %s", data[1].Legend)
	}

	sa := data[0]
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

	if meta.From != "2026-07-05T17:00:00Z" {
		t.Errorf("Expected Meta.From 2026-07-05T17:00:00Z, got %s", meta.From)
	}
	if meta.To != "2026-07-05T18:00:00Z" {
		t.Errorf("Expected Meta.To 2026-07-05T18:00:00Z, got %s", meta.To)
	}
	if meta.GrafanaExplorerURL == "" {
		t.Errorf("Expected GrafanaExplorerURL to be populated")
	}
}

func TestQueryMetricHistory(t *testing.T) {
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

		resp := prometheusResponse{Status: "success"}
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

	data, meta, err := client.QueryHistory(
		context.Background(),
		[]string{"cpu_usage"},
		nil,
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
	)

	if err != nil {
		t.Fatalf("QueryHistory failed: %v", err)
	}

	if meta.Legend != "" {
		t.Errorf("Expected meta.legend to be empty for single-query mode, got %q", meta.Legend)
	}

	if len(data) != 2 {
		t.Fatalf("Expected 2 data points, got %d", len(data))
	}

	dp1 := data[0]
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

	dp1JSON, err := json.Marshal(dp1)
	if err != nil {
		t.Fatalf("Failed to marshal dp1: %v", err)
	}
	if !strings.HasPrefix(string(dp1JSON), `{"time":"17:00"`) {
		t.Errorf("Expected time to be the first key in JSON, got: %s", string(dp1JSON))
	}

	dp2 := data[1]
	t2, _ := dp2.Get("time")
	if t2 != "17:01" {
		t.Errorf("Expected second time to be 17:01, got %v", t2)
	}

	if meta.TimeFrom != "2026-07-05T17:00:00Z" {
		t.Errorf("Expected Meta.TimeFrom 2026-07-05T17:00:00Z, got %s", meta.TimeFrom)
	}
	if meta.TimeTo != "2026-07-05T18:00:00Z" {
		t.Errorf("Expected Meta.TimeTo 2026-07-05T18:00:00Z, got %s", meta.TimeTo)
	}
	if meta.GrafanaExplorerURL == "" {
		t.Errorf("Expected GrafanaExplorerURL to be populated")
	}
}

func TestQueryMetricHistoryMultiQuery(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		resp := prometheusResponse{Status: "success"}
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

	data, meta, err := client.QueryHistory(
		context.Background(),
		[]string{"cpu_usage", "disk_usage"},
		nil,
		"{{host}}",
		"2026-07-05T17:00:00Z",
		"2026-07-05T18:00:00Z",
	)
	if err != nil {
		t.Fatalf("QueryHistory failed: %v", err)
	}

	if meta.Legend != "host1" {
		t.Errorf("Expected meta.legend to be host1, got %q", meta.Legend)
	}
	if len(data) != 2 {
		t.Fatalf("Expected 2 data points, got %d", len(data))
	}

	dp1 := data[0]

	dp1JSON, err := json.Marshal(dp1)
	if err != nil {
		t.Fatalf("Failed to marshal dp1: %v", err)
	}
	if !strings.HasPrefix(string(dp1JSON), `{"time":"17:00"`) {
		t.Errorf("Expected time to be first key, got: %s", string(dp1JSON))
	}

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
		resp := prometheusResponse{Status: "success"}
		resp.Data.ResultType = "matrix"

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

	_, _, err := client.QueryHistory(
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
