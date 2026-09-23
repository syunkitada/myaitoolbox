package application

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/domain"
)

func ParseTime(timeStr string, baseTime time.Time) (time.Time, error) {
	if strings.HasPrefix(timeStr, "+") {
		d, err := time.ParseDuration(timeStr[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration format: %s", timeStr)
		}
		return baseTime.Add(d), nil
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339 time format: %s", timeStr)
	}
	return t, nil
}

func ParseMatchers(matchersStr string) ([]domain.Matcher, error) {
	input := strings.TrimSpace(matchersStr)
	if input == "" {
		return nil, fmt.Errorf("no valid matchers found in: %s", matchersStr)
	}

	var matchers []domain.Matcher
	for pos := 0; pos < len(input); {
		for pos < len(input) && isSpace(input[pos]) {
			pos++
		}

		nameStart := pos
		if pos >= len(input) || !isLabelStart(input[pos]) {
			return nil, fmt.Errorf("invalid matcher near %q", input[nameStart:])
		}
		pos++
		for pos < len(input) && isLabelChar(input[pos]) {
			pos++
		}
		name := input[nameStart:pos]

		for pos < len(input) && isSpace(input[pos]) {
			pos++
		}
		op := ""
		for _, candidate := range []string{"!=", "!~", "=~", "="} {
			if strings.HasPrefix(input[pos:], candidate) {
				op = candidate
				pos += len(candidate)
				break
			}
		}
		if op == "" {
			return nil, fmt.Errorf("invalid matcher operator near %q", input[pos:])
		}

		for pos < len(input) && isSpace(input[pos]) {
			pos++
		}
		if pos >= len(input) {
			return nil, fmt.Errorf("matcher %q has no value", name)
		}

		var value string
		if input[pos] == '"' {
			valueStart := pos
			pos++
			escaped := false
			closed := false
			for pos < len(input) {
				if escaped {
					escaped = false
					pos++
					continue
				}
				if input[pos] == '\\' {
					escaped = true
					pos++
					continue
				}
				if input[pos] == '"' {
					pos++
					closed = true
					break
				}
				pos++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted value for matcher %q", name)
			}
			quoted := input[valueStart:pos]
			var err error
			value, err = strconv.Unquote(quoted)
			if err != nil {
				return nil, fmt.Errorf("invalid quoted value for matcher %q: %w", name, err)
			}
		} else {
			valueStart := pos
			for pos < len(input) && input[pos] != ',' {
				pos++
			}
			value = strings.TrimSpace(input[valueStart:pos])
			if value == "" {
				return nil, fmt.Errorf("matcher %q has no value", name)
			}
		}

		matchers = append(matchers, domain.Matcher{
			Name:    name,
			Value:   value,
			IsRegex: op == "=~" || op == "!~",
			IsEqual: op == "=" || op == "=~",
		})

		for pos < len(input) && isSpace(input[pos]) {
			pos++
		}
		if pos == len(input) {
			break
		}
		if input[pos] != ',' {
			return nil, fmt.Errorf("expected comma between matchers near %q", input[pos:])
		}
		pos++
		if pos == len(input) {
			return nil, fmt.Errorf("trailing comma in matchers")
		}
	}

	return matchers, nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isLabelStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isLabelChar(c byte) bool {
	return isLabelStart(c) || c >= '0' && c <= '9'
}

func FormatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	return strings.Join(parts, ",")
}

func FormatSelectedLabels(labels map[string]string, keys ...string) string {
	var parts []string
	for _, k := range keys {
		if v, ok := labels[k]; ok {
			parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
		}
	}
	return strings.Join(parts, ",")
}
