package entrypoint

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStartCodexAppServerDaemon(t *testing.T) {
	binDir := t.TempDir()
	codexPath := filepath.Join(binDir, "codex")
	script := `#!/bin/sh
case "$3" in
  start) exit 0 ;;
  version) printf '%s\n' '{"status":"running","socketPath":"/tmp/codex-control.sock"}' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(codexPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	socketPath, err := startCodexAppServerDaemon(context.Background(), codexPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if socketPath != "/tmp/codex-control.sock" {
		t.Fatalf("daemon socket path = %q, want the path reported by Codex", socketPath)
	}
}

func TestDialCodexAppServerSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "app-server.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, request, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var message codexRPCMessage
		if json.Unmarshal(request, &message) != nil || message.Method != "initialize" {
			return
		}
		_ = conn.WriteJSON(map[string]any{"id": message.ID, "result": map[string]string{"ready": "yes"}})
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	conn, err := dialCodexAppServerSocket(context.Background(), socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, response, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var message codexRPCMessage
	if err := json.Unmarshal(response, &message); err != nil {
		t.Fatal(err)
	}
	if message.Result == nil || !strings.Contains(string(message.Result), `"ready":"yes"`) {
		t.Fatalf("unexpected app-server socket response: %s", response)
	}
}

func TestCodexPathWithinProject(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if !codexPathWithinProject(root, root) {
		t.Fatal("project root should be within itself")
	}
	if !codexPathWithinProject(root, nested) {
		t.Fatal("nested directory should be within the project")
	}
	if codexPathWithinProject(root, outside) {
		t.Fatal("sibling directory should not be within the project")
	}
	if codexPathWithinProject(root, filepath.Join(root, "missing")) {
		t.Fatal("nonexistent paths should not be trusted")
	}

	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if codexPathWithinProject(root, link) {
		t.Fatal("symlink target outside the project should not be within the project")
	}
}

func TestSupportedCodexServerRequest(t *testing.T) {
	for _, method := range []string{
		"item/commandExecution/requestApproval",
		"item/fileChange/requestApproval",
		"item/permissions/requestApproval",
		"item/tool/requestUserInput",
	} {
		if !supportedCodexServerRequest(method) {
			t.Errorf("expected %q to be supported", method)
		}
	}
	if supportedCodexServerRequest("item/tool/call") {
		t.Fatal("dynamic tool calls must not be accepted as approval requests")
	}
}

func TestNormalizeCodexRequestID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "number", id: `42`, want: "42"},
		{name: "string", id: `"req-1"`, want: "req-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCodexRequestID([]byte(tt.id)); got != tt.want {
				t.Fatalf("normalizeCodexRequestID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCodexActiveWriterConflict(t *testing.T) {
	if !isCodexActiveWriterConflict(errors.New("codex app-server: thread 01abc already has an active writer")) {
		t.Fatal("active writer conflicts should be recognized")
	}
	if isCodexActiveWriterConflict(errors.New("codex app-server: thread not found")) {
		t.Fatal("unrelated app-server errors should not be treated as writer conflicts")
	}
}

func TestGetCodexThreadHistoryDoesNotResumeActiveThread(t *testing.T) {
	server, app := newTestServer(t)
	installFakeCodexAppServer(t, server, app.Project.Path)
	defer server.Shutdown()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/codex/threads/thread-1", nil)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("history request status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"turn-1"`) {
		t.Fatalf("history response did not contain the stored turn: %s", recorder.Body.String())
	}
}

func TestCodexThreadStreamReportsWriterConflict(t *testing.T) {
	server, app := newTestServer(t)
	installFakeCodexAppServer(t, server, app.Project.Path)
	defer server.Shutdown()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/codex/threads/thread-1/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(message, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "writer_conflict" {
		t.Fatalf("stream event type = %q, want writer_conflict; message: %s", event.Type, message)
	}
}

func TestCodexThreadStreamUnsubscribesAfterLastClientDisconnects(t *testing.T) {
	server, app := newTestServer(t)
	unsubscribeFile := filepath.Join(t.TempDir(), "unsubscribed.json")
	installConnectedFakeCodexAppServer(t, server, app.Project.Path, unsubscribeFile)
	defer server.Shutdown()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/codex/threads/thread-1/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(message, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "ready" {
		t.Fatalf("stream event type = %q, want ready; message: %s", event.Type, message)
	}
	secondConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := secondConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, message, err = secondConn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	event = struct {
		Type string `json:"type"`
	}{}
	if err := json.Unmarshal(message, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "ready" {
		t.Fatalf("second stream event type = %q, want ready; message: %s", event.Type, message)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(unsubscribeFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app-server unsubscribed while another client was still attached: %v", err)
	}
	if err := secondConn.Close(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(unsubscribeFile); err == nil {
			if !strings.Contains(string(data), `"method":"thread/unsubscribe"`) || !strings.Contains(string(data), `"threadId":"thread-1"`) {
				t.Fatalf("unexpected unsubscribe request: %s", data)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("app-server did not receive thread/unsubscribe after the client disconnected")
}

func TestCreateCodexThreadKeepsInitialSubscription(t *testing.T) {
	server, app := newTestServer(t)
	unsubscribeFile := filepath.Join(t.TempDir(), "unsubscribed.json")
	installConnectedFakeCodexAppServer(t, server, app.Project.Path, unsubscribeFile)
	defer server.Shutdown()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/codex/threads", nil)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create thread status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if _, err := os.Stat(unsubscribeFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new thread subscription was released before its first message: %v", err)
	}
}

func TestGetCodexThreadHistoryUsesStartedThreadBeforeItIsMaterialized(t *testing.T) {
	server, app := newTestServer(t)
	installUnmaterializedFakeCodexAppServer(t, server, app.Project.Path)
	defer server.Shutdown()

	create := httptest.NewRecorder()
	server.Handler().ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/codex/threads", nil))
	if create.Code != http.StatusCreated {
		t.Fatalf("create thread status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}

	history := httptest.NewRecorder()
	server.Handler().ServeHTTP(history, httptest.NewRequest(http.MethodGet, "/api/codex/threads/thread-new", nil))
	if history.Code != http.StatusOK {
		t.Fatalf("history request status = %d, want %d: %s", history.Code, http.StatusOK, history.Body.String())
	}
	if !strings.Contains(history.Body.String(), `"thread_id":"thread-new"`) || !strings.Contains(history.Body.String(), `"turns":[]`) {
		t.Fatalf("history response did not preserve the started empty thread: %s", history.Body.String())
	}
}

func TestEmptyCodexThreadCanBeReopenedAfterSwitchingAway(t *testing.T) {
	server, app := newTestServer(t)
	installUnmaterializedFakeCodexAppServer(t, server, app.Project.Path)
	defer server.Shutdown()

	create := httptest.NewRecorder()
	server.Handler().ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/codex/threads", nil))
	if create.Code != http.StatusCreated {
		t.Fatalf("create thread status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/codex/threads/thread-new/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, ready, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ready), `"type":"ready"`) {
		t.Fatalf("stream did not attach to the empty thread: %s", ready)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	server.codexAppServer.mu.Lock()
	p := server.codexAppServer.process
	server.codexAppServer.mu.Unlock()
	key := codexThreadKey{project: app.Project.Name, thread: "thread-new"}
	deadline := time.Now().Add(time.Second)
	for {
		state := server.codexAppServer.threadSubscription(key, p)
		state.mu.Lock()
		clients := state.clients
		state.mu.Unlock()
		if clients == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("WebSocket client did not detach from the empty thread")
		}
		time.Sleep(10 * time.Millisecond)
	}

	list := httptest.NewRecorder()
	server.Handler().ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/codex/threads", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"thread_id":"thread-new"`) {
		t.Fatalf("thread list did not retain the empty thread after switching away: status=%d body=%s", list.Code, list.Body.String())
	}
	history := httptest.NewRecorder()
	server.Handler().ServeHTTP(history, httptest.NewRequest(http.MethodGet, "/api/codex/threads/thread-new", nil))
	if history.Code != http.StatusOK {
		t.Fatalf("history request after switching away = %d, want %d: %s", history.Code, http.StatusOK, history.Body.String())
	}
}

func installFakeCodexAppServer(t *testing.T, server *Server, projectDir string) {
	t.Helper()
	server.codexAppServer.stdioTransport = true
	binDir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "app-server" ] && [ "$2" = "daemon" ]; then
  if [ "$3" = "version" ]; then
    printf '%s\n' '{"status":"running","socketPath":"/tmp/codex-test.sock"}'
  fi
  exit 0
fi
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      printf '%s\n' '{"id":1,"result":{}}'
      ;;
    *'"method":"thread/read"'*)
      printf '{"id":2,"result":{"thread":{"id":"thread-1","cwd":"%s","status":{"type":"notLoaded"}}}}\n' "$MYBOX_TEST_PROJECT"
      ;;
    *'"method":"thread/turns/list"'*)
      printf '%s\n' '{"id":3,"result":{"data":[{"id":"turn-1","status":"completed","items":[]}],"nextCursor":null}}'
      ;;
    *'"method":"thread/resume"'*)
      printf '%s\n' '{"id":3,"error":{"code":-32600,"message":"thread thread-1 already has an active writer"}}'
      ;;
  esac
done
`
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MYBOX_TEST_PROJECT", projectDir)
}

func TestResolveCodexPermissionRequestIsTurnScoped(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	process := &codexAppServerProcess{stdin: writer, done: make(chan struct{})}
	manager := newCodexAppServerManager()
	manager.pendingRequests["7"] = codexPendingServerRequest{
		id:      json.RawMessage(`7`),
		method:  "item/permissions/requestApproval",
		project: "demo",
		thread:  "thread-1",
		params:  json.RawMessage(`{"permissions":{"network":{"enabled":true}}}`),
		process: process,
	}
	if err := manager.resolveServerRequest("demo", "thread-1", "7", "accept", nil); err != nil {
		t.Fatal(err)
	}

	line, err := bufio.NewReader(reader).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		ID     int `json:"id"`
		Result struct {
			Permissions map[string]any `json:"permissions"`
			Scope       string         `json:"scope"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != 7 || response.Result.Scope != "turn" {
		t.Fatalf("unexpected permission response: %+v", response)
	}
	if _, ok := response.Result.Permissions["network"]; !ok {
		t.Fatalf("accepted permissions were not forwarded: %+v", response.Result.Permissions)
	}
}

func TestResolveCodexApprovalCanRetryAfterInvalidDecision(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	process := &codexAppServerProcess{stdin: writer, done: make(chan struct{})}
	manager := newCodexAppServerManager()
	manager.pendingRequests["7"] = codexPendingServerRequest{
		id:      json.RawMessage(`7`),
		method:  "item/commandExecution/requestApproval",
		project: "demo",
		thread:  "thread-1",
		params:  json.RawMessage(`{"availableDecisions":["accept","decline"]}`),
		process: process,
	}
	if err := manager.resolveServerRequest("demo", "thread-1", "7", "cancel", nil); err == nil {
		t.Fatal("invalid decision should be rejected")
	}
	if err := manager.resolveServerRequest("demo", "thread-1", "7", "decline", nil); err != nil {
		t.Fatalf("valid retry after invalid decision failed: %v", err)
	}

	line, err := bufio.NewReader(reader).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(line), `"decision":"decline"`) {
		t.Fatalf("valid retry did not send the decline decision: %s", line)
	}
}

func TestHandleCodexClientMessageStartsThreadCompaction(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	process := &codexAppServerProcess{
		stdin:   writer,
		done:    make(chan struct{}),
		pending: make(map[string]chan codexRPCResult),
	}
	manager := newCodexAppServerManager()
	server := &Server{codexAppServer: manager}
	done := make(chan error, 1)
	go func() {
		done <- server.handleCodexClientMessage(context.Background(), process, codexThreadKey{project: "demo", thread: "thread-1"}, codexClientMessage{Type: "compact"})
	}()

	line, err := bufio.NewReader(reader).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			ThreadID string `json:"threadId"`
		} `json:"params"`
	}
	if err := json.Unmarshal(line, &request); err != nil {
		t.Fatal(err)
	}
	if request.Method != "thread/compact/start" || request.Params.ThreadID != "thread-1" {
		t.Fatalf("unexpected Codex compaction request: %+v", request)
	}
	process.pendingMu.Lock()
	requestID := normalizeCodexRequestID(request.ID)
	response := process.pending[requestID]
	process.pendingMu.Unlock()
	if response == nil {
		t.Fatalf("request %s was not registered as pending", requestID)
	}
	response <- codexRPCResult{result: json.RawMessage(`{}`)}
	if err := <-done; err != nil {
		t.Fatalf("compact command failed: %v", err)
	}
}

func TestHandleCodexClientMessageSetsGuardianApprovalReviewer(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	process := &codexAppServerProcess{
		stdin:   writer,
		done:    make(chan struct{}),
		pending: make(map[string]chan codexRPCResult),
	}
	manager := newCodexAppServerManager()
	key := codexThreadKey{project: "demo", thread: "thread-1"}
	manager.threadSubscription(key, process)
	server := &Server{codexAppServer: manager}
	done := make(chan error, 1)
	go func() {
		done <- server.handleCodexClientMessage(context.Background(), process, key, codexClientMessage{Type: "set_approvals_reviewer", ApprovalsReviewer: "guardian_subagent"})
	}()

	line, err := bufio.NewReader(reader).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			ThreadID          string `json:"threadId"`
			ApprovalsReviewer string `json:"approvalsReviewer"`
		} `json:"params"`
	}
	if err := json.Unmarshal(line, &request); err != nil {
		t.Fatal(err)
	}
	if request.Method != "thread/settings/update" || request.Params.ThreadID != key.thread || request.Params.ApprovalsReviewer != "guardian_subagent" {
		t.Fatalf("unexpected approval reviewer update request: %+v", request)
	}
	process.pendingMu.Lock()
	response := process.pending[normalizeCodexRequestID(request.ID)]
	process.pendingMu.Unlock()
	if response == nil {
		t.Fatal("approval reviewer update was not registered as pending")
	}
	response <- codexRPCResult{result: json.RawMessage(`{}`)}
	if err := <-done; err != nil {
		t.Fatalf("approval reviewer update failed: %v", err)
	}
	if manager.threadSubs[codexThreadSubscriptionKey{thread: key, process: process}].approvalsReviewer != "guardian_subagent" {
		t.Fatal("thread subscription did not retain the selected approval reviewer")
	}
	manager.handleNotification(codexRPCMessage{
		Method: "thread/settings/updated",
		Params: json.RawMessage(`{"threadId":"thread-1","threadSettings":{"approvalsReviewer":"auto_review"}}`),
	})
	if manager.threadSubs[codexThreadSubscriptionKey{thread: key, process: process}].approvalsReviewer != "auto_review" {
		t.Fatal("thread settings notification did not refresh the approval reviewer")
	}
}

func installConnectedFakeCodexAppServer(t *testing.T, server *Server, projectDir, unsubscribeFile string) {
	t.Helper()
	server.codexAppServer.stdioTransport = true
	binDir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "app-server" ] && [ "$2" = "daemon" ]; then
  if [ "$3" = "version" ]; then
    printf '%s\n' '{"status":"running","socketPath":"/tmp/codex-test.sock"}'
  fi
  exit 0
fi
while IFS= read -r line; do
  request_id=$(printf '%s\n' "$line" | sed -E 's/.*"id":([0-9]+).*/\1/')
  case "$line" in
    *'"method":"initialize"'*)
      printf '{"id":%s,"result":{}}\n' "$request_id"
      ;;
    *'"method":"thread/read"'*)
      printf '{"id":%s,"result":{"thread":{"id":"thread-1","cwd":"%s","status":{"type":"notLoaded"}}}}\n' "$request_id" "$MYBOX_TEST_PROJECT"
      ;;
    *'"method":"thread/start"'*)
      printf '{"id":%s,"result":{"thread":{"id":"thread-1","cwd":"%s","status":{"type":"notLoaded"}}}}\n' "$request_id" "$MYBOX_TEST_PROJECT"
      ;;
    *'"method":"thread/resume"'*)
      printf '{"id":%s,"result":{"thread":{"id":"thread-1"}}}\n' "$request_id"
      ;;
    *'"method":"thread/unsubscribe"'*)
      printf '%s\n' "$line" > "$MYBOX_TEST_UNSUBSCRIBE_FILE"
      printf '{"id":%s,"result":{"status":"unsubscribed"}}\n' "$request_id"
      ;;
  esac
done
`
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MYBOX_TEST_PROJECT", projectDir)
	t.Setenv("MYBOX_TEST_UNSUBSCRIBE_FILE", unsubscribeFile)
}

func installUnmaterializedFakeCodexAppServer(t *testing.T, server *Server, projectDir string) {
	t.Helper()
	server.codexAppServer.stdioTransport = true
	binDir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "app-server" ] && [ "$2" = "daemon" ]; then
  if [ "$3" = "version" ]; then
    printf '%s\n' '{"status":"running","socketPath":"/tmp/codex-test.sock"}'
  fi
  exit 0
fi
while IFS= read -r line; do
  request_id=$(printf '%s\n' "$line" | sed -E 's/.*"id":([0-9]+).*/\1/')
  case "$line" in
    *'"method":"initialize"'*)
      printf '{"id":%s,"result":{}}\n' "$request_id"
      ;;
    *'"method":"thread/start"'*)
      printf '{"id":%s,"result":{"thread":{"id":"thread-new","cwd":"%s","status":{"type":"notLoaded"}}}}\n' "$request_id" "$MYBOX_TEST_PROJECT"
      ;;
    *'"method":"thread/read"'*)
      printf '{"id":%s,"error":{"code":-32600,"message":"thread not materialized yet"}}\n' "$request_id"
      ;;
    *'"method":"thread/turns/list"'*)
      printf '{"id":%s,"error":{"code":-32600,"message":"thread not materialized yet"}}\n' "$request_id"
      ;;
    *'"method":"thread/list"'*)
      printf '{"id":%s,"result":{"data":[],"nextCursor":null}}\n' "$request_id"
      ;;
  esac
done
`
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MYBOX_TEST_PROJECT", projectDir)
}
