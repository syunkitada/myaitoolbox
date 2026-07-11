package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

type grafanaClient struct {
	client        *http.Client
	baseURL       string
	apiToken      string
	datasourceUID string
}

func NewGrafanaClient(baseURL, apiToken, datasourceUID string) domain.MetricRepository {
	return &grafanaClient{
		client:        &http.Client{Timeout: 30 * time.Second},
		baseURL:       baseURL,
		apiToken:      apiToken,
		datasourceUID: datasourceUID,
	}
}

type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (c *grafanaClient) QuerySummary(ctx context.Context, query string, vars map[string]string, legendTemplate string, timeFrom, timeTo string, sortField string, reverse bool, limit, offset int) ([]domain.MetricSummary, domain.SummaryMeta, error) {
	now := time.Now()
	toTime, err := domain.ParseFlexibleTime(timeTo, now, false)
	if err != nil {
		return nil, domain.SummaryMeta{}, fmt.Errorf("invalid time_to: %w", err)
	}
	fromTime, err := domain.ParseFlexibleTime(timeFrom, toTime, true)
	if err != nil {
		return nil, domain.SummaryMeta{}, fmt.Errorf("invalid time_from: %w", err)
	}

	if !toTime.After(fromTime) {
		return nil, domain.SummaryMeta{}, fmt.Errorf("time_to must be after time_from")
	}

	expandedQL := domain.ExpandVariables(query, vars)
	duration := toTime.Sub(fromTime)
	step := domain.ComputeStep(duration, 300)

	promResp, err := c.queryPrometheus(ctx, expandedQL, fromTime, toTime, step)
	if err != nil {
		return nil, domain.SummaryMeta{}, err
	}

	var data []domain.MetricSummary

	for _, result := range promResp.Data.Result {
		var vals []float64
		var minVal, maxVal float64
		var minTime, maxTime time.Time
		var lastVal float64
		var hasData bool

		for _, valPair := range result.Values {
			if len(valPair) < 2 {
				continue
			}
			tsFloat, ok := valPair[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := valPair[1].(string)
			if !ok {
				continue
			}
			v, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}

			t := time.Unix(int64(tsFloat), 0).In(fromTime.Location())

			if !hasData {
				minVal = v
				minTime = t
				maxVal = v
				maxTime = t
				hasData = true
			} else {
				if v < minVal {
					minVal = v
					minTime = t
				}
				if v > maxVal {
					maxVal = v
					maxTime = t
				}
			}
			vals = append(vals, v)
			lastVal = v
		}

		if len(vals) == 0 {
			continue
		}

		p50, p90, p99 := domain.Percentiles(vals)
		legend := domain.FormatLegend(legendTemplate, result.Metric)

		data = append(data, domain.MetricSummary{
			Legend:  legend,
			Samples: len(vals),
			Min:     minVal,
			MinAt:   minTime.Format(time.RFC3339),
			P50:     p50,
			P90:     p90,
			P99:     p99,
			Max:     maxVal,
			MaxAt:   maxTime.Format(time.RFC3339),
			Last:    lastVal,
		})
	}

	if sortField == "" {
		sortField = "p99"
	}
	SortMetricSummary(data, sortField, reverse)

	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	if limit <= 0 {
		limit = 100
	}
	end := offset + limit
	if end > len(data) {
		end = len(data)
	}
	data = data[offset:end]

	exploreURL := domain.BuildExploreURL(c.baseURL, c.datasourceUID, []domain.ExploreQueryDef{{Expr: expandedQL}}, fromTime, toTime)

	meta := domain.SummaryMeta{
		Query:              expandedQL,
		From:               fromTime.Format(time.RFC3339),
		To:                 toTime.Format(time.RFC3339),
		Outputs:            []string{"grafana_explore_url"},
		GrafanaExplorerURL: exploreURL,
	}

	return data, meta, nil
}

func (c *grafanaClient) QueryHistory(ctx context.Context, queries []string, vars map[string]string, legendTemplate string, timeFrom, timeTo string) ([]domain.OrderedMap, domain.HistoryMeta, error) {
	now := time.Now()
	toTime, err := domain.ParseFlexibleTime(timeTo, now, false)
	if err != nil {
		return nil, domain.HistoryMeta{}, fmt.Errorf("invalid time_to: %w", err)
	}
	fromTime, err := domain.ParseFlexibleTime(timeFrom, toTime, true)
	if err != nil {
		return nil, domain.HistoryMeta{}, fmt.Errorf("invalid time_from: %w", err)
	}

	if !toTime.After(fromTime) {
		return nil, domain.HistoryMeta{}, fmt.Errorf("time_to must be after time_from")
	}

	duration := toTime.Sub(fromTime)
	step := domain.ComputeStep(duration, 60)

	multiQuery := len(queries) > 1

	timeMap := make(map[int64]map[string]float64)
	multiMap := make(map[string]map[int64]map[string]float64)

	var expandedQueries []string

	for _, promQL := range queries {
		expandedQL := domain.ExpandVariables(promQL, vars)
		expandedQueries = append(expandedQueries, expandedQL)

		promResp, err := c.queryPrometheus(ctx, expandedQL, fromTime, toTime, step)
		if err != nil {
			return nil, domain.HistoryMeta{}, err
		}

		if multiQuery {
			for _, result := range promResp.Data.Result {
				legend := domain.FormatLegend(legendTemplate, result.Metric)
				if _, ok := multiMap[legend]; !ok {
					multiMap[legend] = make(map[int64]map[string]float64)
				}
				for _, valPair := range result.Values {
					if len(valPair) < 2 {
						continue
					}
					tsFloat, ok := valPair[0].(float64)
					if !ok {
						continue
					}
					valStr, ok := valPair[1].(string)
					if !ok {
						continue
					}
					v, err := strconv.ParseFloat(valStr, 64)
					if err != nil {
						continue
					}
					ts := int64(tsFloat)
					if _, ok := multiMap[legend][ts]; !ok {
						multiMap[legend][ts] = make(map[string]float64)
					}
					multiMap[legend][ts][expandedQL] = v
				}
			}
		} else {
			for _, result := range promResp.Data.Result {
				legend := domain.FormatLegend(legendTemplate, result.Metric)
				for _, valPair := range result.Values {
					if len(valPair) < 2 {
						continue
					}
					tsFloat, ok := valPair[0].(float64)
					if !ok {
						continue
					}
					valStr, ok := valPair[1].(string)
					if !ok {
						continue
					}
					v, err := strconv.ParseFloat(valStr, 64)
					if err != nil {
						continue
					}
					ts := int64(tsFloat)
					if _, ok := timeMap[ts]; !ok {
						timeMap[ts] = make(map[string]float64)
					}
					timeMap[ts][legend] = v
				}
			}
		}
	}

	stepSeconds := int(duration.Seconds() / 60)
	if stepSeconds < 15 {
		stepSeconds = 15
	}

	var data []domain.OrderedMap
	var resolvedLegend string

	if multiQuery {
		if len(multiMap) != 1 {
			var foundLegends []string
			for l := range multiMap {
				foundLegends = append(foundLegends, l)
			}
			sort.Strings(foundLegends)
			return nil, domain.HistoryMeta{}, fmt.Errorf("multi-query requires legend to resolve to exactly one value, got %d: %v", len(multiMap), foundLegends)
		}

		for legend := range multiMap {
			resolvedLegend = legend
		}
		tsMap := multiMap[resolvedLegend]

		var timestamps []int64
		for ts := range tsMap {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		for _, ts := range timestamps {
			var item domain.OrderedMap
			t := time.Unix(ts, 0).In(fromTime.Location())
			item = append(item, domain.MapEntry{Key: "time", Value: domain.FormatTime(t, duration, stepSeconds)})
			for _, eq := range expandedQueries {
				item = append(item, domain.MapEntry{Key: eq, Value: tsMap[ts][eq]})
			}
			data = append(data, item)
		}
	} else {
		var timestamps []int64
		for ts := range timeMap {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		for _, ts := range timestamps {
			var item domain.OrderedMap
			t := time.Unix(ts, 0).In(fromTime.Location())
			item = append(item, domain.MapEntry{Key: "time", Value: domain.FormatTime(t, duration, stepSeconds)})

			var legends []string
			for legend := range timeMap[ts] {
				legends = append(legends, legend)
			}
			sort.Strings(legends)

			for _, legend := range legends {
				item = append(item, domain.MapEntry{Key: legend, Value: timeMap[ts][legend]})
			}
			data = append(data, item)
		}
	}

	var exploreDefs []domain.ExploreQueryDef
	for _, eq := range expandedQueries {
		exploreDefs = append(exploreDefs, domain.ExploreQueryDef{Expr: eq})
	}
	exploreURL := domain.BuildExploreURL(c.baseURL, c.datasourceUID, exploreDefs, fromTime, toTime)

	meta := domain.HistoryMeta{
		Queries:            expandedQueries,
		Legend:             resolvedLegend,
		TimeFrom:           fromTime.Format(time.RFC3339),
		TimeTo:             toTime.Format(time.RFC3339),
		Outputs:            []string{"grafana_explore_url"},
		GrafanaExplorerURL: exploreURL,
	}

	return data, meta, nil
}

func (c *grafanaClient) queryPrometheus(ctx context.Context, query string, fromTime, toTime time.Time, step string) (*prometheusResponse, error) {
	proxyURL := fmt.Sprintf("%s/api/datasources/proxy/uid/%s/api/v1/query_range",
		strings.TrimSuffix(c.baseURL, "/"), c.datasourceUID)
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	q := u.Query()
	q.Set("query", query)
	q.Set("start", fromTime.Format(time.RFC3339))
	q.Set("end", toTime.Format(time.RFC3339))
	q.Set("step", step)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to Grafana failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("grafana API returned non-OK status: %s (body: %s)", resp.Status, string(b))
	}

	var promResp prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if promResp.Status == "error" {
		return nil, fmt.Errorf("prometheus error: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	return &promResp, nil
}

func SortMetricSummary(data []domain.MetricSummary, field string, reverse bool) {
	sort.Slice(data, func(i, j int) bool {
		var valI, valJ float64
		var strI, strJ string
		isStr := false

		switch field {
		case "legend":
			strI = data[i].Legend
			strJ = data[j].Legend
			isStr = true
		case "samples":
			valI = float64(data[i].Samples)
			valJ = float64(data[j].Samples)
		case "min":
			valI = data[i].Min
			valJ = data[j].Min
		case "p50":
			valI = data[i].P50
			valJ = data[j].P50
		case "p90":
			valI = data[i].P90
			valJ = data[j].P90
		case "p99":
			valI = data[i].P99
			valJ = data[j].P99
		case "max":
			valI = data[i].Max
			valJ = data[j].Max
		case "last":
			valI = data[i].Last
			valJ = data[j].Last
		default:
			valI = data[i].P99
			valJ = data[j].P99
		}

		var less bool
		if isStr {
			less = strI < strJ
		} else {
			less = valI < valJ
		}

		if reverse {
			return !less
		}
		return less
	})
}

// Ensure grafanaClient implements MetricRepository.
var _ domain.MetricRepository = (*grafanaClient)(nil)
