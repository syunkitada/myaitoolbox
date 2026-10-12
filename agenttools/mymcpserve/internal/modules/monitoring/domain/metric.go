package domain

import (
	"context"
	"encoding/json"
	"strings"
)

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

type SummaryMeta struct {
	Query              string   `json:"query"`
	From               string   `json:"from"`
	To                 string   `json:"to"`
	Outputs            []string `json:"outputs"`
	GrafanaExplorerURL string   `json:"grafana_explore_url"`
}

type HistoryMeta struct {
	Queries            []string `json:"queries"`
	Legend             string   `json:"legend,omitempty"`
	TimeFrom           string   `json:"time_from"`
	TimeTo             string   `json:"time_to"`
	Outputs            []string `json:"outputs"`
	GrafanaExplorerURL string   `json:"grafana_explore_url"`
}

type MetricRepository interface {
	QuerySummary(ctx context.Context, query string, vars map[string]string, legendTemplate string, timeFrom, timeTo string, sortField string, reverse bool, limit, offset int) ([]MetricSummary, SummaryMeta, error)
	QueryHistory(ctx context.Context, queries []string, vars map[string]string, legendTemplate string, timeFrom, timeTo string) ([]OrderedMap, HistoryMeta, error)
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

type ExploreQueryDef struct {
	Expr string
}
