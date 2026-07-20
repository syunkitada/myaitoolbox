package application

import (
	"testing"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

func TestParseTime(t *testing.T) {
	base := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{"plus duration", "+1h", base.Add(1 * time.Hour), false},
		{"plus 30m", "+30m", base.Add(30 * time.Minute), false},
		{"RFC3339", "2026-07-06T10:00:00Z", time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC), false},
		{"invalid duration", "+invalid", time.Time{}, true},
		{"invalid RFC3339", "not-a-time", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.input, base)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.expected) {
				t.Errorf("ParseTime() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseMatchers(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantFirst domain.Matcher
		wantErr   bool
	}{
		{"single equals", `alertname="CPU"`, 1, domain.Matcher{Name: "alertname", Value: "CPU", IsRegex: false, IsEqual: true}, false},
		{"regex match", `host=~"server-.*"`, 1, domain.Matcher{Name: "host", Value: "server-.*", IsRegex: true, IsEqual: true}, false},
		{"not equal", `severity!="critical"`, 1, domain.Matcher{Name: "severity", Value: "critical", IsRegex: false, IsEqual: false}, false},
		{"not regex", `host!~"db-.*"`, 1, domain.Matcher{Name: "host", Value: "db-.*", IsRegex: true, IsEqual: false}, false},
		{"multiple", `alertname="CPU",severity="critical"`, 2, domain.Matcher{Name: "alertname", Value: "CPU", IsEqual: true}, false},
		{"no match", `invalid`, 0, domain.Matcher{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchers, err := ParseMatchers(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMatchers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(matchers) != tt.wantCount {
				t.Errorf("expected %d matchers, got %d", tt.wantCount, len(matchers))
			}
			if tt.wantCount > 0 {
				if matchers[0].Name != tt.wantFirst.Name || matchers[0].Value != tt.wantFirst.Value {
					t.Errorf("expected first matcher %+v, got %+v", tt.wantFirst, matchers[0])
				}
				if matchers[0].IsRegex != tt.wantFirst.IsRegex || matchers[0].IsEqual != tt.wantFirst.IsEqual {
					t.Errorf("expected first matcher flags {IsRegex:%v IsEqual:%v}, got {IsRegex:%v IsEqual:%v}",
						tt.wantFirst.IsRegex, tt.wantFirst.IsEqual, matchers[0].IsRegex, matchers[0].IsEqual)
				}
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected string
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"single", map[string]string{"host": "server-a"}, `host="server-a"`},
		{"multiple sorted", map[string]string{"b": "2", "a": "1"}, `a="1",b="2"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLabels(tt.input)
			if got != tt.expected {
				t.Errorf("FormatLabels() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatSelectedLabels(t *testing.T) {
	labels := map[string]string{
		"alertname": "CPU",
		"host":      "server-a",
		"severity":  "critical",
	}

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{"no keys", nil, ""},
		{"single key", []string{"alertname"}, `alertname="CPU"`},
		{"multiple keys", []string{"alertname", "host"}, `alertname="CPU",host="server-a"`},
		{"missing key", []string{"alertname", "missing"}, `alertname="CPU"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSelectedLabels(labels, tt.keys...)
			if got != tt.expected {
				t.Errorf("FormatSelectedLabels() = %q, want %q", got, tt.expected)
			}
		})
	}
}
