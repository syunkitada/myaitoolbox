package markdown

import (
	"strings"

	"github.com/goccy/go-yaml"
)

func splitFrontMatter(content string) (fmStr string, body string, ok bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, false
	}
	var fm []string
	restIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			restIdx = i + 1
			break
		}
		fm = append(fm, lines[i])
	}
	if restIdx == -1 {
		return "", content, false
	}
	return strings.Join(fm, "\n"), strings.TrimLeft(strings.Join(lines[restIdx:], "\n"), "\n"), true
}

func parseFrontMatter(fmStr string) (map[string]any, error) {
	fm := map[string]any{}
	if strings.TrimSpace(fmStr) == "" {
		return fm, nil
	}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, err
	}
	return fm, nil
}

func extractStatus(content string) string {
	fmStr, _, ok := splitFrontMatter(content)
	if !ok {
		return ""
	}
	fm, err := parseFrontMatter(fmStr)
	if err != nil {
		return ""
	}
	status, _ := fm["status"].(string)
	return status
}

func buildFrontMatter(fields map[string]any, body string) (string, error) {
	fmBytes, err := yaml.Marshal(fields)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString("\n")
		sb.WriteString(body)
	}
	return sb.String(), nil
}
