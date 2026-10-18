package entrypoint

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	configinfra "github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
)

type browserAuditRecorder struct {
	events []domain.BrowserAuditEvent
}

func (r *browserAuditRecorder) Append(_ context.Context, event domain.BrowserAuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func TestBrowserProjectPrefersBodyThenHeaderThenQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=query", nil)
	req.Header.Set("X-Project", "header")
	require.Equal(t, "body", browserProject(req, "body"))
	require.Equal(t, "header", browserProject(req, ""))
	req.Header.Del("X-Project")
	require.Equal(t, "query", browserProject(req, ""))
}

func TestBrowserSessionAuthorizationIsExclusive(t *testing.T) {
	session := &browserSession{owner: "agent"}
	require.NoError(t, session.authorize("agent"))
	require.EqualError(t, session.authorize("user"), "browser session is controlled by agent")
	session.owner = "user"
	require.NoError(t, session.authorize("user"))
}

func TestBrowserApprovalModeCanOnlyBeChangedByUser(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	server.browser.sessions["session-1"] = &browserSession{
		project: "demo",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
	}

	request := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/approval-mode?project=demo", strings.NewReader(`{"mode":"auto_safe"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Browser-Actor", "agent")
	recorder := httptest.NewRecorder()
	server.BrowserApprovalMode(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "only the user")
	require.ErrorContains(t, authorizeBrowserCommand("agent", "set_approval_mode"), "user-only")
}

func TestBrowserApprovalModeRejectsUnknownValuesBeforeWorkerCommand(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	server.browser.sessions["session-1"] = &browserSession{
		project: "demo",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
	}

	request := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/approval-mode?project=demo", strings.NewReader(`{"mode":"always"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.BrowserApprovalMode(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "must be manual or auto_safe")
}

func TestBrowserRoutesAreRegisteredOutsideGeneratedAPI(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"session":null}`, recorder.Body.String())
}

func TestAgentCannotCreateBrowserSessionWithInitialURL(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	request := httptest.NewRequest(http.MethodPost, "/api/browser/sessions", strings.NewReader(`{"project":"demo","url":"https://example.com"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Browser-Actor", "agent")
	recorder := httptest.NewRecorder()

	server.CreateBrowserSession(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "agent must start at about:blank")
}

func TestBrowserInitialURLAllowsAgentBlankPageOnly(t *testing.T) {
	for _, test := range []struct {
		name  string
		actor string
		url   string
		want  bool
	}{
		{name: "agent empty URL", actor: "agent"},
		{name: "agent blank page", actor: "agent", url: "about:blank"},
		{name: "agent trimmed blank page", actor: "agent", url: " about:blank "},
		{name: "agent external URL", actor: "agent", url: "https://example.com", want: true},
		{name: "user external URL", actor: "user", url: "https://example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateBrowserInitialURL(test.actor, test.url)
			if test.want {
				require.ErrorContains(t, err, "browser_navigate for user approval")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBrowserAuditEndpointFiltersCurrentProjectAndResult(t *testing.T) {
	store := configinfra.NewBrowserAuditStoreAt(filepath.Join(t.TempDir(), "audit.jsonl"))
	base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, event := range []domain.BrowserAuditEvent{
		{Timestamp: base, Event: "session_created", Project: "demo", SessionID: "session-1", Result: "success"},
		{Timestamp: base.Add(time.Minute), Event: "browser_command", Project: "demo", SessionID: "session-1", Command: "navigate", Result: "blocked", Reason: "sensitive_field"},
		{Timestamp: base.Add(2 * time.Minute), Event: "browser_command", Project: "other", SessionID: "other-session", Command: "navigate", Result: "blocked"},
	} {
		require.NoError(t, store.Append(context.Background(), event))
	}

	server := NewServerWithBrowserAuditStore(&domain.Config{}, "", "", store)
	defer server.Shutdown()
	req := httptest.NewRequest(http.MethodGet, "/api/browser/audit?project=demo&result=blocked&limit=1", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response api.BrowserAuditResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Total)
	require.Len(t, response.Events, 1)
	require.Equal(t, "demo", response.Events[0].Project)
	require.Equal(t, "blocked", response.Events[0].Result)
	require.Equal(t, "sensitive_field", *response.Events[0].Reason)
}

func TestBrowserAuditEndpointRequiresLocalUserAndProjectScope(t *testing.T) {
	server := NewServerWithBrowserAuditStore(&domain.Config{}, "", "", configinfra.NewBrowserAuditStoreAt(filepath.Join(t.TempDir(), "audit.jsonl")))
	defer server.Shutdown()

	missingProject := httptest.NewRequest(http.MethodGet, "/api/browser/audit", nil)
	missingProject.RemoteAddr = "127.0.0.1:1234"
	missingRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingRecorder, missingProject)
	require.Equal(t, http.StatusBadRequest, missingRecorder.Code)

	remote := httptest.NewRequest(http.MethodGet, "/api/browser/audit?project=demo", nil)
	remote.RemoteAddr = "198.51.100.10:1234"
	remoteRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(remoteRecorder, remote)
	require.Equal(t, http.StatusForbidden, remoteRecorder.Code)
}

func TestBrowserSessionIDHelpers(t *testing.T) {
	require.Equal(t, "session-1", browserSessionID("/api/browser/sessions/session-1/commands"))
	require.Equal(t, "approval-1", browserApprovalID("/api/browser/sessions/session-1/approvals/approval-1/approve"))
	require.Empty(t, browserSessionID("/api/browser/stream"))
}

func TestBrowserStreamReplaysLatestFrameToNewClient(t *testing.T) {
	session := &browserSession{
		id:      "session-1",
		clients: make(map[*browserClient]struct{}),
	}
	hub := &browserSessionHub{sessions: map[string]*browserSession{session.id: session}}
	frame := []byte{0xff, 0xd8, 0x01, 0x02, 0xff, 0xd9}
	message := make([]byte, 4+len(session.id)+len(frame))
	binary.BigEndian.PutUint32(message[:4], uint32(len(session.id)))
	copy(message[4:], session.id)
	copy(message[4+len(session.id):], frame)

	hub.broadcastBinary(message)
	client := session.addClient()
	defer client.close()

	select {
	case received := <-client.send:
		require.Equal(t, websocket.BinaryMessage, received.messageType)
		require.Equal(t, frame, received.data)
	default:
		t.Fatal("new browser stream client did not receive the latest frame")
	}
}

func TestBrowserSessionsWrapsAnActiveSession(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	server.browser.sessions["session-1"] = &browserSession{
		project: "demo",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
		url:     "https://example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	server.BrowserSessions(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"session":{"id":"session-1","project":"demo","state":"running","owner":"agent","url":"https://example.com","title":"","pending_approval":null,"approval_mode":"manual"}}`, recorder.Body.String())
}

func TestBrowserSessionsRequiresProjectScope(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()

	server.BrowserSessions(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"error":"browser project is required"}`, recorder.Body.String())
}

func TestBrowserSessionLookupRequiresMatchingProjectScope(t *testing.T) {
	hub := newBrowserSessionHub()
	hub.sessions["session-1"] = &browserSession{
		project: "project-a",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
	}

	withoutProject := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/commands", nil)
	withoutProject.RemoteAddr = "127.0.0.1:1234"
	_, ok := hub.session(withoutProject)
	require.False(t, ok)

	wrongProject := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/commands", nil)
	wrongProject.RemoteAddr = "127.0.0.1:1234"
	wrongProject.Header.Set("X-Project", "project-b")
	_, ok = hub.session(wrongProject)
	require.False(t, ok)

	matchingProject := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/commands", nil)
	matchingProject.RemoteAddr = "127.0.0.1:1234"
	matchingProject.Header.Set("X-Project", "project-a")
	_, ok = hub.session(matchingProject)
	require.True(t, ok)
}

func TestBrowserSessionsRejectsRemoteUserRequests(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Host = "mybox.example"
	req.Header.Set("Origin", "http://mybox.example")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	recorder := httptest.NewRecorder()

	server.BrowserSessions(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestBrowserRequestRejectsRemoteUserEvenWithAgentToken(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_AGENT_TOKEN", "agent-secret")
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Browser-Agent-Token", "agent-secret")

	require.Error(t, authorizeBrowserRequest(req, "user", false))
}

func TestBrowserRequestRejectsCrossOriginLoopbackRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Host = "mybox.example"
	req.Header.Set("Origin", "https://attacker.example")

	require.Error(t, authorizeBrowserRequest(req, "user", false))
}

func TestBrowserRequestAllowsSameOriginLoopbackUserRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/browser/sessions?project=demo", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Host = "mybox.example"
	req.Header.Set("Origin", "https://mybox.example")

	require.NoError(t, authorizeBrowserRequest(req, "user", false))
}

func TestBrowserStreamRejectsRemoteUserRequests(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	req := httptest.NewRequest(http.MethodGet, "/api/browser/stream?session_id=session-1", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Host = "mybox.example"
	req.Header.Set("Origin", "http://mybox.example")
	recorder := httptest.NewRecorder()

	server.BrowserStream(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestBrowserRequestRejectsRemoteAgentWithoutToken(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_AGENT_TOKEN", "")
	req := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/commands", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Browser-Actor", "agent")

	require.Error(t, authorizeBrowserRequest(req, "agent", false))
}

func TestBrowserRequestAcceptsConfiguredAgentToken(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_AGENT_TOKEN", "agent-secret")
	req := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/commands", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Browser-Actor", "agent")
	req.Header.Set("X-Browser-Agent-Token", "agent-secret")

	require.NoError(t, authorizeBrowserRequest(req, "agent", false))
}

func TestBrowserCommandRejectsAgentInput(t *testing.T) {
	require.Error(t, authorizeBrowserCommand("agent", "input"))
	require.NoError(t, authorizeBrowserCommand("user", "input"))
}

func TestBrowserCommandTimeoutFromEnvironment(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_COMMAND_TIMEOUT", "1500ms")
	require.Equal(t, 1500*time.Millisecond, browserCommandTimeoutFromEnvironment())

	t.Setenv("MYBOX_BROWSER_COMMAND_TIMEOUT", "not-a-duration")
	require.Equal(t, browserCommandTimeoutDefault, browserCommandTimeoutFromEnvironment())

	t.Setenv("MYBOX_BROWSER_COMMAND_TIMEOUT", "0s")
	require.Equal(t, browserCommandTimeoutDefault, browserCommandTimeoutFromEnvironment())
}

func TestBrowserSessionPolicyFromEnvironment(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_MAX_WORKERS", "2")
	t.Setenv("MYBOX_BROWSER_IDLE_TIMEOUT", "90s")
	policy := browserSessionPolicyFromEnvironment()
	require.Equal(t, 2, policy.maxWorkers)
	require.Equal(t, 90*time.Second, policy.idleTimeout)

	t.Setenv("MYBOX_BROWSER_MAX_WORKERS", "0")
	t.Setenv("MYBOX_BROWSER_IDLE_TIMEOUT", "0s")
	policy = browserSessionPolicyFromEnvironment()
	require.Zero(t, policy.maxWorkers)
	require.Zero(t, policy.idleTimeout)

	t.Setenv("MYBOX_BROWSER_MAX_WORKERS", "invalid")
	t.Setenv("MYBOX_BROWSER_IDLE_TIMEOUT", "invalid")
	policy = browserSessionPolicyFromEnvironment()
	require.Equal(t, browserWorkerMaxDefault, policy.maxWorkers)
	require.Zero(t, policy.idleTimeout)
}

func TestBrowserAuditCommandRedactsPayloadAndClassifiesSensitiveBlock(t *testing.T) {
	recorder := &browserAuditRecorder{}
	hub := newBrowserSessionHubWithAudit(browserSessionPolicy{}, recorder)
	session := &browserSession{hub: hub, project: "demo", id: "session-1"}
	data := json.RawMessage(`{"value":"do-not-log-this-secret"}`)

	hub.auditCommand(session, "agent", "fill", data, errors.New("sensitive fields must be filled by the user"))

	if len(recorder.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(recorder.events))
	}
	event := recorder.events[0]
	require.Equal(t, "browser_command", event.Event)
	require.Equal(t, "fill", event.Command)
	require.Equal(t, "blocked", event.Result)
	require.Equal(t, "sensitive_field", event.Reason)
	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "do-not-log-this-secret")
	require.NotContains(t, string(encoded), "https://")
}

func TestBrowserAuditOmitsSuccessfulUserInputAndNormalizesUnknownCommands(t *testing.T) {
	recorder := &browserAuditRecorder{}
	hub := newBrowserSessionHubWithAudit(browserSessionPolicy{}, recorder)
	session := &browserSession{hub: hub, project: "demo", id: "session-1"}

	hub.auditCommand(session, "user", "input", json.RawMessage(`{"text":"private text"}`), nil)
	hub.auditCommand(session, "agent", "unexpected-command", nil, errors.New("unsupported browser command"))

	require.Len(t, recorder.events, 1)
	require.Equal(t, "unknown", recorder.events[0].Command)
	require.Equal(t, "error", recorder.events[0].Result)
}

func TestBrowserControlWritesTakeoverAndReturnAuditEvents(t *testing.T) {
	recorder := &browserAuditRecorder{}
	server := NewServerWithBrowserAuditStore(&domain.Config{}, "", "", recorder)
	defer server.Shutdown()
	session := &browserSession{
		hub:     server.browser,
		project: "demo",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
	}
	server.browser.sessions[session.id] = session

	takeover := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/control/takeover?project=demo", nil)
	takeover.RemoteAddr = "127.0.0.1:1234"
	takeoverRecorder := httptest.NewRecorder()
	server.BrowserTakeover(takeoverRecorder, takeover)
	require.Equal(t, http.StatusOK, takeoverRecorder.Code)

	returnControl := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/control/return?project=demo", nil)
	returnControl.RemoteAddr = "127.0.0.1:1234"
	returnRecorder := httptest.NewRecorder()
	server.BrowserReturnControl(returnRecorder, returnControl)
	require.Equal(t, http.StatusOK, returnRecorder.Code)

	require.Len(t, recorder.events, 2)
	require.Equal(t, "control_takeover", recorder.events[0].Event)
	require.Equal(t, "control_return", recorder.events[1].Event)
	require.Equal(t, "success", recorder.events[0].Result)
	require.Equal(t, "success", recorder.events[1].Result)
}

func TestBrowserSessionHubRejectsWorkerBeyondConfiguredLimit(t *testing.T) {
	hub := newBrowserSessionHubWithPolicy(browserSessionPolicy{maxWorkers: 1})
	defer hub.stop()
	hub.workers["project-a"] = &browserWorker{project: "project-a", hub: hub, stop: make(chan struct{})}

	_, err := hub.ensureWorker(context.Background(), "project-b")
	var httpErr *httpError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusTooManyRequests, httpErr.status)
	require.EqualError(t, err, "browser worker limit reached")
}

func TestBrowserSessionHubExpiresOnlyIdleSessions(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	hub := newBrowserSessionHubWithPolicy(browserSessionPolicy{idleTimeout: time.Minute})
	defer hub.stop()

	idle := &browserSession{
		hub:          hub,
		project:      "idle-project",
		id:           "idle-session",
		state:        "running",
		owner:        "agent",
		lastActivity: now.Add(-2 * time.Minute),
		pending:      make(map[string]chan browserWorkerResult),
		clients:      make(map[*browserClient]struct{}),
	}
	userControlled := &browserSession{
		hub:          hub,
		project:      "user-project",
		id:           "user-session",
		state:        "user_control",
		owner:        "user",
		lastActivity: now.Add(-2 * time.Minute),
		pending:      make(map[string]chan browserWorkerResult),
		clients:      make(map[*browserClient]struct{}),
	}
	approvalPending := &browserSession{
		hub:             hub,
		project:         "approval-project",
		id:              "approval-session",
		state:           "approval_pending",
		owner:           "agent",
		lastActivity:    now.Add(-2 * time.Minute),
		pending:         make(map[string]chan browserWorkerResult),
		clients:         make(map[*browserClient]struct{}),
		pendingApproval: &browserApproval{ID: "approval-1"},
	}
	connected := &browserSession{
		hub:          hub,
		project:      "connected-project",
		id:           "connected-session",
		state:        "running",
		owner:        "agent",
		lastActivity: now.Add(-2 * time.Minute),
		pending:      make(map[string]chan browserWorkerResult),
		clients:      make(map[*browserClient]struct{}),
	}
	connected.clients[&browserClient{}] = struct{}{}
	inFlight := &browserSession{
		hub:             hub,
		project:         "in-flight-project",
		id:              "in-flight-session",
		state:           "running",
		owner:           "agent",
		lastActivity:    now.Add(-2 * time.Minute),
		workerRequestID: "request-1",
		pending:         make(map[string]chan browserWorkerResult),
		clients:         make(map[*browserClient]struct{}),
	}
	for _, session := range []*browserSession{idle, userControlled, approvalPending, connected, inFlight} {
		hub.sessions[session.id] = session
	}

	require.Equal(t, 1, hub.expireIdleSessions(now))
	require.True(t, idle.isClosed())
	require.False(t, userControlled.isClosed())
	require.False(t, approvalPending.isClosed())
	require.False(t, connected.isClosed())
	require.False(t, inFlight.isClosed())
	hub.mu.Lock()
	_, idleRegistered := hub.sessions[idle.id]
	hub.mu.Unlock()
	require.False(t, idleRegistered)
}

func TestBrowserSessionControlIsBlockedWhileWorkerCommandIsInFlight(t *testing.T) {
	session := &browserSession{workerRequestID: "request-1"}
	require.False(t, session.tryAcquireCommand())

	session.finishWorkerRequest("request-1")
	require.True(t, session.tryAcquireCommand())
	session.releaseCommand()
}

func TestBrowserCommandTimeoutClosesOnlyTimedOutSession(t *testing.T) {
	originalTimeout := browserCommandTimeout
	browserCommandTimeout = 10 * time.Millisecond
	t.Cleanup(func() { browserCommandTimeout = originalTimeout })

	hub := newBrowserSessionHub()
	session := &browserSession{
		hub:     hub,
		id:      "timed-out-session",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
		state:   "running",
		owner:   "agent",
	}
	other := &browserSession{
		hub:     hub,
		id:      "other-session",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
		state:   "running",
		owner:   "agent",
	}
	hub.sessions[session.id] = session
	hub.sessions[other.id] = other

	commands := make(chan browserWorkerMessage, 2)
	stopServer := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var message browserWorkerMessage
			if err := json.Unmarshal(data, &message); err != nil {
				return
			}
			commands <- message
			if message.Command == "close_session" {
				_ = conn.WriteJSON(map[string]any{
					"type":       "result",
					"session_id": message.SessionID,
					"request_id": message.RequestID,
					"data":       map[string]any{"ok": true},
				})
				<-stopServer
				return
			}
		}
	}))
	defer server.Close()

	workerConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	worker := &browserWorker{conn: workerConn, hub: hub, stop: make(chan struct{})}
	session.worker = worker
	other.worker = worker
	hub.workers[""] = worker
	go worker.readLoop()

	commandResult := make(chan error, 1)
	go func() {
		_, err := session.commandWithWorker(context.Background(), worker, "agent", "wait", nil)
		commandResult <- err
	}()

	first := <-commands
	require.Equal(t, "wait", first.Command)
	second := <-commands
	require.Equal(t, "close_session", second.Command)
	require.Equal(t, session.id, second.SessionID)
	require.EqualError(t, <-commandResult, "browser command timed out")

	hub.mu.Lock()
	require.Same(t, worker, hub.workers[""])
	_, timedOutStillRegistered := hub.sessions[session.id]
	_, otherStillRegistered := hub.sessions[other.id]
	hub.mu.Unlock()
	require.False(t, timedOutStillRegistered)
	require.True(t, otherStillRegistered)

	session.mu.Lock()
	require.Empty(t, session.pending)
	require.True(t, session.closed)
	session.mu.Unlock()

	hub.mu.Lock()
	delete(hub.workers, "")
	hub.mu.Unlock()
	worker.close()
	close(stopServer)
}

func TestBrowserWorkerFailureKeepsSessionForExplicitRestart(t *testing.T) {
	recorder := &browserAuditRecorder{}
	hub := newBrowserSessionHubWithAudit(browserSessionPolicy{}, recorder)
	session := &browserSession{
		hub:     hub,
		project: "demo",
		id:      "failed-session",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
		state:   "running",
		owner:   "agent",
	}
	pending := make(chan browserWorkerResult, 1)
	session.pending["request-1"] = pending
	hub.sessions[session.id] = session
	worker := &browserWorker{hub: hub, stop: make(chan struct{})}
	session.worker = worker
	hub.workers[""] = worker

	hub.workerFailed(worker)

	hub.mu.Lock()
	_, stillRegistered := hub.sessions[session.id]
	workerCleared := hub.workers[""] == nil
	hub.mu.Unlock()
	require.True(t, stillRegistered)
	require.True(t, workerCleared)

	session.mu.Lock()
	closed := session.closed
	state := session.state
	session.mu.Unlock()
	require.False(t, closed)
	require.Equal(t, "error", state)
	snapshot := session.snapshot()
	require.Equal(t, "browser worker disconnected", snapshot["error"])
	require.EqualError(t, (<-pending).err, "browser worker disconnected")
	require.Len(t, recorder.events, 1)
	require.Equal(t, "worker_failure", recorder.events[0].Event)
	require.Equal(t, "system", recorder.events[0].Actor)
	require.Equal(t, "error", recorder.events[0].Result)
}

func TestBrowserWorkerFailureOnlyAffectsItsProject(t *testing.T) {
	hub := newBrowserSessionHub()
	workerA := &browserWorker{project: "project-a", hub: hub, stop: make(chan struct{})}
	workerB := &browserWorker{project: "project-b", hub: hub, stop: make(chan struct{})}
	sessionA := &browserSession{
		hub:     hub,
		project: "project-a",
		id:      "session-a",
		worker:  workerA,
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
		state:   "running",
		owner:   "agent",
	}
	sessionB := &browserSession{
		hub:     hub,
		project: "project-b",
		id:      "session-b",
		worker:  workerB,
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
		state:   "running",
		owner:   "agent",
	}
	hub.sessions[sessionA.id] = sessionA
	hub.sessions[sessionB.id] = sessionB
	hub.workers[workerA.project] = workerA
	hub.workers[workerB.project] = workerB

	hub.workerFailed(workerA)

	require.Equal(t, "error", sessionA.stateValue())
	require.Equal(t, "running", sessionB.stateValue())
	hub.mu.Lock()
	_, workerAStillRegistered := hub.workers[workerA.project]
	workerBStillRegistered := hub.workers[workerB.project] == workerB
	hub.mu.Unlock()
	require.False(t, workerAStillRegistered)
	require.True(t, workerBStillRegistered)
}

func TestBrowserCloseSessionRemovesDisconnectedSessionWithoutWorker(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	session := &browserSession{
		hub:     server.browser,
		project: "demo",
		id:      "failed-session",
		state:   "error",
		owner:   "agent",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
	}
	server.browser.sessions[session.id] = session

	req := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/failed-session/close?project=demo", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	server.BrowserCloseSession(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	server.browser.mu.Lock()
	_, stillRegistered := server.browser.sessions[session.id]
	server.browser.mu.Unlock()
	require.False(t, stillRegistered)
}

func TestBrowserRemoveStopsWorkerWhenProjectHasNoSessions(t *testing.T) {
	hub := newBrowserSessionHub()
	worker := &browserWorker{project: "demo", hub: hub, stop: make(chan struct{})}
	session := &browserSession{
		hub:     hub,
		project: "demo",
		id:      "session-1",
		worker:  worker,
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
	}
	hub.sessions[session.id] = session
	hub.workers[worker.project] = worker

	hub.remove(session)

	hub.mu.Lock()
	_, workerStillRegistered := hub.workers[worker.project]
	hub.mu.Unlock()
	require.False(t, workerStillRegistered)
	select {
	case <-worker.stop:
	default:
		t.Fatal("expected worker to be stopped")
	}
}

func TestBrowserRestartSessionRequiresUserAndFailedState(t *testing.T) {
	server := NewServer(&domain.Config{}, "", "")
	session := &browserSession{
		hub:     server.browser,
		project: "demo",
		id:      "session-1",
		state:   "running",
		owner:   "agent",
		pending: make(map[string]chan browserWorkerResult),
		clients: make(map[*browserClient]struct{}),
	}
	server.browser.sessions[session.id] = session

	agentRequest := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/restart?project=demo", nil)
	agentRequest.RemoteAddr = "127.0.0.1:1234"
	agentRequest.Header.Set("X-Browser-Actor", "agent")
	agentRecorder := httptest.NewRecorder()
	server.BrowserRestartSession(agentRecorder, agentRequest)
	require.Equal(t, http.StatusForbidden, agentRecorder.Code)

	userRequest := httptest.NewRequest(http.MethodPost, "/api/browser/sessions/session-1/restart?project=demo", nil)
	userRequest.RemoteAddr = "127.0.0.1:1234"
	userRecorder := httptest.NewRecorder()
	server.BrowserRestartSession(userRecorder, userRequest)
	require.Equal(t, http.StatusConflict, userRecorder.Code)
}

func TestBrowserStateFollowsControllerOwner(t *testing.T) {
	require.Equal(t, "running", browserStateForOwner("user_control", "agent"))
	require.Equal(t, "user_control", browserStateForOwner("running", "user"))
	require.Equal(t, "approval_pending", browserStateForOwner("approval_pending", "agent"))
}

func TestBrowserClientInitialErrorSnapshotIsRaceSafe(t *testing.T) {
	session := &browserSession{
		state:   "running",
		clients: make(map[*browserClient]struct{}),
		pending: make(map[string]chan browserWorkerResult),
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for index := 0; index < 2000; index++ {
			session.mu.Lock()
			session.state = "error"
			session.lastError = "browser worker disconnected"
			session.mu.Unlock()
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for index := 0; index < 2000; index++ {
			session.addClient().close()
		}
	}()
	close(start)
	workers.Wait()
}

func TestBrowserProfileResetSerializesWithSessionCreation(t *testing.T) {
	project := "profile-reset-race-test"
	t.Setenv("MYBOX_BROWSER_WORKER", filepath.Join(t.TempDir(), "missing-worker.mjs"))
	hub := newBrowserSessionHub()
	resetEntered := make(chan struct{})
	allowReset := make(chan struct{})
	resetDone := make(chan error, 1)
	go func() {
		resetDone <- hub.resetProfile(project, func(string) error {
			close(resetEntered)
			<-allowReset
			return nil
		})
	}()
	<-resetEntered

	createDone := make(chan error, 1)
	go func() {
		_, _, err := hub.create(context.Background(), project, "about:blank", "user")
		createDone <- err
	}()
	select {
	case err := <-createDone:
		t.Fatalf("session creation passed a profile reset in progress: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(allowReset)
	require.NoError(t, <-resetDone)
	require.ErrorContains(t, <-createDone, "browser worker is unavailable")
	hub.mu.Lock()
	defer hub.mu.Unlock()
	require.Empty(t, hub.projectOps)
}
