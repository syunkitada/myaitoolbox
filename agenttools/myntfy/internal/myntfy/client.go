package myntfy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const DefaultServerURL = "https://ntfy.sh"

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type PublishOptions struct {
	Title    string
	Priority string
	Tags     string
}

func ResolveServerURL(flagValue string) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("MYNTFY_SERVER")); value != "" {
		return value
	}
	return DefaultServerURL
}

func ValidatePriority(priority string) error {
	if priority == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "1", "2", "3", "4", "5", "min", "low", "default", "high", "max", "urgent":
		return nil
	default:
		return fmt.Errorf("unsupported priority %q", priority)
	}
}

func NewClient(rawURL string, httpClient *http.Client) (*Client, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("ntfy server URL is empty")
	}
	baseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse ntfy server URL: %w", err)
	}
	if (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("ntfy server URL must be an http or https URL")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	baseURL.RawPath = ""
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *Client) Publish(ctx context.Context, topic, message string, options PublishOptions) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.topicURL(topic, ""), strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("create publish request: %w", err)
	}
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if options.Title != "" {
		request.Header.Set("Title", options.Title)
	}
	if options.Priority != "" {
		request.Header.Set("Priority", options.Priority)
	}
	if options.Tags != "" {
		request.Header.Set("Tags", options.Tags)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("publish notification: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if err := checkResponse(response); err != nil {
		return fmt.Errorf("publish notification: %w", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func (c *Client) Subscribe(ctx context.Context, topic string, output io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.topicURL(topic, "/json"), nil)
	if err != nil {
		return fmt.Errorf("create subscribe request: %w", err)
	}
	request.Header.Set("Accept", "application/x-ndjson")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("subscribe to topic: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if err := checkResponse(response); err != nil {
		return fmt.Errorf("subscribe to topic: %w", err)
	}
	if _, err := io.Copy(output, response.Body); err != nil {
		return fmt.Errorf("read subscription stream: %w", err)
	}
	return nil
}

func (c *Client) topicURL(topic, suffix string) string {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + topic + suffix
	endpoint.RawPath = ""
	return endpoint.String()
}

func checkResponse(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("server returned HTTP %d", response.StatusCode)
	}
	return fmt.Errorf("server returned HTTP %d: %s", response.StatusCode, detail)
}
