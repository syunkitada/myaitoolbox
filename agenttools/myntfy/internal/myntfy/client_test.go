package myntfy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/saved-topic" {
			t.Errorf("path = %s, want /saved-topic", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Title"); got != "Build completed" {
			t.Errorf("Title = %q", got)
		}
		if got := r.Header.Get("Priority"); got != "high" {
			t.Errorf("Priority = %q", got)
		}
		if got := r.Header.Get("Tags"); got != "white_check_mark,build" {
			t.Errorf("Tags = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll() error = %v", err)
		}
		if got := string(body); got != "message body" {
			t.Errorf("body = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Publish(context.Background(), "saved-topic", "message body", PublishOptions{
		Title: "Build completed", Priority: "high", Tags: "white_check_mark,build",
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":40001,"error":"invalid topic"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	err = client.Publish(context.Background(), "saved-topic", "message", PublishOptions{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "invalid topic") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestSubscribeCopiesJSONLines(t *testing.T) {
	const stream = `{"event":"open","topic":"saved-topic"}
{"event":"message","topic":"saved-topic","message":"hello"}
{"event":"keepalive","topic":"saved-topic"}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/saved-topic/json" {
			t.Errorf("path = %s, want /saved-topic/json", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/x-ndjson" {
			t.Errorf("Accept = %q", got)
		}
		_, _ = io.WriteString(w, stream)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	var output strings.Builder
	if err := client.Subscribe(context.Background(), "saved-topic", &output); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if got := output.String(); got != stream {
		t.Fatalf("Subscribe() output = %q, want %q", got, stream)
	}
}

func TestSubscribeReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"authentication required"}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	err = client.Subscribe(context.Background(), "saved-topic", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("Subscribe() error = %v", err)
	}
}

func TestResolveServerURL(t *testing.T) {
	t.Setenv("MYNTFY_SERVER", "https://env.example")
	if got := ResolveServerURL(""); got != "https://env.example" {
		t.Fatalf("ResolveServerURL(empty) = %q", got)
	}
	if got := ResolveServerURL("https://flag.example"); got != "https://flag.example" {
		t.Fatalf("ResolveServerURL(flag) = %q", got)
	}
}

func TestValidatePriority(t *testing.T) {
	for _, priority := range []string{"1", "5", "low", "urgent", ""} {
		if err := ValidatePriority(priority); err != nil {
			t.Errorf("ValidatePriority(%q) error = %v", priority, err)
		}
	}
	if err := ValidatePriority("critical"); err == nil {
		t.Error("ValidatePriority(critical) error = nil, want error")
	}
}
