package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

type mockAlertRepo struct {
	alerts []domain.Alert
	err    error
}

func (m *mockAlertRepo) GetAlerts(ctx context.Context, filters ...string) ([]domain.Alert, error) {
	return m.alerts, m.err
}

type mockSilenceRepo struct {
	silences []domain.Silence
	createID string
	err      error
}

func (m *mockSilenceRepo) List(ctx context.Context, filters ...string) ([]domain.Silence, error) {
	return m.silences, m.err
}

func (m *mockSilenceRepo) Create(ctx context.Context, silence domain.Silence) (string, error) {
	return m.createID, m.err
}

func (m *mockSilenceRepo) Delete(ctx context.Context, id string) error {
	return m.err
}

type mockMetricRepo struct {
	summary     []domain.MetricSummary
	summaryMeta domain.SummaryMeta
	history     []domain.OrderedMap
	historyMeta domain.HistoryMeta
	err         error
}

func (m *mockMetricRepo) QuerySummary(ctx context.Context, query string, vars map[string]string, legendTemplate string, timeFrom, timeTo string, sortField string, reverse bool, limit, offset int) ([]domain.MetricSummary, domain.SummaryMeta, error) {
	return m.summary, m.summaryMeta, m.err
}

func (m *mockMetricRepo) QueryHistory(ctx context.Context, queries []string, vars map[string]string, legendTemplate string, timeFrom, timeTo string) ([]domain.OrderedMap, domain.HistoryMeta, error) {
	return m.history, m.historyMeta, m.err
}

func callToolReq(t *testing.T, args map[string]interface{}) *mcp.CallToolRequest {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test",
			Arguments: raw,
		},
	}
}

func TestListAlerts_Success(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{
			alerts: []domain.Alert{
				{Labels: map[string]string{"alertname": "CPU", "host": "server-a"}, Status: domain.AlertStatus("firing")},
				{Labels: map[string]string{"alertname": "MEM", "host": "server-b"}, Status: domain.AlertStatus("resolved")},
			},
		},
		&mockSilenceRepo{},
		nil,
	)

	data, meta, err := app.ListAlerts(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, ok := data.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", meta)
	}
	if metaMap["count"] != 2 {
		t.Errorf("expected count 2, got %v", metaMap["count"])
	}
}

func TestListAlerts_FilterByStatus(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{
			alerts: []domain.Alert{
				{Labels: map[string]string{"alertname": "CPU", "host": "server-a"}, Status: "firing"},
				{Labels: map[string]string{"alertname": "MEM", "host": "server-b"}, Status: "resolved"},
			},
		},
		&mockSilenceRepo{},
		nil,
	)

	data, _, err := app.ListAlerts(context.Background(), callToolReq(t, map[string]interface{}{"status": "firing"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items := data.([]map[string]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestListAlerts_Empty(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{alerts: []domain.Alert{}},
		&mockSilenceRepo{},
		nil,
	)

	data, meta, err := app.ListAlerts(context.Background(), callToolReq(t, map[string]interface{}{}))
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

func TestListAlerts_RepoError(t *testing.T) {
	app := NewApp(&mockAlertRepo{err: assertError("repo down")}, &mockSilenceRepo{}, nil)

	_, _, err := app.ListAlerts(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil || err.Error() != "failed to get alerts: repo down" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSilence_Success(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{createID: "silence-123"},
		nil,
	)

	data, meta, err := app.CreateSilence(context.Background(), callToolReq(t, map[string]interface{}{
		"matchers":   `alertname="Test"`,
		"endat":      "+1h",
		"comment":    "test silence",
		"created_by": "tester",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := data.(map[string]interface{})
	if result["id"] != "silence-123" {
		t.Errorf("expected id silence-123, got %v", result["id"])
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["created_by"] != "tester" {
		t.Errorf("expected created_by tester, got %v", metaMap["created_by"])
	}
}

func TestCreateSilence_InvalidMatchers(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.CreateSilence(context.Background(), callToolReq(t, map[string]interface{}{
		"matchers": "invalid",
		"endat":    "+1h",
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateSilence_EndBeforeStart(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.CreateSilence(context.Background(), callToolReq(t, map[string]interface{}{
		"matchers": `alertname="Test"`,
		"endat":    "-1h",
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateSilence_RepoError(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{err: assertError("create failed")},
		nil,
	)

	_, _, err := app.CreateSilence(context.Background(), callToolReq(t, map[string]interface{}{
		"matchers": `alertname="Test"`,
		"endat":    "+1h",
	}))
	if err == nil || err.Error() != "failed to create silence: create failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListSilences_Success(t *testing.T) {
	now := time.Now()
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{
			silences: []domain.Silence{
				{
					ID: "s-1",
					Matchers: []domain.Matcher{
						{Name: "alertname", Value: "CPU"},
						{Name: "host", Value: "server-a"},
					},
					StartsAt:  now,
					EndsAt:    now.Add(1 * time.Hour),
					Comment:   "test",
					CreatedBy: "tester",
				},
			},
		},
		nil,
	)

	data, meta, err := app.ListSilences(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items := data.([]map[string]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0]["id"] != "s-1" {
		t.Errorf("expected id s-1, got %v", items[0]["id"])
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["count"] != 1 {
		t.Errorf("expected count 1, got %v", metaMap["count"])
	}
}

func TestListSilences_Verbose(t *testing.T) {
	now := time.Now()
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{
			silences: []domain.Silence{
				{
					ID: "s-1",
					Matchers: []domain.Matcher{
						{Name: "alertname", Value: "CPU"},
					},
					StartsAt:  now,
					EndsAt:    now.Add(1 * time.Hour),
					Comment:   "test comment",
					CreatedBy: "tester",
				},
			},
		},
		nil,
	)

	data, _, err := app.ListSilences(context.Background(), callToolReq(t, map[string]interface{}{"verbose": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items := data.([]map[string]interface{})
	if items[0]["comment"] != "test comment" {
		t.Errorf("expected comment in verbose mode, got %v", items[0]["comment"])
	}
	if items[0]["author"] != "tester" {
		t.Errorf("expected author in verbose mode, got %v", items[0]["author"])
	}
}

func TestListSilences_Empty(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	data, meta, err := app.ListSilences(context.Background(), callToolReq(t, map[string]interface{}{}))
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

func TestListAlerts_InvalidJSON(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.ListAlerts(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test",
			Arguments: json.RawMessage(`invalid`),
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestDeleteSilence_Success(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	data, meta, err := app.DeleteSilence(context.Background(), callToolReq(t, map[string]interface{}{"id": "s-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}

	metaMap := meta.(map[string]interface{})
	if metaMap["status"] != "deleted" {
		t.Errorf("expected status deleted, got %v", metaMap["status"])
	}
	if metaMap["id"] != "s-1" {
		t.Errorf("expected id s-1, got %v", metaMap["id"])
	}
}

func TestDeleteSilence_MissingID(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.DeleteSilence(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteSilence_RepoError(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{err: assertError("delete failed")},
		nil,
	)

	_, _, err := app.DeleteSilence(context.Background(), callToolReq(t, map[string]interface{}{"id": "s-1"}))
	if err == nil || err.Error() != "failed to delete silence: delete failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQueryMetricSummary_Success(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{},
		&mockMetricRepo{
			summary: []domain.MetricSummary{
				{Legend: "host1", Samples: 10, Min: 1, Max: 100},
			},
			summaryMeta: domain.SummaryMeta{
				Query: "cpu_usage",
				From:  "2026-01-01T00:00:00Z",
				To:    "2026-01-01T01:00:00Z",
			},
		},
	)

	data, meta, err := app.QueryMetricSummary(context.Background(), callToolReq(t, map[string]interface{}{
		"query": "cpu_usage",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summaries, ok := data.([]domain.MetricSummary)
	if !ok {
		t.Fatalf("expected []domain.MetricSummary, got %T", data)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	_, ok = meta.(domain.SummaryMeta)
	if !ok {
		t.Fatalf("expected domain.SummaryMeta, got %T", meta)
	}
}

func TestQueryMetricSummary_MissingQuery(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, &mockMetricRepo{})

	_, _, err := app.QueryMetricSummary(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQueryMetricSummary_NilRepo(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.QueryMetricSummary(context.Background(), callToolReq(t, map[string]interface{}{
		"query": "cpu_usage",
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQueryMetricHistory_Success(t *testing.T) {
	app := NewApp(
		&mockAlertRepo{},
		&mockSilenceRepo{},
		&mockMetricRepo{
			history: []domain.OrderedMap{
				{{Key: "time", Value: "12:00"}, {Key: "host1", Value: 10.0}},
			},
			historyMeta: domain.HistoryMeta{
				Queries:  []string{"cpu_usage"},
				TimeFrom: "2026-01-01T00:00:00Z",
				TimeTo:   "2026-01-01T01:00:00Z",
			},
		},
	)

	data, meta, err := app.QueryMetricHistory(context.Background(), callToolReq(t, map[string]interface{}{
		"query": []interface{}{"cpu_usage"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, ok := data.([]domain.OrderedMap)
	if !ok {
		t.Fatalf("expected []domain.OrderedMap, got %T", data)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(history))
	}

	_, ok = meta.(domain.HistoryMeta)
	if !ok {
		t.Fatalf("expected domain.HistoryMeta, got %T", meta)
	}
}

func TestQueryMetricHistory_MissingQuery(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, &mockMetricRepo{})

	_, _, err := app.QueryMetricHistory(context.Background(), callToolReq(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQueryMetricHistory_NilRepo(t *testing.T) {
	app := NewApp(&mockAlertRepo{}, &mockSilenceRepo{}, nil)

	_, _, err := app.QueryMetricHistory(context.Background(), callToolReq(t, map[string]interface{}{
		"query": []interface{}{"cpu_usage"},
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseVars(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "no vars",
			args:     map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name: "single var",
			args: map[string]interface{}{
				"var": []interface{}{"host=server-a"},
			},
			expected: map[string]string{"host": "server-a"},
		},
		{
			name: "multiple vars",
			args: map[string]interface{}{
				"var": []interface{}{"host=server-a", "env=prod"},
			},
			expected: map[string]string{"host": "server-a", "env": "prod"},
		},
		{
			name: "invalid var ignored",
			args: map[string]interface{}{
				"var": []interface{}{"host=server-a", "invalid-format"},
			},
			expected: map[string]string{"host": "server-a"},
		},
		{
			name: "non-string item ignored",
			args: map[string]interface{}{
				"var": []interface{}{"host=server-a", 42},
			},
			expected: map[string]string{"host": "server-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVars(tt.args)
			if len(got) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, got[k])
				}
			}
		})
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
