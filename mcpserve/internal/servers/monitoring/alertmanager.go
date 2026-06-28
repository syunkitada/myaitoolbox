package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultAMURL = "http://127.0.0.1:9093"

type AlertmanagerClient struct {
	client  *http.Client
	baseURL string
}

func NewAlertmanagerClient() *AlertmanagerClient {
	url := os.Getenv("ALERTMANAGER_URL")
	if url == "" {
		url = defaultAMURL
	}
	return &AlertmanagerClient{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: url,
	}
}

type AMAlertStatus struct {
	State string `json:"state"`
}

type AMAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      AMAlertStatus     `json:"status"`
}

type Silence struct {
	ID     string `json:"id"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

func (c *AlertmanagerClient) GetAlerts(filters ...string) ([]AMAlert, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v2/alerts", c.baseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for _, f := range filters {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	resp, err := c.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get alerts: %s", resp.Status)
	}

	var alerts []AMAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

func (c *AlertmanagerClient) GetSilences(filters ...string) ([]Silence, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v2/silences", c.baseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for _, f := range filters {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	resp, err := c.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get silences: %s", resp.Status)
	}

	var silences []Silence
	if err := json.NewDecoder(resp.Body).Decode(&silences); err != nil {
		return nil, err
	}
	return silences, nil
}

func (c *AlertmanagerClient) CreateSilence(silence Silence) (string, error) {
	body, err := json.Marshal(silence)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Post(fmt.Sprintf("%s/api/v2/silences", c.baseURL), "application/json", bytes.NewBuffer(body))
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

func (c *AlertmanagerClient) DeleteSilence(id string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v2/silence/%s", c.baseURL, id), nil)
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
