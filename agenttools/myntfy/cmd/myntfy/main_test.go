package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunTopicLifecycle(t *testing.T) {
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "topic"))

	var output bytes.Buffer
	if err := run(context.Background(), []string{"topic", "regenerate", "--prefix", "server01"}, &output); err != nil {
		t.Fatalf("regenerate error = %v", err)
	}
	first := strings.TrimSpace(output.String())
	if ok, err := regexp.MatchString(`^server01-[a-f0-9]{32}$`, first); err != nil || !ok {
		t.Fatalf("generated topic has unexpected format: %q", first)
	}

	output.Reset()
	if err := run(context.Background(), []string{"topic", "show"}, &output); err != nil {
		t.Fatalf("show error = %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != first {
		t.Fatalf("show = %q, want %q", got, first)
	}

	output.Reset()
	if err := run(context.Background(), []string{"topic", "regenerate", "--prefix", "server01"}, &output); err != nil {
		t.Fatalf("second regenerate error = %v", err)
	}
	if second := strings.TrimSpace(output.String()); second == first {
		t.Fatalf("regenerate kept the same topic %q", second)
	}
}

func TestRunSendUsesStoredTopicAndOptions(t *testing.T) {
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "topic"))
	var topicOutput bytes.Buffer
	if err := run(context.Background(), []string{"topic", "regenerate", "--prefix", "server01"}, &topicOutput); err != nil {
		t.Fatalf("regenerate error = %v", err)
	}
	topic := strings.TrimSpace(topicOutput.String())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+topic {
			t.Errorf("path = %q, want %q", r.URL.Path, "/"+topic)
		}
		if got := r.Header.Get("Title"); got != "Deploy finished" {
			t.Errorf("Title = %q", got)
		}
		if got := r.Header.Get("Tags"); got != "tada,deploy" {
			t.Errorf("Tags = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("MYNTFY_SERVER", server.URL)

	var output bytes.Buffer
	if err := run(context.Background(), []string{"send", "--title", "Deploy finished", "--tags", "tada,deploy", "message"}, &output); err != nil {
		t.Fatalf("send error = %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != "sent" {
		t.Fatalf("send output = %q, want sent", got)
	}
}

func TestRunSubscribeStreamsEvents(t *testing.T) {
	const stream = `{"event":"open"}
{"event":"message","message":"hello"}
`
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "topic"))
	var topicOutput bytes.Buffer
	if err := run(context.Background(), []string{"topic", "regenerate", "--prefix", "server01"}, &topicOutput); err != nil {
		t.Fatalf("regenerate error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/json") {
			t.Errorf("path = %q, want JSON endpoint", r.URL.Path)
		}
		_, _ = w.Write([]byte(stream))
	}))
	defer server.Close()
	t.Setenv("MYNTFY_SERVER", server.URL)

	var output bytes.Buffer
	if err := run(context.Background(), []string{"subscribe"}, &output); err != nil {
		t.Fatalf("subscribe error = %v", err)
	}
	if got := output.String(); got != stream {
		t.Fatalf("subscribe output = %q, want %q", got, stream)
	}
}

func TestRunTopicRegenerateRequiresPrefix(t *testing.T) {
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "topic"))
	var output bytes.Buffer
	err := run(context.Background(), []string{"topic", "regenerate"}, &output)
	if err == nil || !strings.Contains(err.Error(), "required flag(s) \"prefix\"") {
		t.Fatalf("regenerate error = %v, want required prefix error", err)
	}
}

func TestRunRegenerateConfirmsAndReplacesInvalidPrefix(t *testing.T) {
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "topic"))
	var output bytes.Buffer
	err := runWithInput(
		context.Background(),
		[]string{"topic", "regenerate", "--prefix", "server.example"},
		strings.NewReader("yes\n"),
		&output,
	)
	if err != nil {
		t.Fatalf("regenerate error = %v", err)
	}
	if !strings.Contains(output.String(), `replace with "server-example"`) {
		t.Fatalf("confirmation prompt = %q", output.String())
	}
	if ok, err := regexp.MatchString(`server-example-[a-f0-9]{32}`, output.String()); err != nil || !ok {
		t.Fatalf("generated topic = %q", output.String())
	}
}

func TestRunRegenerateDeclinesInvalidPrefixReplacement(t *testing.T) {
	topicPath := filepath.Join(t.TempDir(), "topic")
	t.Setenv("MYNTFY_CONFIG", topicPath)
	var output bytes.Buffer
	err := runWithInput(
		context.Background(),
		[]string{"topic", "regenerate", "--prefix", "server.example"},
		strings.NewReader("n\n"),
		&output,
	)
	if err == nil || !strings.Contains(err.Error(), "prefix replacement canceled") {
		t.Fatalf("regenerate error = %v, want cancellation", err)
	}
	if _, statErr := os.Stat(topicPath); !os.IsNotExist(statErr) {
		t.Fatalf("topic file exists after cancellation: stat error = %v", statErr)
	}
}

func TestRunRequiresConfiguredTopic(t *testing.T) {
	t.Setenv("MYNTFY_CONFIG", filepath.Join(t.TempDir(), "missing-topic"))
	var output bytes.Buffer
	err := run(context.Background(), []string{"send", "message"}, &output)
	if err == nil || !strings.Contains(err.Error(), "myntfy topic regenerate --prefix <prefix>") {
		t.Fatalf("send error = %v, want regenerate guidance", err)
	}
}
