package domain

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type AlertStatus string

type Alert struct {
	Labels      map[string]string
	Annotations map[string]string
	Status      AlertStatus
}

type Matcher struct {
	Name    string
	Value   string
	IsRegex bool
	IsEqual bool
}

type Silence struct {
	ID        string
	Matchers  []Matcher
	StartsAt  time.Time
	EndsAt    time.Time
	UpdatedAt time.Time
	CreatedBy string
	Comment   string
}

type AlertRepository interface {
	GetAlerts(ctx context.Context, filters ...string) ([]Alert, error)
}

type SilenceRepository interface {
	List(ctx context.Context, filters ...string) ([]Silence, error)
	Create(ctx context.Context, silence Silence) (string, error)
	Delete(ctx context.Context, id string) error
}

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

func ParseMatchers(matchersStr string) ([]Matcher, error) {
	var matchers []Matcher
	re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)(!=|!~|=~|=)("[^"]*"|[^,]+)`)
	matches := re.FindAllStringSubmatch(matchersStr, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no valid matchers found in: %s", matchersStr)
	}

	for _, m := range matches {
		name := m[1]
		op := m[2]
		val := m[3]
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = val[1 : len(val)-1]
		}
		matchers = append(matchers, Matcher{
			Name:    name,
			Value:   val,
			IsRegex: op == "=~" || op == "!~",
			IsEqual: op == "=" || op == "=~",
		})
	}

	return matchers, nil
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
