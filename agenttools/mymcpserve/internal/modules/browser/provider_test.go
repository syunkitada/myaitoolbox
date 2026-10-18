package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestValidateBaseURLRequiresTLSForRemoteMybox(t *testing.T) {
	for _, raw := range []string{
		"http://mybox.example",
		"http://192.0.2.10:8080",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := newClient(raw, "agent-secret")
			if err == nil || !strings.Contains(err.Error(), "HTTPS") {
				t.Fatalf("expected remote HTTP URL to be rejected, got %v", err)
			}
		})
	}
}

func TestValidateBaseURLAllowsLoopbackHTTPAndRemoteHTTPS(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"https://mybox.example",
	} {
		t.Run(raw, func(t *testing.T) {
			client, err := newClient(raw, "agent-secret")
			if err != nil {
				t.Fatalf("expected URL to be accepted: %v", err)
			}
			if client.configErr != nil {
				t.Fatalf("expected valid client configuration: %v", client.configErr)
			}
		})
	}
}

func TestValidateBaseURLRequiresTokenForRemoteHTTPS(t *testing.T) {
	_, err := newClient("https://mybox.example", "")
	if err == nil || !strings.Contains(err.Error(), "agent token") {
		t.Fatalf("expected remote HTTPS URL without a token to be rejected, got %v", err)
	}
}

func TestValidateBaseURLRejectsCredentialsAndQuery(t *testing.T) {
	for _, raw := range []string{
		"https://user:pass@mybox.example",
		"https://mybox.example?token=secret",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := newClient(raw, "agent-secret")
			if err == nil {
				t.Fatal("expected URL to be rejected")
			}
		})
	}
}

func TestClientScopesAgentRequestToProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/browser/sessions/session-1/commands" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("project") != "project-a" {
			t.Errorf("expected project-a scope, got %q", r.URL.Query().Get("project"))
		}
		if r.Header.Get("X-Browser-Actor") != "agent" {
			t.Errorf("expected agent actor, got %q", r.Header.Get("X-Browser-Actor"))
		}
		if r.Header.Get("X-Browser-Agent-Token") != "agent-secret" {
			t.Errorf("expected configured agent token")
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["command"] != "observe" {
			t.Errorf("expected observe command, got %#v", body["command"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := newClient(server.URL, "agent-secret")
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	_, _, err = client.do(context.Background(), http.MethodPost, "/api/browser/sessions/session-1/commands?project=project-a", map[string]interface{}{"command": "observe"}, true)
	if err != nil {
		t.Fatalf("client.do: %v", err)
	}
}

func TestBrowserStartDoesNotAcceptAnInitialURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["project"] != "project-a" {
			t.Errorf("expected project-a, got %#v", body["project"])
		}
		if _, exists := body["url"]; exists {
			t.Errorf("browser_start_session must not send an unapproved initial URL")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"session-1","project":"project-a","state":"running"}`))
	}))
	defer server.Close()

	client, err := newClient(server.URL, "agent-secret")
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	request := &mcp.CallToolRequest{}
	request.Params = &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"project":"project-a","url":"https://example.com"}`)}
	_, _, err = client.start(context.Background(), request)
	if err != nil {
		t.Fatalf("client.start: %v", err)
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://redirect.example/", http.StatusFound)
	}))
	defer server.Close()

	client, err := newClient(server.URL, "agent-secret")
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	_, _, err = client.do(context.Background(), http.MethodGet, "/api/browser/sessions?project=project-a", nil, true)
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("expected redirect response to be returned as an API error, got %v", err)
	}
}
