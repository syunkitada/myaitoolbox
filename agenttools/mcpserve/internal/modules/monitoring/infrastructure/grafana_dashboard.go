package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/modules/monitoring/domain"
)

type grafanaDashboardClient struct {
	client   *http.Client
	baseURL  string
	apiToken string
}

func NewGrafanaDashboardClient(baseURL, apiToken string) domain.DashboardRepository {
	return &grafanaDashboardClient{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  baseURL,
		apiToken: apiToken,
	}
}

func (c *grafanaDashboardClient) do(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	u := strings.TrimSuffix(c.baseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request to Grafana failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("grafana API returned non-OK status: %s (body: %s)", resp.Status, string(b))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

func (c *grafanaDashboardClient) SearchDashboards(ctx context.Context, query, folderUID string, tags []string, limit int) ([]domain.DashboardSummary, error) {
	q := url.Values{}
	q.Set("type", "dash-db")
	if query != "" {
		q.Set("query", query)
	}
	if folderUID != "" {
		q.Set("folderUIDs", folderUID)
	}
	for _, tag := range tags {
		q.Add("tag", tag)
	}
	if limit <= 0 {
		limit = 50
	}
	q.Set("limit", strconv.Itoa(limit))

	var summaries []domain.DashboardSummary
	if err := c.do(ctx, http.MethodGet, "/api/search", q, nil, &summaries); err != nil {
		return nil, err
	}

	if folderUID != "" {
		filtered := summaries[:0]
		for _, s := range summaries {
			if s.FolderUID == folderUID {
				filtered = append(filtered, s)
			}
		}
		summaries = filtered
	}
	if len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

func (c *grafanaDashboardClient) GetDashboard(ctx context.Context, uid string) (domain.Dashboard, error) {
	var result domain.Dashboard
	err := c.do(ctx, http.MethodGet, "/api/dashboards/uid/"+url.PathEscape(uid), nil, nil, &result)
	return result, err
}

func (c *grafanaDashboardClient) SaveDashboard(ctx context.Context, model map[string]interface{}, folderUID string, overwrite bool, message string) (domain.DashboardSaveResult, error) {
	payload := map[string]interface{}{
		"dashboard": model,
		"overwrite": overwrite,
	}
	if folderUID != "" {
		payload["folderUid"] = folderUID
	}
	if message != "" {
		payload["message"] = message
	}

	var result domain.DashboardSaveResult
	err := c.do(ctx, http.MethodPost, "/api/dashboards/db", nil, payload, &result)
	return result, err
}

func (c *grafanaDashboardClient) DeleteDashboard(ctx context.Context, uid string) (domain.DashboardDeleteResult, error) {
	var result domain.DashboardDeleteResult
	err := c.do(ctx, http.MethodDelete, "/api/dashboards/uid/"+url.PathEscape(uid), nil, nil, &result)
	return result, err
}

func (c *grafanaDashboardClient) ListFolders(ctx context.Context) ([]domain.Folder, error) {
	var folders []domain.Folder
	err := c.do(ctx, http.MethodGet, "/api/folders", nil, nil, &folders)
	return folders, err
}

// Ensure grafanaDashboardClient implements DashboardRepository.
var _ domain.DashboardRepository = (*grafanaDashboardClient)(nil)
