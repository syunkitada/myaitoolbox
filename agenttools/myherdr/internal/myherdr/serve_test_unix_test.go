//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package myherdr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServeHandlerEnforcesHostAndOriginAndServesAssets(t *testing.T) {
	app, baseURL, server, listener := startTestWebServer(t, t.TempDir())
	defer stopTestWebServer(t, app, server, listener)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}

	request, err := http.NewRequest(http.MethodGet, baseURL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "attacker.example"
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("wrong Host status = %d, want %d", response.StatusCode, http.StatusMisdirectedRequest)
	}

	request, err = http.NewRequest(http.MethodGet, baseURL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "http://attacker.example")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong Origin status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}

	response, err = client.Get(baseURL + "/assets/vendor/xterm.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("embedded xterm asset status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestServePublishesURLStateAndRemovesItOnShutdown(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if _, err := FindServeInstance(); err == nil {
		t.Fatal("FindServeInstance() succeeded before serve started")
	}
	var stoppedURL strings.Builder
	if err := Run(context.Background(), []string{"url", "codex-project"}, strings.NewReader(""), &stoppedURL, &strings.Builder{}); err == nil {
		t.Fatal("url command succeeded before serve started")
	} else if !strings.Contains(err.Error(), "myherdr serve") {
		t.Fatalf("stopped url error = %q, want serve startup instructions", err)
	}
	if stoppedURL.Len() != 0 {
		t.Fatalf("url command printed an invalid URL while serve was stopped: %q", stoppedURL.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output strings.Builder
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(ctx, Project{Root: t.TempDir()}, 0, &output)
	}()

	var state serveState
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if current, err := FindServeInstance(); err == nil {
			state = current
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state.Port == 0 {
		cancel()
		<-serveDone
		t.Fatal("serve did not publish a live instance")
	}

	var urlOutput strings.Builder
	if err := Run(ctx, []string{"url", "codex-project", "--pane", "w1:p1"}, strings.NewReader(""), &urlOutput, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	wantURL, err := SessionURL(state.Port, "codex-project", "w1:p1")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(urlOutput.String()); got != wantURL {
		t.Fatalf("url output = %q, want %q", got, wantURL)
	}

	cancel()
	if err := <-serveDone; err != nil {
		t.Fatalf("Serve() after cancellation: %v", err)
	}
	if !strings.Contains(output.String(), fmt.Sprintf("http://127.0.0.1:%d/", state.Port)) {
		t.Fatalf("serve output does not include its actual port: %q", output.String())
	}
	if _, err := FindServeInstance(); err == nil {
		t.Fatal("FindServeInstance() succeeded after serve shut down")
	}
}

func TestBrowserTerminalUsesFakeHerdrForPaneAndInputStreams(t *testing.T) {
	binDir := t.TempDir()
	capturePath := filepath.Join(t.TempDir(), "herdr-calls")
	inputPath := filepath.Join(t.TempDir(), "herdr-input")
	writeFakeExecutable(t, binDir, "herdr", `
args="$*"
printf 'session=%s args=%s\n' "${HERDR_SESSION-}" "$args" >> "$MYHERDR_TEST_CAPTURE"
case " $args " in
  *" workspace list "*) printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1"}]}}' ;;
  *" pane list "*) printf '%s\n' '{"result":{"panes":[{"pane_id":"w1:p1","label":"Codex pane","agent":"codex","cwd":"/tmp/project"}]}}' ;;
  *"terminal session control"*)
    printf '%s\n' '{"type":"terminal.frame","bytes":"aGVsbG8="}'
    while IFS= read -r line; do printf '%s\n' "$line" >> "$MYHERDR_TEST_INPUT"; done
    ;;
  *) exit 2 ;;
esac
`)
	prependTestPath(t, binDir)
	t.Setenv("MYHERDR_TEST_CAPTURE", capturePath)
	t.Setenv("MYHERDR_TEST_INPUT", inputPath)
	app, baseURL, server, listener := startTestWebServer(t, t.TempDir())
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			stopTestWebServer(t, app, server, listener)
		}
	})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get(baseURL + "/api/sessions/codex-project/panes")
	if err != nil {
		t.Fatal(err)
	}
	var paneResponse paneListResponse
	if err := json.NewDecoder(response.Body).Decode(&paneResponse); err != nil {
		_ = response.Body.Close()
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || paneResponse.Recommended != "w1:p1" || len(paneResponse.Panes) != 1 {
		t.Fatalf("pane response = %#v, status %d", paneResponse, response.StatusCode)
	}

	wsURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	wsURL.Scheme = "ws"
	wsURL.Path = "/ws"
	query := wsURL.Query()
	query.Set("session", "codex-project")
	query.Set("pane", "w1:p1")
	query.Set("mode", "control")
	wsURL.RawQuery = query.Encode()
	header := http.Header{"Origin": []string{baseURL}}
	socket, handshake, err := websocket.DefaultDialer.Dial(wsURL.String(), header)
	if err != nil {
		if handshake != nil {
			_ = handshake.Body.Close()
		}
		t.Fatal(err)
	}
	if err := socket.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	messageType, frame, err := socket.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.BinaryMessage || string(frame) != "hello" {
		t.Fatalf("terminal frame = (%d, %q), want binary hello", messageType, frame)
	}
	if err := socket.WriteJSON(map[string]any{"type": "resize", "cols": 120, "rows": 40}); err != nil {
		t.Fatal(err)
	}
	waitForFileContent(t, inputPath, `"type":"terminal.resize","cols":120,"rows":40`)
	if err := socket.WriteJSON(map[string]string{"type": "input", "text": "ls\r"}); err != nil {
		t.Fatal(err)
	}
	waitForFileContent(t, inputPath, `"text":"ls\r"`)
	if err := socket.WriteJSON(map[string]string{"type": "input", "text": "\x00"}); err != nil {
		t.Fatal(err)
	}
	waitForFileContent(t, inputPath, `"text":"\u0000"`)
	if err := socket.WriteJSON(map[string]string{"type": "input", "text": "still-connected\r"}); err != nil {
		t.Fatalf("terminal disconnected after NUL input: %v", err)
	}
	waitForFileContent(t, inputPath, `"text":"still-connected\r"`)
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	stopTestWebServer(t, app, server, listener)
	stopped = true

	callData, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(callData), "session=codex-project args=terminal session control w1:p1") {
		t.Fatalf("fake Herdr calls did not include session and pane: %q", callData)
	}
	if strings.Contains(string(callData), "pane create") || strings.Contains(string(callData), "pane delete") {
		t.Fatalf("browser terminal unexpectedly created or deleted a pane: %q", callData)
	}
}

func TestServeHandlerRejectsInvalidSessionAndPaneIDs(t *testing.T) {
	binDir := t.TempDir()
	prependTestPath(t, binDir)
	app, baseURL, server, listener := startTestWebServer(t, t.TempDir())
	defer stopTestWebServer(t, app, server, listener)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get(baseURL + "/api/sessions/bad%24name/panes")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("invalid session status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}

	for _, test := range []struct {
		name string
		pane string
		want int
	}{
		{name: "bad$name", pane: "w1:p1", want: http.StatusBadRequest},
		{name: "codex-project", pane: "w1:p1;touch", want: http.StatusBadRequest},
	} {
		wsURL, err := url.Parse(baseURL)
		if err != nil {
			t.Fatal(err)
		}
		wsURL.Scheme = "ws"
		wsURL.Path = "/ws"
		query := wsURL.Query()
		query.Set("session", test.name)
		query.Set("pane", test.pane)
		wsURL.RawQuery = query.Encode()
		_, response, err := websocket.DefaultDialer.Dial(wsURL.String(), http.Header{"Origin": []string{baseURL}})
		if err == nil {
			t.Fatalf("WebSocket target (%q, %q) unexpectedly connected", test.name, test.pane)
		}
		if response == nil || response.StatusCode != test.want {
			t.Fatalf("WebSocket target (%q, %q) response = %#v, want HTTP %d", test.name, test.pane, response, test.want)
		}
	}
}

func TestRemoteBrowserTerminalUsesFakeSSHForFramesAndInput(t *testing.T) {
	binDir := t.TempDir()
	capturePath := filepath.Join(t.TempDir(), "ssh-calls")
	inputPath := filepath.Join(t.TempDir(), "ssh-input")
	writeFakeExecutable(t, binDir, "ssh", `
args="$*"
printf '%s\n' "$args" >> "$MYHERDR_TEST_CAPTURE"
case " $args " in
  *" workspace list "*) printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1"}]}}' ;;
  *" pane list "*) printf '%s\n' '{"result":{"panes":[{"pane_id":"w1:p1","label":"Remote Codex pane","agent":"codex","cwd":"/srv/project"}]}}' ;;
  *"terminal session control"*)
    printf '%s\n' '{"type":"terminal.frame","bytes":"cmVtb3Rl"}'
    while IFS= read -r line; do printf '%s\n' "$line" >> "$MYHERDR_TEST_INPUT"; done
    ;;
  *) exit 2 ;;
esac
`)
	prependTestPath(t, binDir)
	t.Setenv("MYHERDR_TEST_CAPTURE", capturePath)
	t.Setenv("MYHERDR_TEST_INPUT", inputPath)
	projectRoot := t.TempDir()
	configDir := filepath.Join(projectRoot, ".myherdr")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "sessions.toml"), []byte("[sessions.remote-codex]\nhost = \"devbox\"\nherdr_session = \"codex-remote\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "ssh-hosts.md"), []byte("# allowed\n- `devbox`\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app, baseURL, server, listener := startTestWebServer(t, projectRoot)
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			stopTestWebServer(t, app, server, listener)
		}
	})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get(baseURL + "/api/sessions/remote-codex/panes")
	if err != nil {
		t.Fatal(err)
	}
	var paneResponse paneListResponse
	if err := json.NewDecoder(response.Body).Decode(&paneResponse); err != nil {
		_ = response.Body.Close()
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || paneResponse.Recommended != "w1:p1" {
		t.Fatalf("remote pane response = %#v, status %d", paneResponse, response.StatusCode)
	}

	wsURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	wsURL.Scheme = "ws"
	wsURL.Path = "/ws"
	query := wsURL.Query()
	query.Set("session", "remote-codex")
	query.Set("pane", "w1:p1")
	query.Set("mode", "control")
	wsURL.RawQuery = query.Encode()
	socket, handshake, err := websocket.DefaultDialer.Dial(wsURL.String(), http.Header{"Origin": []string{baseURL}})
	if err != nil {
		if handshake != nil {
			_ = handshake.Body.Close()
		}
		t.Fatal(err)
	}
	if err := socket.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	messageType, frame, err := socket.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.BinaryMessage || string(frame) != "remote" {
		t.Fatalf("remote terminal frame = (%d, %q), want binary remote", messageType, frame)
	}
	if err := socket.WriteJSON(map[string]string{"type": "input", "text": "whoami\r"}); err != nil {
		t.Fatal(err)
	}
	waitForFileContent(t, inputPath, `"type":"terminal.input"`)
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	stopTestWebServer(t, app, server, listener)
	stopped = true

	callData, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"-- devbox env HERDR_SESSION=codex-remote herdr workspace list",
		"-- devbox env HERDR_SESSION=codex-remote herdr terminal session control w1:p1 --cols 100 --rows 30",
	} {
		if !strings.Contains(string(callData), expected) {
			t.Errorf("fake SSH did not receive %q; calls were %q", expected, callData)
		}
	}
}

func TestWebSocketRejectsForeignOrigin(t *testing.T) {
	app, baseURL, server, listener := startTestWebServer(t, t.TempDir())
	defer stopTestWebServer(t, app, server, listener)
	wsURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	wsURL.Scheme = "ws"
	wsURL.Path = "/ws"
	query := wsURL.Query()
	query.Set("session", "codex-project")
	query.Set("pane", "w1:p1")
	wsURL.RawQuery = query.Encode()
	_, response, err := websocket.DefaultDialer.Dial(wsURL.String(), http.Header{"Origin": []string{"http://foreign.example"}})
	if err == nil {
		t.Fatal("expected foreign WebSocket Origin to be rejected")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("WebSocket rejection response = %#v, want HTTP 403", response)
	}
}

func startTestWebServer(t *testing.T, projectRoot string) (*serveApplication, string, *http.Server, net.Listener) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	app := &serveApplication{
		project:     Project{Root: projectRoot},
		port:        port,
		token:       "test-token",
		controllers: make(map[string]struct{}),
		connections: make(map[*websocket.Conn]struct{}),
	}
	server := &http.Server{Handler: app.handler()}
	go func() { _ = server.Serve(listener) }()
	return app, fmt.Sprintf("http://127.0.0.1:%d", port), server, listener
}

func stopTestWebServer(t *testing.T, app *serveApplication, server *http.Server, listener net.Listener) {
	t.Helper()
	app.closeConnections()
	_ = server.Close()
	_ = listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	app.waitConnections(ctx)
}

func waitForFileContent(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, _ := os.ReadFile(path)
	t.Fatalf("file %q did not contain %q; contents: %q", path, want, data)
}
