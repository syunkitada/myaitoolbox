package infrastructure

import (
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
			got, err := ParseFlexibleTime(tt.timeStr, baseTime, tt.isFrom)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlexibleTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.expected) {
				t.Errorf("ParseFlexibleTime() = %v, want %v", got, tt.expected)
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
			got := ExpandVariables(tt.query, vars)
			if got != tt.expected {
				t.Errorf("ExpandVariables() = %q, want %q", got, tt.expected)
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
			got := FormatLegend(tt.template, metric)
			if got != tt.expected {
				t.Errorf("FormatLegend() = %q, want %q", got, tt.expected)
			}
		})
	}
}
