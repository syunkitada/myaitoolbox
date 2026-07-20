package domain

import "time"

// Metadata represents the runtime-controlled task metadata.
// Agents must NOT update this directly.
type Metadata struct {
	ID              string     `yaml:"id"`
	Title           string     `yaml:"title"`
	Status          TaskStatus `yaml:"status"`
	CurrentAssignee string     `yaml:"current_assignee"`
	Priority        string     `yaml:"priority"`
	RetryCount      int        `yaml:"retry_count"`
	MaxRetry        int        `yaml:"max_retry"`
	CreatedAt       string     `yaml:"created_at"`
	UpdatedAt       string     `yaml:"updated_at"`
	Source          string     `yaml:"source"`
}

// DefaultMaxRetry is the default maximum number of retries before handing off to a human.
const DefaultMaxRetry = 3

// EnsureDefaults sets default values for fields that were zero-valued in YAML.
func (m *Metadata) EnsureDefaults() {
	if m.MaxRetry == 0 {
		m.MaxRetry = DefaultMaxRetry
	}
	if m.Priority == "" {
		m.Priority = "normal"
	}
}

// NowISO returns the current time in ISO 8601 format.
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
