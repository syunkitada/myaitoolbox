package entrypoint

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

const (
	codexAppServerRequestTimeout        = 45 * time.Second
	codexAppServerStartTimeout          = 30 * time.Second
	codexAppServerMaxLine               = 32 << 20
	codexAppServerMaxPrompt             = 10000
	codexAppServerMaxClientMessageBytes = 1 << 20
	codexAppServerMaxPlanPromptBytes    = codexAppServerMaxClientMessageBytes
	codexThreadPageSize                 = 100
	codexThreadScanLimit                = 5000
	codexTurnPageSize                   = 20
	codexUnmaterializedThreadTTL        = 2 * time.Minute
	codexSubscriptionRetryDelay         = 2 * time.Second
)

var errCodexEmptyThreadExpired = errors.New("Codex empty conversation is no longer available")

type codexAppServerManager struct {
	mu              sync.Mutex
	process         *codexAppServerProcess
	closed          bool
	stdioTransport  bool
	threadSubMu     sync.Mutex
	threadSubs      map[codexThreadSubscriptionKey]*codexThreadSubscription
	clientsMu       sync.Mutex
	clients         map[codexThreadKey]map[*codexStreamClient]struct{}
	pendingRequests map[string]codexPendingServerRequest
}

type codexThreadSubscription struct {
	mu                sync.Mutex
	clients           int
	subscribed        bool
	materialized      bool
	expired           bool
	startedThread     *codexThreadRecord
	idleTimer         *time.Timer
	retryTimer        *time.Timer
	unknown           bool
	collaborationMode json.RawMessage
	approvalsReviewer string
}

type codexThreadSubscriptionKey struct {
	thread  codexThreadKey
	process *codexAppServerProcess
}

type codexThreadKey struct {
	project string
	thread  string
}

type codexStreamClient struct {
	key       codexThreadKey
	process   *codexAppServerProcess
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	manager   *codexAppServerManager
}

type codexPendingServerRequest struct {
	id        json.RawMessage
	method    string
	project   string
	thread    string
	params    json.RawMessage
	process   *codexAppServerProcess
	resolving bool
}

type codexAppServerProcess struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	conn      *websocket.Conn
	done      chan struct{}
	doneOnce  sync.Once
	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan codexRPCResult
	requestID atomic.Uint64
	initOnce  sync.Once
	initErr   error
	manager   *codexAppServerManager
}

type codexRPCMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *codexRPCError  `json:"error,omitempty"`
}

type codexRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type codexRPCResult struct {
	result json.RawMessage
	err    error
}

type codexThreadRecord struct {
	ID              string          `json:"id"`
	Name            *string         `json:"name"`
	Preview         string          `json:"preview"`
	CWD             string          `json:"cwd"`
	CreatedAt       int64           `json:"createdAt"`
	UpdatedAt       int64           `json:"updatedAt"`
	Model           *string         `json:"model"`
	ReasoningEffort *string         `json:"reasoningEffort"`
	ParentThreadID  *string         `json:"parentThreadId"`
	Status          json.RawMessage `json:"status"`
}

func newCodexAppServerManager() *codexAppServerManager {
	return &codexAppServerManager{
		threadSubs:      make(map[codexThreadSubscriptionKey]*codexThreadSubscription),
		clients:         make(map[codexThreadKey]map[*codexStreamClient]struct{}),
		pendingRequests: make(map[string]codexPendingServerRequest),
	}
}

func (m *codexAppServerManager) threadSubscription(key codexThreadKey, p *codexAppServerProcess) *codexThreadSubscription {
	m.threadSubMu.Lock()
	defer m.threadSubMu.Unlock()
	select {
	case <-p.done:
		// Do not retain subscription state for a process that exited while a
		// request was waiting to access it. processExited cleans states that
		// were registered before the exit.
		return &codexThreadSubscription{materialized: true, expired: true}
	default:
	}
	subscriptionKey := codexThreadSubscriptionKey{thread: key, process: p}
	state := m.threadSubs[subscriptionKey]
	if state == nil {
		state = &codexThreadSubscription{materialized: true}
		m.threadSubs[subscriptionKey] = state
	}
	return state
}

func (m *codexAppServerManager) attachThread(ctx context.Context, p *codexAppServerProcess, key codexThreadKey) (json.RawMessage, string, error) {
	state := m.threadSubscription(key, p)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.clients > 0 {
		state.clients++
		return append(json.RawMessage(nil), state.collaborationMode...), state.approvalsReviewer, nil
	}
	if state.expired {
		return nil, "", errCodexEmptyThreadExpired
	}
	if state.idleTimer != nil {
		state.idleTimer.Stop()
		state.idleTimer = nil
	}
	if state.retryTimer != nil {
		state.retryTimer.Stop()
		state.retryTimer = nil
	}
	if state.unknown {
		// An unsubscribe request may have taken effect even if its response was
		// lost. Reconcile the server state before resuming so the stream cannot
		// silently remain detached while mybox believes it is subscribed.
		if _, err := p.request(ctx, "thread/unsubscribe", map[string]string{"threadId": key.thread}); err != nil {
			m.scheduleSubscriptionRetryLocked(p, key, state)
			return nil, "", err
		}
		state.subscribed = false
		state.unknown = false
	}
	if !state.subscribed {
		result, err := p.request(ctx, "thread/resume", map[string]any{"threadId": key.thread, "excludeTurns": true})
		if err != nil {
			if isCodexThreadNotResumable(err) {
				return nil, "", errCodexEmptyThreadExpired
			}
			return nil, "", err
		}
		var resumed struct {
			CollaborationMode json.RawMessage `json:"collaborationMode"`
			ApprovalsReviewer string          `json:"approvalsReviewer"`
		}
		if json.Unmarshal(result, &resumed) == nil && len(resumed.CollaborationMode) > 0 && string(resumed.CollaborationMode) != "null" {
			state.collaborationMode = append(json.RawMessage(nil), resumed.CollaborationMode...)
		}
		state.approvalsReviewer = resumed.ApprovalsReviewer
		state.subscribed = true
		state.materialized = true
	}
	state.clients = 1
	return append(json.RawMessage(nil), state.collaborationMode...), state.approvalsReviewer, nil
}

func (m *codexAppServerManager) retainStartedThread(p *codexAppServerProcess, key codexThreadKey, thread codexThreadRecord, collaborationMode json.RawMessage, approvalsReviewer string) {
	select {
	case <-p.done:
		return
	default:
	}
	state := m.threadSubscription(key, p)
	state.mu.Lock()
	defer state.mu.Unlock()
	select {
	case <-p.done:
		// processExited may have removed and cleaned up this state before we
		// acquired its lock. Do not attach a timer to an already-dead process.
		return
	default:
	}
	if state.expired {
		// The app-server may have exited after the initial p.done check above.
		// threadSubscription returns an expired, unregistered state in that case;
		// do not revive it or schedule a timer that would retain the dead process.
		return
	}
	// thread/start subscribes the app-server connection. Keep it subscribed until
	// the first turn materializes the thread; a new thread cannot be resumed yet.
	state.subscribed = true
	state.materialized = false
	state.expired = false
	state.startedThread = &thread
	state.collaborationMode = append(json.RawMessage(nil), collaborationMode...)
	state.approvalsReviewer = approvalsReviewer
	m.scheduleUnmaterializedThreadExpiryLocked(p, key, state)
}

func (m *codexAppServerManager) startedThread(p *codexAppServerProcess, key codexThreadKey) (codexThreadRecord, bool) {
	m.threadSubMu.Lock()
	state := m.threadSubs[codexThreadSubscriptionKey{thread: key, process: p}]
	m.threadSubMu.Unlock()
	if state == nil {
		return codexThreadRecord{}, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.materialized || state.expired || state.startedThread == nil {
		return codexThreadRecord{}, false
	}
	return *state.startedThread, true
}

func (m *codexAppServerManager) unmaterializedThreads(p *codexAppServerProcess, project string) []codexThreadRecord {
	m.threadSubMu.Lock()
	states := make([]struct {
		key   codexThreadKey
		state *codexThreadSubscription
	}, 0)
	for subscriptionKey, state := range m.threadSubs {
		if subscriptionKey.process == p && subscriptionKey.thread.project == project {
			states = append(states, struct {
				key   codexThreadKey
				state *codexThreadSubscription
			}{key: subscriptionKey.thread, state: state})
		}
	}
	m.threadSubMu.Unlock()

	threads := make([]codexThreadRecord, 0, len(states))
	for _, entry := range states {
		entry.state.mu.Lock()
		if !entry.state.materialized && !entry.state.expired && entry.state.subscribed && entry.state.startedThread != nil {
			threads = append(threads, *entry.state.startedThread)
		}
		entry.state.mu.Unlock()
	}
	return threads
}

func (m *codexAppServerManager) updateCollaborationMode(p *codexAppServerProcess, key codexThreadKey, collaborationMode json.RawMessage) {
	m.threadSubMu.Lock()
	state := m.threadSubs[codexThreadSubscriptionKey{thread: key, process: p}]
	m.threadSubMu.Unlock()
	if state == nil {
		return
	}
	state.mu.Lock()
	state.collaborationMode = append(json.RawMessage(nil), collaborationMode...)
	state.mu.Unlock()
}

func (m *codexAppServerManager) currentCollaborationMode(p *codexAppServerProcess, key codexThreadKey) json.RawMessage {
	m.threadSubMu.Lock()
	state := m.threadSubs[codexThreadSubscriptionKey{thread: key, process: p}]
	m.threadSubMu.Unlock()
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return append(json.RawMessage(nil), state.collaborationMode...)
}

func (m *codexAppServerManager) updateApprovalsReviewer(p *codexAppServerProcess, key codexThreadKey, reviewer string) {
	m.threadSubMu.Lock()
	state := m.threadSubs[codexThreadSubscriptionKey{thread: key, process: p}]
	m.threadSubMu.Unlock()
	if state == nil {
		return
	}
	state.mu.Lock()
	state.approvalsReviewer = reviewer
	state.mu.Unlock()
}

func (m *codexAppServerManager) markThreadMaterialized(p *codexAppServerProcess, key codexThreadKey) {
	state := m.threadSubscription(key, p)
	state.mu.Lock()
	state.materialized = true
	state.expired = false
	if state.idleTimer != nil {
		state.idleTimer.Stop()
		state.idleTimer = nil
	}
	state.mu.Unlock()
}

func (m *codexAppServerManager) detachThread(p *codexAppServerProcess, key codexThreadKey) {
	state := m.threadSubscription(key, p)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.clients == 0 {
		return
	}
	state.clients--
	if state.clients > 0 || !state.subscribed {
		return
	}
	if !state.materialized {
		m.scheduleUnmaterializedThreadExpiryLocked(p, key, state)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), codexAppServerRequestTimeout)
	defer cancel()
	if _, err := p.request(ctx, "thread/unsubscribe", map[string]string{"threadId": key.thread}); err != nil {
		log.Printf("mybox: failed to unsubscribe Codex thread %s: %v", key.thread, err)
		state.unknown = true
		m.scheduleSubscriptionRetryLocked(p, key, state)
		return
	}
	state.subscribed = false
	state.unknown = false
}

// scheduleSubscriptionRetryLocked retries an uncertain unsubscribe while no
// browser client is attached. state.mu must be held.
func (m *codexAppServerManager) scheduleSubscriptionRetryLocked(p *codexAppServerProcess, key codexThreadKey, state *codexThreadSubscription) {
	if state.retryTimer != nil {
		state.retryTimer.Stop()
	}
	state.retryTimer = time.AfterFunc(codexSubscriptionRetryDelay, func() {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.retryTimer = nil
		if state.clients > 0 || !state.unknown || !state.subscribed {
			return
		}
		select {
		case <-p.done:
			state.subscribed = false
			state.unknown = false
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), codexAppServerRequestTimeout)
		defer cancel()
		if _, err := p.request(ctx, "thread/unsubscribe", map[string]string{"threadId": key.thread}); err != nil {
			log.Printf("mybox: failed to reconcile Codex thread subscription %s: %v", key.thread, err)
			select {
			case <-p.done:
				state.subscribed = false
				state.unknown = false
				return
			default:
			}
			m.scheduleSubscriptionRetryLocked(p, key, state)
			return
		}
		state.subscribed = false
		state.unknown = false
	})
}

func (m *codexAppServerManager) scheduleUnmaterializedThreadExpiryLocked(p *codexAppServerProcess, key codexThreadKey, state *codexThreadSubscription) {
	if state.idleTimer != nil {
		state.idleTimer.Stop()
	}
	state.idleTimer = time.AfterFunc(codexUnmaterializedThreadTTL, func() {
		state.mu.Lock()
		defer state.mu.Unlock()
		m.expireUnmaterializedThreadLocked(p, key, state)
	})
}

// expireUnmaterializedThreadLocked requires state.mu to be held.
func (m *codexAppServerManager) expireUnmaterializedThreadLocked(p *codexAppServerProcess, key codexThreadKey, state *codexThreadSubscription) {
	if state.clients > 0 || state.materialized || state.expired || !state.subscribed {
		return
	}
	select {
	case <-p.done:
		state.subscribed = false
		state.expired = true
		m.removeThreadSubscription(key, p, state)
		return
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), codexAppServerRequestTimeout)
	defer cancel()
	if _, err := p.request(ctx, "thread/unsubscribe", map[string]string{"threadId": key.thread}); err != nil {
		log.Printf("mybox: failed to release unused Codex thread %s: %v", key.thread, err)
		select {
		case <-p.done:
			state.subscribed = false
			state.expired = true
			m.removeThreadSubscription(key, p, state)
			return
		default:
		}
		m.scheduleUnmaterializedThreadExpiryLocked(p, key, state)
		return
	}
	state.subscribed = false
	state.expired = true
	state.idleTimer = nil
	m.removeThreadSubscription(key, p, state)
}

// removeThreadSubscription may be called while state.mu is held. Callers of
// threadSubscription release threadSubMu before taking state.mu, so this lock
// order does not form a cycle.
func (m *codexAppServerManager) removeThreadSubscription(key codexThreadKey, p *codexAppServerProcess, state *codexThreadSubscription) {
	m.threadSubMu.Lock()
	defer m.threadSubMu.Unlock()
	subscriptionKey := codexThreadSubscriptionKey{thread: key, process: p}
	if m.threadSubs[subscriptionKey] == state {
		delete(m.threadSubs, subscriptionKey)
	}
}

func (m *codexAppServerManager) ensure(ctx context.Context, projectDir string) (*codexAppServerProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("codex app-server is shutting down")
	}
	if p := m.process; p != nil {
		select {
		case <-p.done:
			m.process = nil
		default:
			m.mu.Unlock()
			if err := p.initialize(); err != nil {
				return nil, err
			}
			return p, nil
		}
	}
	bin, err := exec.LookPath("codex")
	if err != nil {
		m.mu.Unlock()
		return nil, errors.New("codex command not found on the mybox server PATH")
	}
	var p *codexAppServerProcess
	var stdout io.Reader
	if m.stdioTransport {
		cmd := exec.Command(bin, "app-server", "--listen", "stdio://")
		cmd.Dir = projectDir
		cmd.Stderr = io.Discard
		stdin, err := cmd.StdinPipe()
		if err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("prepare Codex app-server input: %w", err)
		}
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("prepare Codex app-server output: %w", err)
		}
		if err := cmd.Start(); err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("start Codex app-server: %w", err)
		}
		p = &codexAppServerProcess{cmd: cmd, stdin: stdin}
	} else {
		socketPath, err := startCodexAppServerDaemon(ctx, bin, projectDir)
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		conn, err := dialCodexAppServerSocket(ctx, socketPath)
		if err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("connect to persistent Codex app-server: %w", err)
		}
		p = &codexAppServerProcess{conn: conn}
	}
	p.done = make(chan struct{})
	p.pending = make(map[string]chan codexRPCResult)
	p.manager = m
	m.process = p
	m.mu.Unlock()
	if p.conn != nil {
		go p.readWebSocketLoop()
	} else {
		go p.readLoop(stdout)
	}
	if err := p.initialize(); err != nil {
		p.stop()
		return nil, err
	}
	return p, nil
}

func startCodexAppServerDaemon(ctx context.Context, bin, projectDir string) (string, error) {
	startCtx, cancel := context.WithTimeout(ctx, codexAppServerStartTimeout)
	defer cancel()
	start := exec.CommandContext(startCtx, bin, "app-server", "daemon", "start")
	start.Dir = projectDir
	start.Stdout = io.Discard
	start.Stderr = io.Discard
	if err := start.Run(); err != nil {
		return "", fmt.Errorf("start persistent Codex app-server daemon: %w", err)
	}

	version := exec.CommandContext(startCtx, bin, "app-server", "daemon", "version")
	version.Dir = projectDir
	output, err := version.Output()
	if err != nil {
		return "", fmt.Errorf("read Codex app-server daemon status: %w", err)
	}
	var status struct {
		Status     string `json:"status"`
		SocketPath string `json:"socketPath"`
	}
	if err := json.Unmarshal(output, &status); err != nil || status.Status != "running" || strings.TrimSpace(status.SocketPath) == "" {
		return "", errors.New("codex app-server daemon did not report a running control socket")
	}
	return status.SocketPath, nil
}

func dialCodexAppServerSocket(ctx context.Context, socketPath string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: codexAppServerStartTimeout,
		NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	conn, _, err := dialer.DialContext(ctx, "ws://localhost/", nil)
	return conn, err
}

func (p *codexAppServerProcess) initialize() error {
	p.initOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), codexAppServerStartTimeout)
		defer cancel()
		_, p.initErr = p.request(ctx, "initialize", map[string]any{
			"clientInfo": map[string]string{
				"name":    "mybox",
				"title":   "mybox",
				"version": "dev",
			},
			"capabilities": map[string]bool{
				"experimentalApi":    true,
				"requestAttestation": false,
			},
		})
		if p.initErr == nil {
			p.initErr = p.notify("initialized", map[string]any{})
		}
	})
	return p.initErr
}

func (p *codexAppServerProcess) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	select {
	case <-p.done:
		return nil, errors.New("codex app-server exited")
	default:
	}
	id := p.requestID.Add(1)
	key := strconv.FormatUint(id, 10)
	ch := make(chan codexRPCResult, 1)
	p.pendingMu.Lock()
	p.pending[key] = ch
	p.pendingMu.Unlock()
	if err := p.writeJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		p.pendingMu.Lock()
		delete(p.pending, key)
		p.pendingMu.Unlock()
		return nil, err
	}
	select {
	case result := <-ch:
		return result.result, result.err
	case <-ctx.Done():
		p.pendingMu.Lock()
		delete(p.pending, key)
		p.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-p.done:
		return nil, errors.New("codex app-server exited")
	}
}

func (p *codexAppServerProcess) notify(method string, params any) error {
	return p.writeJSON(map[string]any{"method": method, "params": params})
}

func (p *codexAppServerProcess) writeJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	select {
	case <-p.done:
		return errors.New("codex app-server exited")
	default:
	}
	if p.conn != nil {
		if err := p.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return fmt.Errorf("write to Codex app-server: %w", err)
		}
	} else {
		data = append(data, '\n')
		if _, err := p.stdin.Write(data); err != nil {
			return fmt.Errorf("write to Codex app-server: %w", err)
		}
	}
	return nil
}

func (p *codexAppServerProcess) writeServerResponse(id json.RawMessage, result any, rpcErr *codexRPCError) error {
	message := map[string]any{"id": id}
	if rpcErr != nil {
		message["error"] = rpcErr
	} else {
		message["result"] = result
	}
	return p.writeJSON(message)
}

func (p *codexAppServerProcess) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), codexAppServerMaxLine)
	for scanner.Scan() {
		p.handleRPCMessage(scanner.Bytes())
	}
	if scanner.Err() != nil && p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	if p.cmd != nil {
		_ = p.cmd.Wait()
	}
	p.finish()
}

func (p *codexAppServerProcess) readWebSocketLoop() {
	for {
		_, data, err := p.conn.ReadMessage()
		if err != nil {
			break
		}
		p.handleRPCMessage(data)
	}
	p.finish()
}

func (p *codexAppServerProcess) handleRPCMessage(data []byte) {
	var message codexRPCMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}
	if message.Method != "" {
		if len(message.ID) > 0 {
			p.manager.handleServerRequest(p, message)
		} else {
			p.manager.handleNotification(message)
		}
		return
	}
	if len(message.ID) == 0 {
		return
	}
	key := normalizeCodexRequestID(message.ID)
	p.pendingMu.Lock()
	ch := p.pending[key]
	delete(p.pending, key)
	p.pendingMu.Unlock()
	if ch == nil {
		return
	}
	if message.Error != nil {
		ch <- codexRPCResult{err: fmt.Errorf("codex app-server: %s", message.Error.Message)}
	} else {
		ch <- codexRPCResult{result: message.Result}
	}
}

func (p *codexAppServerProcess) finish() {
	p.doneOnce.Do(func() {
		close(p.done)
		p.pendingMu.Lock()
		for key, ch := range p.pending {
			delete(p.pending, key)
			ch <- codexRPCResult{err: errors.New("codex app-server exited")}
		}
		p.pendingMu.Unlock()
		p.manager.processExited(p)
	})
}

func (p *codexAppServerProcess) stop() {
	if p.conn != nil {
		_ = p.conn.Close()
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
		return
	}
	if p.stdin == nil {
		return
	}
	_ = p.stdin.Close()
	select {
	case <-p.done:
		return
	case <-time.After(2 * time.Second):
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	}
}

func normalizeCodexRequestID(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return strings.TrimSpace(string(raw))
}

func (m *codexAppServerManager) processExited(p *codexAppServerProcess) {
	m.mu.Lock()
	if m.process == p {
		m.process = nil
	}
	m.mu.Unlock()
	m.clientsMu.Lock()
	for requestID, request := range m.pendingRequests {
		if request.process == p {
			delete(m.pendingRequests, requestID)
		}
	}
	clients := make([]*codexStreamClient, 0)
	for _, group := range m.clients {
		for client := range group {
			if client.process == p {
				clients = append(clients, client)
			}
		}
	}
	m.clientsMu.Unlock()
	// The process pointer is part of each subscription key. Remove its states
	// on exit so restarts do not retain the dead process and its thread states.
	m.threadSubMu.Lock()
	threadStates := make([]*codexThreadSubscription, 0)
	for key, state := range m.threadSubs {
		if key.process == p {
			delete(m.threadSubs, key)
			threadStates = append(threadStates, state)
		}
	}
	m.threadSubMu.Unlock()
	for _, state := range threadStates {
		state.mu.Lock()
		if state.idleTimer != nil {
			state.idleTimer.Stop()
			state.idleTimer = nil
		}
		if state.retryTimer != nil {
			state.retryTimer.Stop()
			state.retryTimer = nil
		}
		state.subscribed = false
		state.unknown = false
		state.expired = true
		state.mu.Unlock()
	}
	for _, client := range clients {
		client.close()
	}
}

func (m *codexAppServerManager) Close() {
	m.mu.Lock()
	m.closed = true
	p := m.process
	m.process = nil
	m.mu.Unlock()
	if p != nil {
		// Close only this control-socket connection. The Codex daemon owns the
		// app-server and remains available across mybox restarts so active turns
		// can continue.
		p.stop()
	}
	m.threadSubMu.Lock()
	threadStates := make([]*codexThreadSubscription, 0, len(m.threadSubs))
	for _, state := range m.threadSubs {
		threadStates = append(threadStates, state)
	}
	m.threadSubMu.Unlock()
	for _, state := range threadStates {
		state.mu.Lock()
		if state.idleTimer != nil {
			state.idleTimer.Stop()
			state.idleTimer = nil
		}
		if state.retryTimer != nil {
			state.retryTimer.Stop()
			state.retryTimer = nil
		}
		state.mu.Unlock()
	}
	m.clientsMu.Lock()
	clients := make([]*codexStreamClient, 0)
	for _, group := range m.clients {
		for client := range group {
			clients = append(clients, client)
		}
	}
	m.clientsMu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func (m *codexAppServerManager) subscribe(key codexThreadKey, p *codexAppServerProcess, conn *websocket.Conn) *codexStreamClient {
	client := &codexStreamClient{key: key, process: p, conn: conn, send: make(chan []byte, 256), done: make(chan struct{}), manager: m}
	m.clientsMu.Lock()
	group := m.clients[key]
	if group == nil {
		group = make(map[*codexStreamClient]struct{})
		m.clients[key] = group
	}
	group[client] = struct{}{}
	pending := make([]codexPendingServerRequest, 0)
	for _, req := range m.pendingRequests {
		if req.project == key.project && req.thread == key.thread {
			pending = append(pending, req)
		}
	}
	m.clientsMu.Unlock()
	for _, req := range pending {
		client.push(map[string]any{"type": "server_request", "request_id": normalizeCodexRequestID(req.id), "method": req.method, "params": req.params})
	}
	return client
}

func (m *codexAppServerManager) unsubscribe(client *codexStreamClient) {
	m.clientsMu.Lock()
	delete(m.clients[client.key], client)
	if len(m.clients[client.key]) == 0 {
		delete(m.clients, client.key)
	}
	m.clientsMu.Unlock()
}

func (c *codexStreamClient) push(value any) bool {
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	select {
	case <-c.done:
		return false
	case c.send <- data:
		return true
	default:
		c.close()
		return false
	}
}

func (c *codexStreamClient) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.manager.unsubscribe(c)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (m *codexAppServerManager) broadcast(key codexThreadKey, value any) {
	m.clientsMu.Lock()
	clients := make([]*codexStreamClient, 0, len(m.clients[key]))
	for client := range m.clients[key] {
		clients = append(clients, client)
	}
	m.clientsMu.Unlock()
	for _, client := range clients {
		client.push(value)
	}
}

func (m *codexAppServerManager) handleNotification(message codexRPCMessage) {
	threadID := codexThreadIDFromParams(message.Params)
	if threadID == "" {
		return
	}
	if message.Method == "thread/settings/updated" {
		var params struct {
			ThreadSettings struct {
				ApprovalsReviewer string `json:"approvalsReviewer"`
			} `json:"threadSettings"`
		}
		if json.Unmarshal(message.Params, &params) == nil && isCodexApprovalsReviewer(params.ThreadSettings.ApprovalsReviewer) {
			m.updateApprovalsReviewerForThread(threadID, params.ThreadSettings.ApprovalsReviewer)
		}
	}
	m.broadcastThread(threadID, map[string]any{
		"type": "notification", "method": message.Method, "params": message.Params,
	})
}

func isCodexApprovalsReviewer(reviewer string) bool {
	return reviewer == "user" || reviewer == "auto_review" || reviewer == "guardian_subagent"
}

func (m *codexAppServerManager) updateApprovalsReviewerForThread(threadID, reviewer string) {
	m.threadSubMu.Lock()
	states := make([]*codexThreadSubscription, 0)
	for key, state := range m.threadSubs {
		if key.thread.thread == threadID {
			states = append(states, state)
		}
	}
	m.threadSubMu.Unlock()
	for _, state := range states {
		state.mu.Lock()
		state.approvalsReviewer = reviewer
		state.mu.Unlock()
	}
}

func (m *codexAppServerManager) broadcastThread(threadID string, value any) {
	m.clientsMu.Lock()
	clients := make([]*codexStreamClient, 0)
	for key, group := range m.clients {
		if key.thread != threadID {
			continue
		}
		for client := range group {
			clients = append(clients, client)
		}
	}
	m.clientsMu.Unlock()
	for _, client := range clients {
		client.push(value)
	}
}

func codexThreadIDFromParams(params json.RawMessage) string {
	var data map[string]json.RawMessage
	if json.Unmarshal(params, &data) != nil {
		return ""
	}
	var threadID string
	_ = json.Unmarshal(data["threadId"], &threadID)
	if threadID != "" {
		return threadID
	}
	var thread struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data["thread"], &thread)
	return thread.ID
}

func (m *codexAppServerManager) handleServerRequest(p *codexAppServerProcess, message codexRPCMessage) {
	threadID := codexThreadIDFromParams(message.Params)
	key := normalizeCodexRequestID(message.ID)
	if threadID == "" || !supportedCodexServerRequest(message.Method) {
		_ = p.writeServerResponse(message.ID, nil, &codexRPCError{Code: -32601, Message: "mybox does not support this Codex app-server request"})
		return
	}
	var project string
	m.clientsMu.Lock()
	for threadKey := range m.clients {
		if threadKey.thread == threadID {
			project = threadKey.project
			break
		}
	}
	if project != "" {
		m.pendingRequests[key] = codexPendingServerRequest{
			id: message.ID, method: message.Method, project: project, thread: threadID, params: message.Params, process: p,
		}
	}
	m.clientsMu.Unlock()
	if project == "" {
		_ = p.writeServerResponse(message.ID, nil, &codexRPCError{Code: -32601, Message: "no mybox client is attached to this Codex request"})
		return
	}
	m.broadcast(codexThreadKey{project: project, thread: threadID}, map[string]any{
		"type": "server_request", "request_id": key, "method": message.Method, "params": message.Params,
	})
}

func supportedCodexServerRequest(method string) bool {
	switch method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval", "item/tool/requestUserInput":
		return true
	default:
		return false
	}
}

func (m *codexAppServerManager) resolveServerRequest(project, thread, requestID, decision string, answers json.RawMessage) error {
	m.clientsMu.Lock()
	request, ok := m.pendingRequests[requestID]
	if !ok || request.project != project || request.thread != thread {
		m.clientsMu.Unlock()
		return errors.New("codex approval request is no longer pending")
	}
	if request.resolving {
		m.clientsMu.Unlock()
		return errors.New("codex approval request is already being answered")
	}
	request.resolving = true
	m.pendingRequests[requestID] = request
	m.clientsMu.Unlock()
	defer func() {
		m.clientsMu.Lock()
		if current, exists := m.pendingRequests[requestID]; exists && current.process == request.process {
			current.resolving = false
			m.pendingRequests[requestID] = current
		}
		m.clientsMu.Unlock()
	}()
	p := request.process
	if p == nil {
		return errors.New("codex app-server is not running")
	}
	var result any
	switch request.method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		if decision != "accept" && decision != "decline" && decision != "cancel" {
			return errors.New("unsupported Codex approval decision")
		}
		var requestParams struct {
			AvailableDecisions []string `json:"availableDecisions"`
		}
		_ = json.Unmarshal(request.params, &requestParams)
		if len(requestParams.AvailableDecisions) > 0 {
			available := false
			for _, candidate := range requestParams.AvailableDecisions {
				if candidate == decision {
					available = true
					break
				}
			}
			if !available {
				return errors.New("codex did not offer that approval decision")
			}
		}
		result = map[string]string{"decision": decision}
	case "item/permissions/requestApproval":
		if decision != "accept" && decision != "decline" {
			return errors.New("unsupported Codex permission decision")
		}
		var requestParams struct {
			Permissions json.RawMessage `json:"permissions"`
		}
		if err := json.Unmarshal(request.params, &requestParams); err != nil || len(requestParams.Permissions) == 0 {
			return errors.New("codex permission request is invalid")
		}
		permissions := json.RawMessage(`{}`)
		if decision == "accept" {
			permissions = requestParams.Permissions
		}
		result = map[string]any{"permissions": permissions, "scope": "turn"}
	case "item/tool/requestUserInput":
		var value map[string]json.RawMessage
		if len(answers) == 0 || json.Unmarshal(answers, &value) != nil || len(value) == 0 {
			return errors.New("answers are required for this Codex question")
		}
		result = map[string]json.RawMessage{"answers": answers}
	default:
		return errors.New("mybox cannot answer this Codex request type")
	}
	if err := p.writeServerResponse(request.id, result, nil); err != nil {
		return err
	}
	m.clientsMu.Lock()
	delete(m.pendingRequests, requestID)
	m.clientsMu.Unlock()
	return nil
}

func (s *Server) ListCodexThreads(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.codexAppServer.ensure(r.Context(), app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	threads, truncated, err := listCodexProjectThreads(ctx, p, app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	threads = mergeStartedCodexThreads(threads, s.codexAppServer.unmaterializedThreads(p, app.Project.Name))
	writeJSONResponse(w, http.StatusOK, api.CodexThreadList{Threads: threads, Truncated: truncated})
}

func mergeStartedCodexThreads(threads []api.CodexThread, started []codexThreadRecord) []api.CodexThread {
	seen := make(map[string]struct{}, len(threads)+len(started))
	for _, thread := range threads {
		seen[thread.ThreadId] = struct{}{}
	}
	for _, thread := range started {
		if thread.ID == "" {
			continue
		}
		if _, exists := seen[thread.ID]; exists {
			continue
		}
		threads = append(threads, summarizeCodexThread(thread))
		seen[thread.ID] = struct{}{}
	}
	sort.SliceStable(threads, func(i, j int) bool {
		return threads[i].UpdatedAt > threads[j].UpdatedAt
	})
	return threads
}

func (s *Server) ListCodexActiveSessions(w http.ResponseWriter, r *http.Request, params api.ListCodexActiveSessionsParams) {
	projects, err := s.projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	roots := codexProjectRoots(projects)
	if len(roots) == 0 {
		writeJSONResponse(w, http.StatusOK, api.CodexActiveSessionList{Sessions: []api.CodexActiveSession{}})
		return
	}

	s.mu.RLock()
	defaultProject := s.defaultProject
	s.mu.RUnlock()
	projectDir := roots[0].path
	for _, root := range roots {
		if root.name == defaultProject {
			projectDir = root.path
			break
		}
	}
	p, err := s.codexAppServer.ensure(r.Context(), projectDir)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	since := int64(0)
	if params.Since != nil {
		since = *params.Since
	}
	sessions, truncated, err := listCodexActiveSessions(ctx, p, roots, since)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.CodexActiveSessionList{Sessions: sessions, Truncated: truncated})
}

func (s *Server) CreateCodexThread(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.codexAppServer.ensure(r.Context(), app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	result, err := p.request(ctx, "thread/start", map[string]any{"cwd": app.Project.Path})
	if err != nil {
		writeError(w, err)
		return
	}
	var response struct {
		Thread            codexThreadRecord `json:"thread"`
		Model             string            `json:"model"`
		ReasoningEffort   *string           `json:"reasoningEffort"`
		ApprovalsReviewer string            `json:"approvalsReviewer"`
	}
	if err := json.Unmarshal(result, &response); err != nil || response.Thread.ID == "" {
		writeError(w, errors.New("codex app-server returned an invalid thread"))
		return
	}
	if response.Thread.CWD == "" {
		response.Thread.CWD = app.Project.Path
	}
	key := codexThreadKey{project: app.Project.Name, thread: response.Thread.ID}
	model := response.Model
	if model == "" && response.Thread.Model != nil {
		model = *response.Thread.Model
	}
	var collaborationMode json.RawMessage
	if model != "" {
		collaborationMode, _ = json.Marshal(map[string]any{
			"mode": "default",
			"settings": map[string]any{
				"model":                  model,
				"reasoning_effort":       response.ReasoningEffort,
				"developer_instructions": nil,
			},
		})
	}
	s.codexAppServer.retainStartedThread(p, key, response.Thread, collaborationMode, response.ApprovalsReviewer)
	writeJSONResponse(w, http.StatusCreated, summarizeCodexThread(response.Thread))
}

func (s *Server) GetCodexThreadHistory(w http.ResponseWriter, r *http.Request, id string, params api.GetCodexThreadHistoryParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.codexAppServer.ensure(r.Context(), app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	thread, err := readCodexThread(ctx, p, id, app.Project.Path)
	if err != nil {
		started, ok := s.codexAppServer.startedThread(p, codexThreadKey{project: app.Project.Name, thread: id})
		if !ok || started.ParentThreadID != nil || !codexPathWithinProject(app.Project.Path, started.CWD) {
			writeError(w, err)
			return
		}
		thread = started
	}
	limit := codexTurnPageSize
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
		if limit > 100 {
			limit = 100
		}
	}
	request := map[string]any{"threadId": id, "limit": limit, "sortDirection": "desc", "itemsView": "full"}
	if params.Cursor != nil && *params.Cursor != "" {
		request["cursor"] = *params.Cursor
	}
	page, err := p.request(ctx, "thread/turns/list", request)
	if err != nil {
		if isCodexThreadNotMaterialized(err) {
			writeJSONResponse(w, http.StatusOK, api.CodexThreadHistory{
				Thread: summarizeCodexThread(thread), Turns: []map[string]interface{}{},
			})
			return
		}
		writeError(w, err)
		return
	}
	var turns struct {
		Data       []map[string]interface{} `json:"data"`
		NextCursor *string                  `json:"nextCursor"`
	}
	if err := json.Unmarshal(page, &turns); err != nil {
		writeError(w, errors.New("codex app-server returned invalid conversation history"))
		return
	}
	for left, right := 0, len(turns.Data)-1; left < right; left, right = left+1, right-1 {
		turns.Data[left], turns.Data[right] = turns.Data[right], turns.Data[left]
	}
	writeJSONResponse(w, http.StatusOK, api.CodexThreadHistory{
		Thread: summarizeCodexThread(thread), Turns: turns.Data, NextCursor: turns.NextCursor,
	})
}

func (s *Server) DeleteCodexThread(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.codexAppServer.ensure(r.Context(), app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	if _, err := readCodexThread(ctx, p, id, app.Project.Path); err != nil {
		writeError(w, err)
		return
	}
	if _, err := p.request(ctx, "thread/delete", map[string]string{"threadId": id}); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) RenameCodexThread(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("invalid Codex conversation name")})
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len([]rune(name)) > 100 {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("Codex conversation name must contain 1 to 100 characters")})
		return
	}
	p, err := s.codexAppServer.ensure(r.Context(), app.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), codexAppServerRequestTimeout)
	defer cancel()
	if _, err := readCodexThread(ctx, p, id, app.Project.Path); err != nil {
		writeError(w, err)
		return
	}
	if _, err := p.request(ctx, "thread/name/set", map[string]string{"threadId": id, "name": name}); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) registerCodexAppServerRoutes(e *echo.Echo, basePath string) {
	e.GET(basePath+"/api/codex/threads/:id/stream", s.streamCodexThread)
}

func (s *Server) streamCodexThread(c echo.Context) error {
	project := c.QueryParam("project")
	var app *App
	var err error
	if project == "" {
		app, err = s.getApp(c.Request())
	} else {
		app, err = s.getAppByProject(c.Request().Context(), project)
	}
	if err != nil {
		return err
	}
	threadID := c.Param("id")
	p, err := s.codexAppServer.ensure(c.Request().Context(), app.Project.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}
	readCtx, cancelRead := context.WithTimeout(c.Request().Context(), codexAppServerRequestTimeout)
	_, readErr := readCodexThread(readCtx, p, threadID, app.Project.Path)
	cancelRead()
	if readErr != nil {
		started, ok := s.codexAppServer.startedThread(p, codexThreadKey{project: app.Project.Name, thread: threadID})
		if !ok || started.ParentThreadID != nil || !codexPathWithinProject(app.Project.Path, started.CWD) {
			return echo.NewHTTPError(http.StatusNotFound, "codex conversation not found in this project")
		}
	}
	conn, err := terminalUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	key := codexThreadKey{project: app.Project.Name, thread: threadID}
	client := s.codexAppServer.subscribe(key, p, conn)
	threadAttached := false
	defer func() {
		client.close()
		if threadAttached {
			s.codexAppServer.detachThread(p, key)
		}
	}()
	var writeMu sync.Mutex
	write := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, data)
	}
	go func() {
		for {
			select {
			case data := <-client.send:
				if err := write(websocket.TextMessage, data); err != nil {
					client.close()
					return
				}
			case <-client.done:
				return
			}
		}
	}()
	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-ticker.C:
				if err := write(websocket.PingMessage, nil); err != nil {
					client.close()
					return
				}
			}
		}
	}()
	conn.SetReadLimit(codexAppServerMaxClientMessageBytes)
	resumeCtx, cancelResume := context.WithTimeout(c.Request().Context(), codexAppServerRequestTimeout)
	var collaborationMode json.RawMessage
	var approvalsReviewer string
	collaborationMode, approvalsReviewer, err = s.codexAppServer.attachThread(resumeCtx, p, key)
	cancelResume()
	if err != nil {
		message := map[string]string{"type": "error", "error": err.Error()}
		if errors.Is(err, errCodexEmptyThreadExpired) || isCodexThreadNotResumable(err) {
			message = map[string]string{"type": "empty_thread_unavailable"}
		} else if isCodexActiveWriterConflict(err) {
			message = map[string]string{"type": "writer_conflict"}
		}
		data, _ := json.Marshal(message)
		_ = write(websocket.TextMessage, data)
		return nil
	}
	threadAttached = true
	client.push(map[string]any{"type": "ready", "collaboration_mode": collaborationMode, "approvals_reviewer": approvalsReviewer})
	for {
		_, data, readErr := conn.ReadMessage()
		if readErr != nil {
			return nil
		}
		var request codexClientMessage
		if err := json.Unmarshal(data, &request); err != nil {
			client.push(map[string]string{"type": "error", "error": "Invalid Codex control message"})
			continue
		}
		result, err := s.handleCodexClientMessageWithResult(c.Request().Context(), p, key, app.Project.Path, request)
		if err != nil {
			client.push(map[string]any{"type": "error", "error": err.Error(), "request_id": request.RequestID})
		} else if request.Type == "prompt" || request.Type == "plan_handoff" {
			client.push(map[string]string{"type": "prompt_started"})
		} else if request.Type == "respond" {
			client.push(map[string]any{"type": "response", "request_id": request.RequestID})
		} else {
			client.push(map[string]any{"type": "control_response", "control": request.Type, "request_id": request.RequestID, "result": result})
		}
	}
}

func isCodexActiveWriterConflict(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already has an active writer")
}

func isCodexThreadNotMaterialized(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not materialized yet")
}

func isCodexThreadNotResumable(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no rollout found for thread id")
}

type codexClientMessage struct {
	Type              string          `json:"type"`
	Text              string          `json:"text"`
	Mode              string          `json:"mode"`
	Objective         string          `json:"objective"`
	Status            string          `json:"status"`
	ApprovalsReviewer string          `json:"approvals_reviewer"`
	RequestID         string          `json:"request_id"`
	Decision          string          `json:"decision"`
	Answers           json.RawMessage `json:"answers"`
}

func (s *Server) handleCodexClientMessage(ctx context.Context, p *codexAppServerProcess, key codexThreadKey, request codexClientMessage) error {
	_, err := s.handleCodexClientMessageWithResult(ctx, p, key, "", request)
	return err
}

func (s *Server) handleCodexClientMessageWithResult(ctx context.Context, p *codexAppServerProcess, key codexThreadKey, projectDir string, request codexClientMessage) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancel()
	switch request.Type {
	case "prompt", "plan_handoff":
		text := strings.TrimSpace(request.Text)
		if request.Type == "plan_handoff" {
			if text == "" {
				return nil, errors.New("plan handoff must not be empty")
			}
			if len([]byte(text)) > codexAppServerMaxPlanPromptBytes {
				return nil, errors.New("plan handoff exceeds the 1 MiB message limit")
			}
		} else if text == "" || len([]rune(text)) > codexAppServerMaxPrompt {
			return nil, errors.New("prompt must contain between 1 and 10000 characters")
		}
		latest, err := latestCodexTurn(ctx, p, key.thread)
		if err != nil {
			return nil, err
		}
		input := []map[string]string{{"type": "text", "text": text}}
		if latest.Status == "inProgress" {
			_, err = p.request(ctx, "turn/steer", map[string]any{"threadId": key.thread, "expectedTurnId": latest.ID, "input": input})
		} else {
			_, err = p.request(ctx, "turn/start", map[string]any{"threadId": key.thread, "input": input})
			if err == nil {
				s.codexAppServer.markThreadMaterialized(p, key)
			}
		}
		return nil, err
	case "interrupt":
		latest, err := latestCodexTurn(ctx, p, key.thread)
		if err != nil {
			return nil, err
		}
		if latest.ID == "" || latest.Status != "inProgress" {
			return nil, errors.New("there is no active Codex turn to stop")
		}
		_, err = p.request(ctx, "turn/interrupt", map[string]string{"threadId": key.thread, "turnId": latest.ID})
		return nil, err
	case "compact":
		_, err := p.request(ctx, "thread/compact/start", map[string]string{"threadId": key.thread})
		return nil, err
	case "respond":
		return nil, s.codexAppServer.resolveServerRequest(key.project, key.thread, request.RequestID, request.Decision, request.Answers)
	case "set_mode":
		mode := strings.TrimSpace(request.Mode)
		if mode != "plan" && mode != "default" {
			return nil, errors.New("Codex mode must be plan or default")
		}
		thread, err := readCodexThread(ctx, p, key.thread, projectDir)
		if err != nil {
			return nil, err
		}
		model := ""
		if thread.Model != nil {
			model = strings.TrimSpace(*thread.Model)
		}
		if model == "" {
			var current struct {
				Settings struct {
					Model string `json:"model"`
				} `json:"settings"`
			}
			_ = json.Unmarshal(s.codexAppServer.currentCollaborationMode(p, key), &current)
			model = strings.TrimSpace(current.Settings.Model)
		}
		if model == "" {
			return nil, errors.New("Codex thread model is unavailable")
		}
		presetResult, err := p.request(ctx, "collaborationMode/list", map[string]any{})
		if err != nil {
			return nil, err
		}
		var presets struct {
			Data []struct {
				Mode            *string `json:"mode"`
				Model           *string `json:"model"`
				ReasoningEffort *string `json:"reasoning_effort"`
			} `json:"data"`
		}
		if err := json.Unmarshal(presetResult, &presets); err != nil {
			return nil, errors.New("Codex app-server returned invalid collaboration modes")
		}
		var reasoningEffort *string
		presetFound := false
		for _, preset := range presets.Data {
			if preset.Mode == nil || *preset.Mode != mode {
				continue
			}
			if preset.Model != nil && strings.TrimSpace(*preset.Model) != "" {
				model = strings.TrimSpace(*preset.Model)
			}
			reasoningEffort = preset.ReasoningEffort
			presetFound = true
			break
		}
		if !presetFound {
			return nil, fmt.Errorf("Codex mode %q is unavailable", mode)
		}
		collaborationMode, err := json.Marshal(map[string]any{
			"mode": mode,
			"settings": map[string]any{
				"model":                  model,
				"reasoning_effort":       reasoningEffort,
				"developer_instructions": nil,
			},
		})
		if err != nil {
			return nil, err
		}
		if _, err := p.request(ctx, "thread/settings/update", map[string]any{"threadId": key.thread, "collaborationMode": json.RawMessage(collaborationMode)}); err != nil {
			return nil, err
		}
		s.codexAppServer.updateCollaborationMode(p, key, collaborationMode)
		return map[string]any{"mode": mode, "collaboration_mode": json.RawMessage(collaborationMode)}, nil
	case "set_approvals_reviewer":
		reviewer := strings.TrimSpace(request.ApprovalsReviewer)
		if !isCodexApprovalsReviewer(reviewer) {
			return nil, errors.New("unsupported Codex approval reviewer")
		}
		if _, err := p.request(ctx, "thread/settings/update", map[string]any{"threadId": key.thread, "approvalsReviewer": reviewer}); err != nil {
			return nil, err
		}
		s.codexAppServer.updateApprovalsReviewer(p, key, reviewer)
		return map[string]string{"approvals_reviewer": reviewer}, nil
	case "goal_get":
		result, err := p.request(ctx, "thread/goal/get", map[string]string{"threadId": key.thread})
		return json.RawMessage(result), err
	case "goal_set":
		objective := strings.TrimSpace(request.Objective)
		if objective == "" || len([]rune(objective)) > codexAppServerMaxPrompt {
			return nil, errors.New("goal must contain between 1 and 10000 characters")
		}
		result, err := p.request(ctx, "thread/goal/set", map[string]any{
			"threadId":  key.thread,
			"origin":    "user",
			"objective": objective,
			"status":    "active",
		})
		return json.RawMessage(result), err
	case "goal_status":
		status := strings.TrimSpace(request.Status)
		if status != "active" && status != "paused" {
			return nil, errors.New("goal status must be active or paused")
		}
		result, err := p.request(ctx, "thread/goal/set", map[string]any{"threadId": key.thread, "origin": "user", "status": status})
		return json.RawMessage(result), err
	case "goal_clear":
		result, err := p.request(ctx, "thread/goal/clear", map[string]any{"threadId": key.thread, "origin": "user"})
		return json.RawMessage(result), err
	default:
		return nil, errors.New("unsupported Codex control message")
	}
}

type codexLatestTurn struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func latestCodexTurn(ctx context.Context, p *codexAppServerProcess, threadID string) (codexLatestTurn, error) {
	result, err := p.request(ctx, "thread/turns/list", map[string]any{
		"threadId": threadID, "limit": 1, "sortDirection": "desc", "itemsView": "notLoaded",
	})
	if err != nil {
		if isCodexThreadNotMaterialized(err) {
			return codexLatestTurn{}, nil
		}
		return codexLatestTurn{}, err
	}
	var response struct {
		Data []codexLatestTurn `json:"data"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return codexLatestTurn{}, errors.New("codex app-server returned invalid turn state")
	}
	if len(response.Data) == 0 {
		return codexLatestTurn{}, nil
	}
	return response.Data[0], nil
}

func listCodexProjectThreads(ctx context.Context, p *codexAppServerProcess, projectDir string) ([]api.CodexThread, bool, error) {
	threads := make([]api.CodexThread, 0)
	cursor := ""
	scanned := 0
	for {
		params := map[string]any{"limit": codexThreadPageSize, "sortKey": "updated_at", "sortDirection": "desc", "archived": false}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := p.request(ctx, "thread/list", params)
		if err != nil {
			return nil, false, err
		}
		var page struct {
			Data       []codexThreadRecord `json:"data"`
			NextCursor *string             `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &page); err != nil {
			return nil, false, errors.New("codex app-server returned invalid thread list")
		}
		for _, thread := range page.Data {
			scanned++
			if thread.ID == "" || thread.ParentThreadID != nil || !codexPathWithinProject(projectDir, thread.CWD) {
				continue
			}
			// An untouched thread has no history to reopen and cannot be resumed
			// after its app-server subscription is released.
			if (thread.Name == nil || strings.TrimSpace(*thread.Name) == "") && strings.TrimSpace(thread.Preview) == "" {
				continue
			}
			threads = append(threads, summarizeCodexThread(thread))
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return threads, false, nil
		}
		if scanned >= codexThreadScanLimit {
			return threads, true, nil
		}
		cursor = *page.NextCursor
	}
}

func readCodexThread(ctx context.Context, p *codexAppServerProcess, threadID, projectDir string) (codexThreadRecord, error) {
	if strings.TrimSpace(threadID) == "" || len(threadID) > 128 {
		return codexThreadRecord{}, &httpError{status: http.StatusBadRequest, err: errors.New("invalid Codex thread id")}
	}
	result, err := p.request(ctx, "thread/read", map[string]any{"threadId": threadID, "includeTurns": false})
	if err != nil {
		return codexThreadRecord{}, &httpError{status: http.StatusNotFound, err: errors.New("codex conversation not found")}
	}
	var response struct {
		Thread codexThreadRecord `json:"thread"`
	}
	if err := json.Unmarshal(result, &response); err != nil || response.Thread.ID == "" {
		return codexThreadRecord{}, &httpError{status: http.StatusNotFound, err: errors.New("codex conversation not found")}
	}
	if response.Thread.ParentThreadID != nil || !codexPathWithinProject(projectDir, response.Thread.CWD) {
		return codexThreadRecord{}, &httpError{status: http.StatusNotFound, err: errors.New("codex conversation not found in this project")}
	}
	return response.Thread, nil
}

func codexPathWithinProject(projectDir, candidate string) bool {
	if projectDir == "" || candidate == "" {
		return false
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	path, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

type codexProjectRoot struct {
	name string
	path string
}

func codexProjectRoots(projects []domain.Project) []codexProjectRoot {
	roots := make([]codexProjectRoot, 0, len(projects))
	for _, project := range projects {
		path := resolveCodexPath(project.Path)
		if path == "" {
			continue
		}
		roots = append(roots, codexProjectRoot{name: project.Name, path: path})
	}
	sort.Slice(roots, func(i, j int) bool {
		if len(roots[i].path) == len(roots[j].path) {
			return roots[i].name < roots[j].name
		}
		return len(roots[i].path) > len(roots[j].path)
	})
	return roots
}

func resolveCodexPath(path string) string {
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return ""
	}
	return filepath.Clean(resolved)
}

func codexProjectForPath(roots []codexProjectRoot, candidate string) string {
	path := resolveCodexPath(candidate)
	if path == "" {
		return ""
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root.path, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return root.name
		}
	}
	return ""
}

func listCodexActiveSessions(ctx context.Context, p *codexAppServerProcess, roots []codexProjectRoot, since int64) ([]api.CodexActiveSession, bool, error) {
	sessions := make([]api.CodexActiveSession, 0)
	cursor := ""
	scanned := 0
	for {
		params := map[string]any{"limit": codexThreadPageSize, "sortKey": "updated_at", "sortDirection": "desc", "archived": false}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := p.request(ctx, "thread/list", params)
		if err != nil {
			return nil, false, err
		}
		var page struct {
			Data       []codexThreadRecord `json:"data"`
			NextCursor *string             `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &page); err != nil {
			return nil, false, errors.New("codex app-server returned invalid thread list")
		}
		for _, thread := range page.Data {
			scanned++
			if thread.ID == "" || thread.ParentThreadID != nil {
				continue
			}
			project := codexProjectForPath(roots, thread.CWD)
			if project == "" {
				continue
			}
			codexThread := summarizeCodexThread(thread)
			if isCodexActiveStatus(codexThread.Status) {
				sessions = append(sessions, api.CodexActiveSession{Project: project, Thread: codexThread})
				continue
			}
			if since <= 0 || thread.UpdatedAt < since || !isCodexRecentTerminalStatus(codexThread.Status, thread.Preview) {
				continue
			}
			if normalizeCodexThreadStatus(codexThread.Status) == "idle" {
				codexThread.Status = "completed"
			}
			sessions = append(sessions, api.CodexActiveSession{Project: project, Thread: codexThread})
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return sessions, false, nil
		}
		if scanned >= codexThreadScanLimit {
			return sessions, true, nil
		}
		cursor = *page.NextCursor
	}
}

func isCodexActiveStatus(status string) bool {
	normalized := normalizeCodexThreadStatus(status)
	switch normalized {
	case "active", "inprogress", "running", "blocked", "waitingonapproval", "needsapproval", "approvalrequired":
		return true
	default:
		return false
	}
}

func isCodexRecentTerminalStatus(status, preview string) bool {
	switch normalizeCodexThreadStatus(status) {
	case "idle":
		return strings.TrimSpace(preview) != ""
	case "completed", "complete", "done", "systemerror", "failed", "error", "interrupted", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func normalizeCodexThreadStatus(status string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "").Replace(status))
}

func summarizeCodexThread(thread codexThreadRecord) api.CodexThread {
	title := "New Codex conversation"
	if thread.Name != nil && strings.TrimSpace(*thread.Name) != "" {
		title = strings.TrimSpace(*thread.Name)
	} else if strings.TrimSpace(thread.Preview) != "" {
		title = strings.TrimSpace(thread.Preview)
	}
	if len([]rune(title)) > 96 {
		title = string([]rune(title)[:93]) + "…"
	}
	status := "idle"
	var statusValue struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(thread.Status, &statusValue) == nil && statusValue.Type != "" {
		status = statusValue.Type
	} else {
		var text string
		if json.Unmarshal(thread.Status, &text) == nil && text != "" {
			status = text
		}
	}
	model := ""
	if thread.Model != nil {
		model = *thread.Model
	}
	return api.CodexThread{ThreadId: thread.ID, Title: title, Preview: thread.Preview, CreatedAt: thread.CreatedAt, UpdatedAt: thread.UpdatedAt, Status: status, Model: model}
}
