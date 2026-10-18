package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/syunkitada/myaitoolbox/mymcpserve/internal/domain"
)

type provider struct{}

type client struct {
	baseURL    *url.URL
	http       *http.Client
	agentToken string
	configErr  error
}

func New() domain.Provider { return &provider{} }

func (p *provider) Name() string { return "browser" }

func (p *provider) Description() string {
	return "Project-scoped Playwright browser control through a running mybox instance."
}

func (p *provider) RegisterTools(server domain.Server) {
	baseURL := strings.TrimRight(os.Getenv("MYBOX_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	c := configuredClient(baseURL, strings.TrimSpace(os.Getenv("MYBOX_BROWSER_AGENT_TOKEN")))

	server.AddTool(tool("browser_start_session", "Start or reuse the persistent browser session at about:blank; navigation needs user approval unless the user enabled auto-safe approval in mybox", schema(
		map[string]interface{}{"project": stringProperty("mybox project name")}, "project")), c.start)
	server.AddTool(tool("browser_attach_session", "Verify and attach to an existing project browser session", schema(
		map[string]interface{}{
			"project":    stringProperty("mybox project name"),
			"session_id": stringProperty("explicit browser session id"),
		}, "project", "session_id")), c.attach)
	server.AddTool(tool("browser_observe", "Read the current page accessibility snapshot and semantic reference ids", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id")}, "project", "session_id")), c.command("observe"))
	server.AddTool(tool("browser_navigate", "Navigate to a public HTTPS URL; requires user approval unless the user enabled auto-safe approval in mybox", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "url": stringProperty("public HTTPS URL")}, "project", "session_id", "url")), c.command("navigate"))
	server.AddTool(tool("browser_click", "Click a semantic reference from the latest browser observation; high-impact actions require approval, while recognized low-risk links, menus, and disclosure controls may run automatically if the user enabled auto-safe approval in mybox", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "observation_id": stringProperty("observation id from browser_observe"), "ref": stringProperty("semantic reference id")}, "project", "session_id", "observation_id", "ref")), c.command("click"))
	server.AddTool(tool("browser_fill", "Fill a non-sensitive field using a semantic reference", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "observation_id": stringProperty("observation id from browser_observe"), "ref": stringProperty("semantic reference id"), "value": stringProperty("field value")}, "project", "session_id", "observation_id", "ref", "value")), c.command("fill"))
	server.AddTool(tool("browser_press", "Press a key on a semantic reference; Enter on a submit-like control may require approval", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "observation_id": stringProperty("observation id from browser_observe"), "ref": stringProperty("semantic reference id"), "key": stringProperty("Playwright key name")}, "project", "session_id", "observation_id", "ref", "key")), c.command("press"))
	server.AddTool(tool("browser_scroll", "Scroll the current browser page", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "delta_y": numberProperty("vertical scroll delta"), "delta_x": numberProperty("horizontal scroll delta")}, "project", "session_id", "delta_y")), c.command("scroll"))
	server.AddTool(tool("browser_wait", "Wait briefly for the current page to settle", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id"), "ms": numberProperty("milliseconds, capped by mybox")}, "project", "session_id")), c.command("wait"))
	server.AddTool(tool("browser_screenshot", "Capture the current browser page as a JPEG", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id")}, "project", "session_id")), c.command("screenshot"))
	server.AddTool(tool("browser_close_session", "Close the browser session while preserving its persistent profile", schema(
		map[string]interface{}{"project": stringProperty("mybox project name"), "session_id": stringProperty("explicit browser session id")}, "project", "session_id")), c.command("close_session"))
}

func configuredClient(rawBaseURL, agentToken string) *client {
	c, err := newClient(rawBaseURL, agentToken)
	if err != nil {
		return &client{configErr: err}
	}
	return c
}

func newClient(rawBaseURL, agentToken string) (*client, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("MYBOX_URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("MYBOX_URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("MYBOX_URL must not contain credentials, query, or fragment")
	}
	loopback := isLoopbackHost(parsed.Hostname())
	if parsed.Scheme == "http" && !loopback {
		return nil, fmt.Errorf("remote MYBOX_URL must use HTTPS")
	}
	if !loopback && strings.TrimSpace(agentToken) == "" {
		return nil, fmt.Errorf("remote MYBOX_URL requires an agent token")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return &client{
		baseURL: parsed,
		http: &http.Client{
			Timeout: 35 * time.Second,
			// The browser API should never follow a redirect to another host. In
			// particular, do not risk forwarding the agent token to a redirect target.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		agentToken: strings.TrimSpace(agentToken),
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func tool(name, description string, schema map[string]interface{}) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: description, InputSchema: schema}
}

func schema(properties map[string]interface{}, required ...string) map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": properties, "required": required}
}

func stringProperty(description string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": description}
}

func numberProperty(description string) map[string]interface{} {
	return map[string]interface{}{"type": "number", "description": description}
}

func (c *client) start(ctx context.Context, req *mcp.CallToolRequest) (interface{}, interface{}, error) {
	args, err := arguments(req)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]interface{}{"project": requiredString(args, "project")}
	return c.do(ctx, http.MethodPost, "/api/browser/sessions", body, true)
}

func (c *client) attach(ctx context.Context, req *mcp.CallToolRequest) (interface{}, interface{}, error) {
	args, err := arguments(req)
	if err != nil {
		return nil, nil, err
	}
	project := requiredString(args, "project")
	sessionID := requiredString(args, "session_id")
	data, _, err := c.do(ctx, http.MethodGet, "/api/browser/sessions?project="+url.QueryEscape(project), nil, true)
	if err != nil {
		return nil, nil, err
	}
	result, ok := data.(map[string]interface{})
	if !ok || result["session"] == nil {
		return nil, nil, fmt.Errorf("browser session %q does not exist for project %q", sessionID, project)
	}
	session, ok := result["session"].(map[string]interface{})
	if !ok || session["id"] != sessionID {
		return nil, nil, fmt.Errorf("browser session %q does not match the project session", sessionID)
	}
	return session, nil, nil
}

func (c *client) command(commandName string) func(context.Context, *mcp.CallToolRequest) (interface{}, interface{}, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (interface{}, interface{}, error) {
		args, err := arguments(req)
		if err != nil {
			return nil, nil, err
		}
		project := requiredString(args, "project")
		sessionID := requiredString(args, "session_id")
		payload := map[string]interface{}{}
		for key, value := range args {
			if key != "project" && key != "session_id" {
				payload[key] = value
			}
		}
		path := "/api/browser/sessions/" + url.PathEscape(sessionID) + "/commands?project=" + url.QueryEscape(project)
		return c.do(ctx, http.MethodPost, path, map[string]interface{}{"command": commandName, "payload": payload}, true)
	}
}

func (c *client) do(ctx context.Context, method, path string, body interface{}, agent bool) (interface{}, interface{}, error) {
	if c.configErr != nil {
		return nil, nil, c.configErr
	}
	endpoint, err := c.endpoint(path)
	if err != nil {
		return nil, nil, err
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if agent {
		request.Header.Set("X-Browser-Actor", "agent")
		if c.agentToken != "" {
			request.Header.Set("X-Browser-Agent-Token", c.agentToken)
		}
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("mybox browser API: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, nil, err
	}
	var value interface{}
	if len(data) > 0 && json.Unmarshal(data, &value) != nil {
		value = string(data)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope, ok := value.(map[string]interface{}); ok && envelope["error"] != nil {
			return nil, nil, fmt.Errorf("mybox browser API: %v", envelope["error"])
		}
		return nil, nil, fmt.Errorf("mybox browser API returned HTTP %d", response.StatusCode)
	}
	return value, nil, nil
}

func (c *client) endpoint(path string) (string, error) {
	relative, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(relative.Path, "/") {
		return "", fmt.Errorf("invalid mybox browser API path")
	}
	target := *c.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + relative.Path
	target.RawQuery = relative.RawQuery
	target.Fragment = ""
	return target.String(), nil
}

func arguments(req *mcp.CallToolRequest) (map[string]interface{}, error) {
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return map[string]interface{}{}, nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid browser tool arguments: %w", err)
	}
	return args, nil
}

func requiredString(args map[string]interface{}, key string) string {
	return strings.TrimSpace(optionalString(args, key))
}

func optionalString(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return value
}
