package infrastructure

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

var LegendRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func ParseFlexibleTime(timeStr string, baseTime time.Time, isFrom bool) (time.Time, error) {
	if timeStr == "" {
		if isFrom {
			return baseTime.Add(-1 * time.Hour), nil
		}
		return baseTime, nil
	}

	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "now" {
		return baseTime, nil
	}

	if strings.HasPrefix(timeStr, "now-") {
		dStr := timeStr[4:]
		d, err := ParseDuration(dStr)
		if err != nil {
			return time.Time{}, err
		}
		return baseTime.Add(-d), nil
	}

	if d, err := ParseDuration(timeStr); err == nil {
		if d < 0 {
			return baseTime.Add(d), nil
		}
		return baseTime.Add(-d), nil
	}

	if strings.HasPrefix(timeStr, "-") {
		d, err := ParseDuration(timeStr[1:])
		if err == nil {
			return baseTime.Add(-d), nil
		}
	}

	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
}

func ParseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		valStr := s[:len(s)-1]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * float64(24*time.Hour)), nil
	}
	if strings.HasSuffix(s, "w") {
		valStr := s[:len(s)-1]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(val * float64(7*24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func ExpandVariables(query string, vars map[string]string) string {
	for k, v := range vars {
		query = strings.ReplaceAll(query, "$"+k, v)
		query = strings.ReplaceAll(query, "${"+k+"}", v)
		query = strings.ReplaceAll(query, "[["+k+"]]", v)
	}
	return query
}

func FormatLegend(template string, metric map[string]string) string {
	if template == "" {
		return FormatMetricLabels(metric)
	}
	return LegendRegex.ReplaceAllStringFunc(template, func(match string) string {
		labelName := LegendRegex.FindStringSubmatch(match)[1]
		labelName = strings.TrimSpace(labelName)
		if val, ok := metric[labelName]; ok {
			return val
		}
		return ""
	})
}

func FormatMetricLabels(metric map[string]string) string {
	if len(metric) == 0 {
		return "{}"
	}
	var keys []string
	for k := range metric {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, metric[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func ComputeStep(duration time.Duration, divisor int) string {
	stepSeconds := int(duration.Seconds() / float64(divisor))
	if stepSeconds < 15 {
		stepSeconds = 15
	}
	return fmt.Sprintf("%ds", stepSeconds)
}

func FormatTime(t time.Time, duration time.Duration, stepSeconds int) string {
	if duration > 24*time.Hour {
		return t.Format("2006-01-02 15:04")
	}
	if stepSeconds < 60 {
		return t.Format("15:04:05")
	}
	return t.Format("15:04")
}

func Percentiles(vals []float64) (p50, p90, p99 float64) {
	if len(vals) == 0 {
		return
	}
	sortedVals := make([]float64, len(vals))
	copy(sortedVals, vals)
	sort.Float64s(sortedVals)

	n := len(sortedVals) - 1
	p50 = sortedVals[int(math.Round(float64(n)*0.5))]
	p90 = sortedVals[int(math.Round(float64(n)*0.9))]
	p99 = sortedVals[int(math.Round(float64(n)*0.99))]

	return
}

func BuildExploreURL(baseURL, datasourceUID string, queries []domain.ExploreQueryDef, fromTime, toTime time.Time) string {
	type ExploreRange struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	type ExploreDatasource struct {
		Type string `json:"type"`
		UID  string `json:"uid"`
	}
	type ExploreQuery struct {
		RefID      string            `json:"refId"`
		Expr       string            `json:"expr"`
		Datasource ExploreDatasource `json:"datasource"`
	}
	type ExploreLeft struct {
		Datasource string         `json:"datasource"`
		Queries    []ExploreQuery `json:"queries"`
		Range      ExploreRange   `json:"range"`
	}

	var exploreQueries []ExploreQuery
	for i, eq := range queries {
		refID := string(rune('A' + i))
		exploreQueries = append(exploreQueries, ExploreQuery{
			RefID: refID,
			Expr:  eq.Expr,
			Datasource: ExploreDatasource{
				Type: "prometheus",
				UID:  datasourceUID,
			},
		})
	}

	left := ExploreLeft{
		Datasource: datasourceUID,
		Queries:    exploreQueries,
		Range: ExploreRange{
			From: fromTime.Format(time.RFC3339),
			To:   toTime.Format(time.RFC3339),
		},
	}

	leftJSON, err := json.Marshal(left)
	if err != nil {
		return ""
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/explore?left=%s", baseURL, url.QueryEscape(string(leftJSON)))
}
