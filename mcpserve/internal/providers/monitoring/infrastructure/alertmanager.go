package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/syunkitada/myaitoolbox/mcpserve/internal/providers/monitoring/domain"
)

type alertmanagerClient struct {
	client  *http.Client
	baseURL string
}

func NewAlertmanagerClient(baseURL string) domain.AlertRepository {
	return &alertmanagerClient{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
	}
}

type amAlertDTO struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      struct {
		State string `json:"state"`
	} `json:"status"`
}

type silenceDTO struct {
	ID     string `json:"id"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Matchers  []domain.Matcher `json:"matchers"`
	StartsAt  time.Time        `json:"startsAt"`
	EndsAt    time.Time        `json:"endsAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	CreatedBy string           `json:"createdBy"`
	Comment   string           `json:"comment"`
}

func (c *alertmanagerClient) GetAlerts(ctx context.Context, filters ...string) ([]domain.Alert, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v2/alerts", c.baseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for _, f := range filters {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get alerts: %s", resp.Status)
	}

	var dtos []amAlertDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, err
	}

	alerts := make([]domain.Alert, len(dtos))
	for i, dto := range dtos {
		alerts[i] = domain.Alert{
			Labels:      dto.Labels,
			Annotations: dto.Annotations,
			Status:      domain.AlertStatus(dto.Status.State),
		}
	}
	return alerts, nil
}

func (c *alertmanagerClient) List(ctx context.Context, filters ...string) ([]domain.Silence, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v2/silences", c.baseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for _, f := range filters {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get silences: %s", resp.Status)
	}

	var dtos []silenceDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, err
	}

	silences := make([]domain.Silence, len(dtos))
	for i, dto := range dtos {
		silences[i] = domain.Silence{
			ID:        dto.ID,
			Matchers:  dto.Matchers,
			StartsAt:  dto.StartsAt,
			EndsAt:    dto.EndsAt,
			UpdatedAt: dto.UpdatedAt,
			CreatedBy: dto.CreatedBy,
			Comment:   dto.Comment,
		}
	}
	return silences, nil
}

func (c *alertmanagerClient) Create(ctx context.Context, silence domain.Silence) (string, error) {
	body, err := json.Marshal(silence)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/api/v2/silences", c.baseURL),
		bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create silence: %s - %s", resp.Status, string(b))
	}

	var result struct {
		SilenceID string `json:"silenceID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.SilenceID, nil
}

func (c *alertmanagerClient) Delete(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/api/v2/silence/%s", c.baseURL, id), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete silence: %s - %s", resp.Status, string(b))
	}
	return nil
}

// Ensure alertmanagerClient implements both interfaces.
var _ domain.AlertRepository = (*alertmanagerClient)(nil)
var _ domain.SilenceRepository = (*alertmanagerClient)(nil)
