package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type GrafanaClient struct {
	client        *http.Client
	baseURL       string
	apiToken      string
	datasourceUID string
}

func NewGrafanaClient(baseURL, apiToken, datasourceUID string) *GrafanaClient {
	return &GrafanaClient{
		client:        &http.Client{Timeout: 30 * time.Second},
		baseURL:       baseURL,
		apiToken:      apiToken,
		datasourceUID: datasourceUID,
	}
}

type PrometheusResponse struct {
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

type MetricSummary struct {
	Legend  string  `json:"legend"`
	Samples int     `json:"samples"`
	Min     float64 `json:"min"`
	P50     float64 `json:"p50"`
	P90     float64 `json:"p90"`
	P99     float64 `json:"p99"`
	Max     float64 `json:"max"`
	Last    float64 `json:"last"`
	MinAt   string  `json:"min_at"`
	MaxAt   string  `json:"max_at"`
}

type QueryMetricSummaryResponse struct {
	Meta struct {
		Query              string   `json:"query"`
		From               string   `json:"from"`
		To                 string   `json:"to"`
		Outputs            []string `json:"outputs"`
		GrafanaExplorerURL string   `json:"grafana_explore_url"`
	} `json:"meta"`
	Data []MetricSummary `json:"data"`
}

func parseFlexibleTime(timeStr string, baseTime time.Time, isFrom bool) (time.Time, error) {
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
		d, err := parseDuration(dStr)
		if err != nil {
			return time.Time{}, err
		}
		return baseTime.Add(-d), nil
	}

	if d, err := parseDuration(timeStr); err == nil {
		if d < 0 {
			return baseTime.Add(d), nil
		}
		return baseTime.Add(-d), nil
	}

	if strings.HasPrefix(timeStr, "-") {
		d, err := parseDuration(timeStr[1:])
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

func parseDuration(s string) (time.Duration, error) {
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

func expandVariables(query string, vars map[string]string) string {
	for k, v := range vars {
		query = strings.ReplaceAll(query, "$"+k, v)
		query = strings.ReplaceAll(query, "${"+k+"}", v)
		query = strings.ReplaceAll(query, "[["+k+"]]", v)
	}
	return query
}

var legendRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func formatLegend(template string, metric map[string]string) string {
	if template == "" {
		return formatMetricLabels(metric)
	}
	return legendRegex.ReplaceAllStringFunc(template, func(match string) string {
		labelName := legendRegex.FindStringSubmatch(match)[1]
		labelName = strings.TrimSpace(labelName)
		if val, ok := metric[labelName]; ok {
			return val
		}
		return ""
	})
}

func formatMetricLabels(metric map[string]string) string {
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

func (c *GrafanaClient) QueryMetricSummary(ctx context.Context, promQL string, vars map[string]string, legendTemplate string, timeFrom, timeTo string, sortField string, reverse bool, limit, offset int) (*QueryMetricSummaryResponse, error) {
	now := time.Now()
	toTime, err := parseFlexibleTime(timeTo, now, false)
	if err != nil {
		return nil, fmt.Errorf("invalid time_to: %w", err)
	}
	fromTime, err := parseFlexibleTime(timeFrom, toTime, true)
	if err != nil {
		return nil, fmt.Errorf("invalid time_from: %w", err)
	}

	if !toTime.After(fromTime) {
		return nil, fmt.Errorf("time_to must be after time_from")
	}

	expandedQL := expandVariables(promQL, vars)

	duration := toTime.Sub(fromTime)
	stepSeconds := int(duration.Seconds() / 300)
	if stepSeconds < 15 {
		stepSeconds = 15
	}
	step := fmt.Sprintf("%ds", stepSeconds)

	proxyURL := fmt.Sprintf("%s/api/datasources/proxy/uid/%s/api/v1/query_range", strings.TrimSuffix(c.baseURL, "/"), c.datasourceUID)
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	q := u.Query()
	q.Set("query", expandedQL)
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

	var promResp PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if promResp.Status == "error" {
		return nil, fmt.Errorf("prometheus error: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	var data []MetricSummary

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
				// Sometimes prometheus timestamp can be a float64 in scientific notation
				// or can be decoded into other types if parsed loosely. But standard JSON decodes it as float64.
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

		sortedVals := make([]float64, len(vals))
		copy(sortedVals, vals)
		sort.Float64s(sortedVals)

		p50 := sortedVals[int(math.Round(float64(len(sortedVals)-1)*0.5))]
		p90 := sortedVals[int(math.Round(float64(len(sortedVals)-1)*0.9))]
		p99 := sortedVals[int(math.Round(float64(len(sortedVals)-1)*0.99))]

		legend := formatLegend(legendTemplate, result.Metric)

		data = append(data, MetricSummary{
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
	sort.Slice(data, func(i, j int) bool {
		var valI, valJ float64
		var strI, strJ string
		isStr := false

		switch sortField {
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

	left := ExploreLeft{
		Datasource: c.datasourceUID,
		Queries: []ExploreQuery{
			{
				RefID: "A",
				Expr:  expandedQL,
				Datasource: ExploreDatasource{
					Type: "prometheus",
					UID:  c.datasourceUID,
				},
			},
		},
		Range: ExploreRange{
			From: fromTime.Format(time.RFC3339),
			To:   toTime.Format(time.RFC3339),
		},
	}

	leftJSON, err := json.Marshal(left)
	var exploreURL string
	if err == nil {
		baseURL := strings.TrimSuffix(c.baseURL, "/")
		exploreURL = fmt.Sprintf("%s/explore?left=%s", baseURL, url.QueryEscape(string(leftJSON)))
	}

	res := &QueryMetricSummaryResponse{}
	res.Meta.Query = expandedQL
	res.Meta.From = fromTime.Format(time.RFC3339)
	res.Meta.To = toTime.Format(time.RFC3339)
	res.Meta.Outputs = []string{"grafana_explore_url"}
	res.Meta.GrafanaExplorerURL = exploreURL
	res.Data = data

	return res, nil
}

type MapEntry struct {
	Key   string
	Value interface{}
}

type OrderedMap []MapEntry

func (om OrderedMap) MarshalJSON() ([]byte, error) {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, entry := range om {
		if i > 0 {
			sb.WriteByte(',')
		}
		kBytes, err := json.Marshal(entry.Key)
		if err != nil {
			return nil, err
		}
		sb.Write(kBytes)
		sb.WriteByte(':')
		vBytes, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		sb.Write(vBytes)
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

func (om OrderedMap) Get(key string) (interface{}, bool) {
	for _, entry := range om {
		if entry.Key == key {
			return entry.Value, true
		}
	}
	return nil, false
}

type QueryMetricHistoryMeta struct {
	Queries            []string `json:"queries"`
	Legend             string   `json:"legend,omitempty"`
	TimeFrom           string   `json:"time_from"`
	TimeTo             string   `json:"time_to"`
	Outputs            []string `json:"outputs"`
	GrafanaExplorerURL string   `json:"grafana_explore_url"`
}

type QueryMetricHistoryResponse struct {
	Meta QueryMetricHistoryMeta `json:"meta"`
	Data []OrderedMap           `json:"data"`
}

func formatTime(t time.Time, duration time.Duration, stepSeconds int) string {
	if duration > 24*time.Hour {
		return t.Format("2006-01-02 15:04")
	}
	if stepSeconds < 60 {
		return t.Format("15:04:05")
	}
	return t.Format("15:04")
}

func (c *GrafanaClient) QueryMetricHistory(ctx context.Context, promQLs []string, vars map[string]string, legendTemplate string, timeFrom, timeTo string) (*QueryMetricHistoryResponse, error) {
	now := time.Now()
	toTime, err := parseFlexibleTime(timeTo, now, false)
	if err != nil {
		return nil, fmt.Errorf("invalid time_to: %w", err)
	}
	fromTime, err := parseFlexibleTime(timeFrom, toTime, true)
	if err != nil {
		return nil, fmt.Errorf("invalid time_from: %w", err)
	}

	if !toTime.After(fromTime) {
		return nil, fmt.Errorf("time_to must be after time_from")
	}

	duration := toTime.Sub(fromTime)
	stepSeconds := int(duration.Seconds() / 60)
	if stepSeconds < 15 {
		stepSeconds = 15
	}
	step := fmt.Sprintf("%ds", stepSeconds)

	multiQuery := len(promQLs) > 1

	// singleMode: timeMap[ts][legend] = value
	// multiMode:  multiMap[legend][ts][queryName] = value
	timeMap := make(map[int64]map[string]float64)               // single-query mode
	multiMap := make(map[string]map[int64]map[string]float64)   // multi-query mode

	var expandedQueries []string

	for _, promQL := range promQLs {
		expandedQL := expandVariables(promQL, vars)
		expandedQueries = append(expandedQueries, expandedQL)

		proxyURL := fmt.Sprintf("%s/api/datasources/proxy/uid/%s/api/v1/query_range", strings.TrimSuffix(c.baseURL, "/"), c.datasourceUID)
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		q := u.Query()
		q.Set("query", expandedQL)
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

		var promResp PrometheusResponse
		if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if promResp.Status == "error" {
			return nil, fmt.Errorf("prometheus error: %s (%s)", promResp.Error, promResp.ErrorType)
		}

		if multiQuery {
			// Validate that all series resolve to exactly one legend value per query
			legendSet := make(map[string]struct{})
			for _, result := range promResp.Data.Result {
				legend := formatLegend(legendTemplate, result.Metric)
				legendSet[legend] = struct{}{}
			}
			// Collect all legends across queries into multiMap
			for _, result := range promResp.Data.Result {
				legend := formatLegend(legendTemplate, result.Metric)
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
				legend := formatLegend(legendTemplate, result.Metric)
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

	// Validate multi-query: all legends must resolve to a single value
	if multiQuery && len(multiMap) != 1 {
		var foundLegends []string
		for l := range multiMap {
			foundLegends = append(foundLegends, l)
		}
		sort.Strings(foundLegends)
		return nil, fmt.Errorf("multi-query requires legend to resolve to exactly one value, got %d: %v", len(multiMap), foundLegends)
	}

	var data []OrderedMap
	var resolvedLegend string

	if multiQuery {
		// Multi-query mode: one legend, columns = query names
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
			var item OrderedMap
			t := time.Unix(ts, 0).In(fromTime.Location())
			item = append(item, MapEntry{Key: "time", Value: formatTime(t, duration, stepSeconds)})
			for _, eq := range expandedQueries {
				item = append(item, MapEntry{Key: eq, Value: tsMap[ts][eq]})
			}
			data = append(data, item)
		}
	} else {
		// Single-query mode: columns = legend values
		var timestamps []int64
		for ts := range timeMap {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		for _, ts := range timestamps {
			var item OrderedMap
			t := time.Unix(ts, 0).In(fromTime.Location())
			item = append(item, MapEntry{Key: "time", Value: formatTime(t, duration, stepSeconds)})

			var legends []string
			for legend := range timeMap[ts] {
				legends = append(legends, legend)
			}
			sort.Strings(legends)

			for _, legend := range legends {
				item = append(item, MapEntry{Key: legend, Value: timeMap[ts][legend]})
			}
			data = append(data, item)
		}
	}

	// Construct Grafana Explorer URL
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
	for i, eq := range expandedQueries {
		refID := string(rune('A' + i))
		exploreQueries = append(exploreQueries, ExploreQuery{
			RefID: refID,
			Expr:  eq,
			Datasource: ExploreDatasource{
				Type: "prometheus",
				UID:  c.datasourceUID,
			},
		})
	}

	left := ExploreLeft{
		Datasource: c.datasourceUID,
		Queries:    exploreQueries,
		Range: ExploreRange{
			From: fromTime.Format(time.RFC3339),
			To:   toTime.Format(time.RFC3339),
		},
	}

	leftJSON, err := json.Marshal(left)
	var exploreURL string
	if err == nil {
		baseURL := strings.TrimSuffix(c.baseURL, "/")
		exploreURL = fmt.Sprintf("%s/explore?left=%s", baseURL, url.QueryEscape(string(leftJSON)))
	}

	res := &QueryMetricHistoryResponse{}
	res.Meta.Queries = expandedQueries
	res.Meta.Legend = resolvedLegend
	res.Meta.TimeFrom = fromTime.Format(time.RFC3339)
	res.Meta.TimeTo = toTime.Format(time.RFC3339)
	res.Meta.Outputs = []string{"grafana_explore_url"}
	res.Meta.GrafanaExplorerURL = exploreURL
	res.Data = data

	return res, nil
}

