package domain

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Handoff represents a routing instruction from an agent to the next assignee.
type Handoff struct {
	NextAssignee string `yaml:"next_assignee"`
	Reason       string `yaml:"reason"`
}

var (
	yamlBlockPattern = regexp.MustCompile("(?s)```yaml\\s*\n(.*?)\n\\s*```")
)

// ParseHandoff performs tolerant parsing of handoff YAML content.
// It handles both raw YAML and YAML wrapped in markdown code blocks.
func ParseHandoff(data []byte) (*Handoff, error) {
	cleaned := extractYAMLBlock(data)

	var h Handoff
	if err := parseYAML(cleaned, &h); err != nil {
		return nil, fmt.Errorf("failed to parse handoff yaml: %w", err)
	}
	if h.NextAssignee == "" {
		return nil, fmt.Errorf("handoff missing required field: next_assignee")
	}
	return &h, nil
}

// extractYAMLBlock extracts YAML from markdown code blocks if present,
// otherwise returns the original data.
func extractYAMLBlock(data []byte) []byte {
	if matches := yamlBlockPattern.FindSubmatch(data); len(matches) > 1 {
		return matches[1]
	}
	return data
}

func parseYAML(data []byte, out interface{}) error {
	return yaml.Unmarshal(data, out)
}
