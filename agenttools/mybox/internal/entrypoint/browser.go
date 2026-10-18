package entrypoint

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
)

const (
	browserWorkerReadyTimeout                      = 10 * time.Second
	browserSessionCancelTimeout                    = 5 * time.Second
	browserCommandTimeoutDefault                   = 30 * time.Second
	browserWorkerMaxDefault                        = 4
	browserSessionIdleTimeoutDefault time.Duration = 0
	browserSessionReaperMinInterval                = 100 * time.Millisecond
	browserApprovalModeManual                      = "manual"
	browserApprovalModeAutoSafe                    = "auto_safe"
)

var browserCommandTimeout = browserCommandTimeoutFromEnvironment()

func browserCommandTimeoutFromEnvironment() time.Duration {
	raw := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_COMMAND_TIMEOUT"))
	if raw == "" {
		return browserCommandTimeoutDefault
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return browserCommandTimeoutDefault
	}
	return duration
}

type browserSessionPolicy struct {
	maxWorkers  int
	idleTimeout time.Duration
}

func browserSessionPolicyFromEnvironment() browserSessionPolicy {
	maxWorkers := browserWorkerMaxDefault
	if raw := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_MAX_WORKERS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			maxWorkers = parsed
		}
	}
	idleTimeout := browserSessionIdleTimeoutDefault
	if raw := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_IDLE_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed >= 0 {
			idleTimeout = parsed
		}
	}
	return browserSessionPolicy{maxWorkers: maxWorkers, idleTimeout: idleTimeout}
}

var (
	errBrowserCommandBusy       = errors.New("browser command is already in progress")
	errBrowserApprovalPending   = errors.New("browser approval is pending")
	errBrowserWorkerUnavailable = errors.New("browser worker is not running")
	errBrowserWorkerFailed      = errors.New("browser worker disconnected; restart the browser session")
	errBrowserWorkerLimit       = errors.New("browser worker limit reached")
)

var browserUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 1 << 20,
	CheckOrigin:     sameOrigin,
}

type browserSession struct {
	hub     *browserSessionHub
	project string
	id      string

	mu              sync.Mutex
	commandMu       chan struct{}
	state           string
	owner           string
	approvalMode    string
	url             string
	pageTitle       string
	latestFrame     []byte
	pendingApproval *browserApproval
	pending         map[string]chan browserWorkerResult
	workerRequestID string
	clients         map[*browserClient]struct{}
	closed          bool
	lastError       string
	lastActivity    time.Time
	worker          *browserWorker
}

type browserApproval struct {
	ID     string `json:"approval_id"`
	Action string `json:"action"`
	Target string `json:"target"`
}

type browserClient struct {
	session *browserSession
	send    chan browserStreamMessage
	stop    chan struct{}
	once    sync.Once
}

type browserStreamMessage struct {
	messageType int
	data        []byte
}

type browserSessionHub struct {
	mu          sync.Mutex
	sessions    map[string]*browserSession
	workers     map[string]*browserWorker
	workerStart map[string]*browserWorkerStart
	projectOps  map[string]*browserProjectOperation
	policy      browserSessionPolicy
	auditStore  domain.BrowserAuditStore
	auditReader domain.BrowserAuditReader
	stopCh      chan struct{}
	stopOnce    sync.Once
}

type browserProjectOperation struct {
	mu         sync.Mutex
	references int
}

type browserWorkerStart struct {
	done   chan struct{}
	worker *browserWorker
	err    error
}

type browserWorker struct {
	cmd     *exec.Cmd
	conn    *websocket.Conn
	pidFile string
	project string
	writeMu sync.Mutex
	hub     *browserSessionHub
	stop    chan struct{}
}

type browserWorkerMessage struct {
	Type       string          `json:"type"`
	SessionID  string          `json:"session_id,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	Command    string          `json:"command,omitempty"`
	State      string          `json:"state,omitempty"`
	URL        string          `json:"url,omitempty"`
	Title      string          `json:"title,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Error      string          `json:"error,omitempty"`
	Message    string          `json:"message,omitempty"`
	ApprovalID string          `json:"approval_id,omitempty"`
	Action     string          `json:"action,omitempty"`
	Target     string          `json:"target,omitempty"`
}

type browserWorkerResult struct {
	data json.RawMessage
	err  error
}

type browserCommandRequest struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type browserApprovalModeRequest struct {
	Mode string `json:"mode"`
}

type browserSessionRequest struct {
	Project string `json:"project,omitempty"`
	URL     string `json:"url,omitempty"`
}

type browserProfileResetRequest struct {
	Project string `json:"project,omitempty"`
}

func newBrowserSessionHub() *browserSessionHub {
	return newBrowserSessionHubWithPolicy(browserSessionPolicyFromEnvironment())
}

func newBrowserSessionHubWithPolicy(policy browserSessionPolicy) *browserSessionHub {
	return newBrowserSessionHubWithAudit(policy, nil)
}

func newBrowserSessionHubWithAudit(policy browserSessionPolicy, auditStore domain.BrowserAuditStore) *browserSessionHub {
	if policy.maxWorkers < 0 {
		policy.maxWorkers = browserWorkerMaxDefault
	}
	if policy.idleTimeout < 0 {
		policy.idleTimeout = browserSessionIdleTimeoutDefault
	}
	var auditReader domain.BrowserAuditReader
	if reader, ok := auditStore.(domain.BrowserAuditReader); ok {
		auditReader = reader
	}
	hub := &browserSessionHub{
		sessions:    make(map[string]*browserSession),
		workers:     make(map[string]*browserWorker),
		workerStart: make(map[string]*browserWorkerStart),
		projectOps:  make(map[string]*browserProjectOperation),
		policy:      policy,
		auditStore:  auditStore,
		auditReader: auditReader,
		stopCh:      make(chan struct{}),
	}
	if policy.idleTimeout > 0 {
		go hub.reapIdleSessions()
	}
	return hub
}

func (h *browserSessionHub) lockProjectOperation(project string) func() {
	h.mu.Lock()
	if h.projectOps == nil {
		h.projectOps = make(map[string]*browserProjectOperation)
	}
	operation := h.projectOps[project]
	if operation == nil {
		operation = &browserProjectOperation{}
		h.projectOps[project] = operation
	}
	operation.references++
	h.mu.Unlock()

	operation.mu.Lock()
	return func() {
		operation.mu.Unlock()
		h.mu.Lock()
		operation.references--
		if operation.references == 0 && h.projectOps[project] == operation {
			delete(h.projectOps, project)
		}
		h.mu.Unlock()
	}
}

func (h *browserSessionHub) resetProfile(project string, removeProfile func(string) error) error {
	unlock := h.lockProjectOperation(project)
	defer unlock()
	if h.hasProject(project) {
		return &httpError{status: http.StatusConflict, err: errors.New("close the browser session before resetting its profile")}
	}
	return removeProfile(config.BrowserProfileDir(project))
}

func (h *browserSessionHub) recordAudit(event domain.BrowserAuditEvent) {
	if h.auditStore == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	_ = h.auditStore.Append(context.Background(), event)
}

func (h *browserSessionHub) auditCommand(session *browserSession, actor, command string, data json.RawMessage, err error) {
	if command == "input" && err == nil {
		return
	}
	result, reason := browserAuditOutcome(data, err)
	h.recordAudit(domain.BrowserAuditEvent{
		Event:     "browser_command",
		Project:   session.project,
		SessionID: session.id,
		Actor:     actor,
		Command:   browserAuditCommandName(command),
		Result:    result,
		Reason:    reason,
	})
}

func browserAuditCommandName(command string) string {
	switch command {
	case "observe", "navigate", "click", "fill", "press", "scroll", "wait", "screenshot", "input", "close_session", "approve_pending", "reject_pending", "set_approval_mode":
		return command
	default:
		return "unknown"
	}
}

func browserAuditOutcome(data json.RawMessage, err error) (string, string) {
	if err != nil {
		message := err.Error()
		switch {
		case strings.Contains(message, "sensitive fields"):
			return "blocked", "sensitive_field"
		case strings.Contains(message, "user-only"):
			return "blocked", "user_only"
		case errors.Is(err, errBrowserApprovalPending):
			return "blocked", "approval_pending"
		case errors.Is(err, errBrowserWorkerFailed):
			return "error", "worker_disconnected"
		default:
			return "error", "operation_failed"
		}
	}
	var response struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &response) == nil && response.Status == "approval_required" {
		return "approval_required", "user_approval_required"
	}
	return "success", ""
}

func (h *browserSessionHub) stop() {
	h.stopOnce.Do(func() { close(h.stopCh) })
	h.mu.Lock()
	workers := make([]*browserWorker, 0, len(h.workers))
	for project, worker := range h.workers {
		workers = append(workers, worker)
		delete(h.workers, project)
	}
	for _, session := range h.sessions {
		session.mu.Lock()
		session.closed = true
		session.state = "closed"
		session.mu.Unlock()
	}
	h.mu.Unlock()
	for _, worker := range workers {
		worker.close()
	}
}

func (h *browserSessionHub) reapIdleSessions() {
	interval := h.policy.idleTimeout / 2
	if interval < browserSessionReaperMinInterval {
		interval = browserSessionReaperMinInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			h.expireIdleSessions(now)
		case <-h.stopCh:
			return
		}
	}
}

func (h *browserSessionHub) expireIdleSessions(now time.Time) int {
	h.mu.Lock()
	sessions := make([]*browserSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		sessions = append(sessions, session)
	}
	h.mu.Unlock()

	expired := 0
	for _, session := range sessions {
		if !session.tryAcquireCommand() {
			continue
		}
		if !session.isIdle(now, h.policy.idleTimeout) {
			session.releaseCommand()
			continue
		}
		session.broadcastJSON(map[string]any{
			"type":   "expired",
			"state":  "closed",
			"reason": "idle_timeout",
		})
		h.recordAudit(domain.BrowserAuditEvent{
			Event: "session_expired", Project: session.project, SessionID: session.id,
			Actor: "system", Result: "success", Reason: "idle_timeout",
		})
		h.remove(session)
		session.releaseCommand()
		expired++
	}
	return expired
}

func (s *Server) BrowserSessions(w http.ResponseWriter, r *http.Request) {
	if err := authorizeBrowserRequest(r, browserActor(r), false); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	project := browserProject(r, "")
	if project == "" {
		writeErrorStatus(w, http.StatusBadRequest, errors.New("browser project is required"))
		return
	}
	s.browser.mu.Lock()
	var session *browserSession
	for _, candidate := range s.browser.sessions {
		if candidate.project == project && !candidate.isClosed() {
			session = candidate
			break
		}
	}
	s.browser.mu.Unlock()
	if session == nil {
		writeJSONResponse(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	session.touch()
	writeJSONResponse(w, http.StatusOK, map[string]any{"session": session.snapshot()})
}

func (s *Server) CreateBrowserSession(w http.ResponseWriter, r *http.Request) {
	var req browserSessionRequest
	if r.ContentLength != 0 && !decodeBody(w, r, &req) {
		return
	}
	project := browserProject(r, req.Project)
	if project == "" {
		writeError(w, errors.New("browser project is required"))
		return
	}
	actor := browserActor(r)
	if err := authorizeBrowserRequest(r, actor, false); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	if err := validateBrowserInitialURL(actor, req.URL); err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.getAppWithProject(r.Context(), project); err != nil {
		writeError(w, err)
		return
	}
	session, created, err := s.browser.create(r.Context(), project, req.URL, actor)
	if err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{
			Event: "session_create", Project: project, Actor: actor, Result: "error", Reason: "operation_failed",
		})
		writeError(w, err)
		return
	}
	if session.stateValue() == "error" {
		s.browser.recordAudit(domain.BrowserAuditEvent{
			Event: "session_create", Project: project, SessionID: session.id, Actor: actor,
			Result: "blocked", Reason: "worker_disconnected",
		})
		writeErrorStatus(w, http.StatusConflict, errBrowserWorkerFailed)
		return
	}
	event := "session_reused"
	if created {
		event = "session_created"
	}
	s.browser.recordAudit(domain.BrowserAuditEvent{
		Event: event, Project: project, SessionID: session.id, Actor: actor, Result: "success",
	})
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSONResponse(w, status, session.snapshot())
}

func (s *Server) BrowserCommand(w http.ResponseWriter, r *http.Request) {
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	actor := browserActor(r)
	if err := authorizeBrowserRequest(r, actor, false); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	var req browserCommandRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		writeError(w, errors.New("browser command is required"))
		return
	}
	if err := authorizeBrowserCommand(actor, req.Command); err != nil {
		s.browser.auditCommand(session, actor, req.Command, nil, err)
		writeError(w, err)
		return
	}
	data, err := session.command(r.Context(), actor, req.Command, req.Payload)
	s.browser.auditCommand(session, actor, req.Command, data, err)
	if err != nil {
		if errors.Is(err, errBrowserCommandBusy) || errors.Is(err, errBrowserApprovalPending) {
			writeErrorStatus(w, http.StatusConflict, err)
			return
		}
		writeError(w, err)
		return
	}
	writeJSONRaw(w, http.StatusOK, data)
}

func (s *Server) BrowserApprovalMode(w http.ResponseWriter, r *http.Request) {
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	actor := browserActor(r)
	if err := authorizeBrowserRequest(r, actor, true); err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "approval_mode_changed", Project: session.project, SessionID: session.id, Actor: actor, Result: "blocked", Reason: "authorization"})
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	var req browserApprovalModeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Mode != browserApprovalModeManual && req.Mode != browserApprovalModeAutoSafe {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("browser approval mode must be manual or auto_safe")})
		return
	}
	payload, _ := json.Marshal(req)
	data, err := session.commandWithoutOwner(r.Context(), "user", "set_approval_mode", payload)
	s.browser.auditCommand(session, "user", "set_approval_mode", data, err)
	if err != nil {
		if errors.Is(err, errBrowserCommandBusy) {
			writeErrorStatus(w, http.StatusConflict, err)
			return
		}
		writeError(w, err)
		return
	}
	session.setApprovalMode(req.Mode)
	s.browser.recordAudit(domain.BrowserAuditEvent{Event: "approval_mode_changed", Project: session.project, SessionID: session.id, Actor: "user", Result: "success", Reason: req.Mode})
	writeJSONResponse(w, http.StatusOK, session.snapshot())
}

func (s *Server) BrowserTakeover(w http.ResponseWriter, r *http.Request) {
	s.browserControl(w, r, "user")
}

func (s *Server) BrowserReturnControl(w http.ResponseWriter, r *http.Request) {
	s.browserControl(w, r, "agent")
}

func (s *Server) browserControl(w http.ResponseWriter, r *http.Request, owner string) {
	event := "control_return"
	if owner == "user" {
		event = "control_takeover"
	}
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	session.touch()
	actor := browserActor(r)
	if err := authorizeBrowserRequest(r, actor, true); err != nil {
		if session != nil {
			s.browser.recordAudit(domain.BrowserAuditEvent{Event: event, Project: session.project, SessionID: session.id, Actor: actor, Result: "blocked", Reason: "authorization"})
		}
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	if !session.tryAcquireCommand() {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: event, Project: session.project, SessionID: session.id, Actor: actor, Result: "blocked", Reason: "command_busy"})
		writeErrorStatus(w, http.StatusConflict, errBrowserCommandBusy)
		return
	}
	defer session.releaseCommand()
	if err := session.setOwner(owner); err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: event, Project: session.project, SessionID: session.id, Actor: actor, Result: "error", Reason: "operation_failed"})
		writeError(w, err)
		return
	}
	s.browser.recordAudit(domain.BrowserAuditEvent{Event: event, Project: session.project, SessionID: session.id, Actor: actor, Result: "success"})
	writeJSONResponse(w, http.StatusOK, session.snapshot())
}

func (s *Server) BrowserApproval(w http.ResponseWriter, r *http.Request) {
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	session.touch()
	if err := authorizeBrowserRequest(r, browserActor(r), true); err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "approval_action", Project: session.project, SessionID: session.id, Actor: browserActor(r), Result: "blocked", Reason: "authorization"})
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	id := browserApprovalID(r.URL.Path)
	command := "reject_pending"
	if strings.HasSuffix(r.URL.Path, "/approve") {
		command = "approve_pending"
	}
	payload, _ := json.Marshal(map[string]string{"approval_id": id})
	data, err := session.commandWithoutOwner(r.Context(), "user", command, payload)
	s.browser.auditCommand(session, "user", command, data, err)
	if err != nil {
		if errors.Is(err, errBrowserCommandBusy) {
			writeErrorStatus(w, http.StatusConflict, err)
			return
		}
		writeError(w, err)
		return
	}
	session.clearApproval(id)
	event := "approval_rejected"
	if command == "approve_pending" {
		event = "approval_approved"
	}
	s.browser.recordAudit(domain.BrowserAuditEvent{Event: event, Project: session.project, SessionID: session.id, Actor: "user", Result: "success"})
	writeJSONRaw(w, http.StatusOK, data)
}

func (s *Server) BrowserCloseSession(w http.ResponseWriter, r *http.Request) {
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	actor := browserActor(r)
	if err := authorizeBrowserRequest(r, actor, false); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	if actor != "user" && session.ownerValue() == "user" {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_close", Project: session.project, SessionID: session.id, Actor: actor, Result: "blocked", Reason: "user_control"})
		writeErrorStatus(w, http.StatusConflict, errors.New("browser session is under user control"))
		return
	}
	if session.stateValue() == "error" {
		s.browser.remove(session)
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_closed", Project: session.project, SessionID: session.id, Actor: actor, Result: "success", Reason: "worker_failed"})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, err := session.commandWithoutOwner(r.Context(), actor, "close_session", nil)
	s.browser.auditCommand(session, actor, "close_session", nil, err)
	if err != nil {
		if errors.Is(err, errBrowserCommandBusy) {
			writeErrorStatus(w, http.StatusConflict, err)
			return
		}
		if errors.Is(err, errBrowserWorkerUnavailable) || errors.Is(err, errBrowserWorkerFailed) {
			s.browser.remove(session)
			s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_closed", Project: session.project, SessionID: session.id, Actor: actor, Result: "success", Reason: "worker_unavailable"})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, err)
		return
	}
	s.browser.remove(session)
	s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_closed", Project: session.project, SessionID: session.id, Actor: actor, Result: "success"})
	w.WriteHeader(http.StatusNoContent)
}

func validateBrowserInitialURL(actor, rawURL string) error {
	initialURL := strings.TrimSpace(rawURL)
	if actor == "agent" && initialURL != "" && initialURL != "about:blank" {
		return &httpError{status: http.StatusBadRequest, err: errors.New("agent must start at about:blank; use browser_navigate for user approval")}
	}
	return nil
}

func (s *Server) BrowserRestartSession(w http.ResponseWriter, r *http.Request) {
	session, ok := s.browser.session(r)
	if !ok {
		writeErrorStatus(w, http.StatusNotFound, errors.New("browser session not found"))
		return
	}
	if err := authorizeBrowserRequest(r, browserActor(r), true); err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_restart", Project: session.project, SessionID: session.id, Actor: browserActor(r), Result: "blocked", Reason: "authorization"})
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	if session.stateValue() != "error" {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_restart", Project: session.project, SessionID: session.id, Actor: "user", Result: "blocked", Reason: "error_state_required"})
		writeErrorStatus(w, http.StatusConflict, errors.New("browser session is not in an error state"))
		return
	}
	next, err := s.browser.restart(r.Context(), session)
	if err != nil {
		s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_restart", Project: session.project, SessionID: session.id, Actor: "user", Result: "error", Reason: "operation_failed"})
		writeError(w, err)
		return
	}
	s.browser.recordAudit(domain.BrowserAuditEvent{Event: "session_restarted", Project: next.project, SessionID: next.id, Actor: "user", Result: "success"})
	writeJSONResponse(w, http.StatusCreated, next.snapshot())
}

func (s *Server) BrowserProfileReset(w http.ResponseWriter, r *http.Request) {
	if err := authorizeBrowserRequest(r, browserActor(r), true); err != nil {
		writeErrorStatus(w, http.StatusForbidden, err)
		return
	}
	var req browserProfileResetRequest
	if !decodeBody(w, r, &req) {
		return
	}
	project := browserProject(r, req.Project)
	if project == "" {
		writeError(w, errors.New("browser project is required"))
		return
	}
	if err := s.browser.resetProfile(project, os.RemoveAll); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) BrowserStream(w http.ResponseWriter, r *http.Request) {
	if err := authorizeBrowserRequest(r, browserActor(r), false); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	session, ok := s.browser.session(r)
	if !ok {
		http.Error(w, "browser session not found", http.StatusNotFound)
		return
	}
	client := session.addClient()
	conn, err := browserUpgrader.Upgrade(w, r, nil)
	if err != nil {
		client.close()
		return
	}
	defer client.close()
	conn.SetReadLimit(1 << 20)
	go client.writeLoop(conn)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *browserSessionHub) create(ctx context.Context, project, initialURL, actor string) (*browserSession, bool, error) {
	unlock := h.lockProjectOperation(project)
	defer unlock()
	return h.createLocked(ctx, project, initialURL, actor)
}

func (h *browserSessionHub) createLocked(ctx context.Context, project, initialURL, actor string) (*browserSession, bool, error) {
	h.mu.Lock()
	for _, session := range h.sessions {
		if session.project == project && !session.isClosed() {
			h.mu.Unlock()
			return session, false, nil
		}
	}
	h.mu.Unlock()

	worker, err := h.ensureWorker(ctx, project)
	if err != nil {
		return nil, false, err
	}
	id, err := randomID()
	if err != nil {
		return nil, false, err
	}
	initialURL = strings.TrimSpace(initialURL)
	if initialURL == "" {
		initialURL = "about:blank"
	}
	session := &browserSession{
		hub: h, project: project, id: id, state: "starting", owner: actor,
		worker: worker,
		url:    initialURL, lastActivity: time.Now(), pending: make(map[string]chan browserWorkerResult), clients: make(map[*browserClient]struct{}),
	}
	h.mu.Lock()
	for _, existing := range h.sessions {
		if existing.project == project && !existing.isClosed() {
			h.mu.Unlock()
			return existing, false, nil
		}
	}
	h.sessions[id] = session
	h.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{
		"project": project, "profile_dir": config.BrowserProfileDir(project), "initial_url": initialURL, "owner": actor,
	})
	if _, err := session.commandWithWorker(ctx, worker, actor, "start_session", payload); err != nil {
		h.removeLocked(session)
		return nil, false, err
	}
	return session, true, nil
}

func (h *browserSessionHub) ensureWorker(ctx context.Context, project string) (*browserWorker, error) {
	h.mu.Lock()
	if worker := h.workers[project]; worker != nil {
		h.mu.Unlock()
		return worker, nil
	}
	if start := h.workerStart[project]; start != nil {
		h.mu.Unlock()
		select {
		case <-start.done:
			return start.worker, start.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if h.policy.maxWorkers > 0 && len(h.workers)+len(h.workerStart) >= h.policy.maxWorkers {
		h.mu.Unlock()
		return nil, &httpError{status: http.StatusTooManyRequests, err: errBrowserWorkerLimit}
	}
	start := &browserWorkerStart{done: make(chan struct{})}
	h.workerStart[project] = start
	h.mu.Unlock()

	worker, err := startBrowserWorker(ctx, h, project)
	h.mu.Lock()
	delete(h.workerStart, project)
	if err != nil {
		start.err = err
		close(start.done)
		h.mu.Unlock()
		return nil, err
	}
	if existing := h.workers[project]; existing != nil {
		start.worker = existing
		start.err = nil
		close(start.done)
		h.mu.Unlock()
		worker.close()
		return existing, nil
	}
	h.workers[project] = worker
	start.worker = worker
	start.err = nil
	close(start.done)
	h.mu.Unlock()
	go worker.readLoop()
	return worker, nil
}

func startBrowserWorker(ctx context.Context, hub *browserSessionHub, project string) (*browserWorker, error) {
	workerPath := os.Getenv("MYBOX_BROWSER_WORKER")
	if workerPath == "" {
		workerPath = filepath.Join("browser-worker", "src", "index.mjs")
	}
	if !filepath.IsAbs(workerPath) {
		candidates := []string{}
		if cwd, err := os.Getwd(); err == nil {
			candidates = append(candidates, filepath.Join(cwd, workerPath))
		}
		if executable, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(executable), workerPath))
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				workerPath = candidate
				break
			}
		}
	}
	if _, err := os.Stat(workerPath); err != nil {
		return nil, fmt.Errorf("browser worker is unavailable at %s: %w (set MYBOX_BROWSER_WORKER)", workerPath, err)
	}
	token, err := randomID()
	if err != nil {
		return nil, err
	}
	// The worker belongs to the mybox server, not to the HTTP request that
	// happened to create the first session. It is terminated explicitly by
	// browserSessionHub.stop or when the worker connection fails.
	cmd := exec.Command("node", workerPath, "--port", "0", "--token", token)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start browser worker: %w", err)
	}
	pidFile := browserWorkerPIDFile(os.Getenv("MYBOX_BROWSER_TEST_WORKER_PID_FILE"), project)
	if pidFile != "" {
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
			stopBrowserProcess(cmd)
			removeBrowserWorkerPIDFile(pidFile, cmd.Process.Pid)
			return nil, fmt.Errorf("write browser worker pid file: %w", err)
		}
	}
	workerReady := false
	defer func() {
		if !workerReady && pidFile != "" {
			removeBrowserWorkerPIDFile(pidFile, cmd.Process.Pid)
		}
	}()
	ready := make(chan struct {
		port string
		err  error
	}, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var message struct {
				Type  string `json:"type"`
				Port  int    `json:"port"`
				Error string `json:"error"`
			}
			if json.Unmarshal(scanner.Bytes(), &message) != nil {
				continue
			}
			if message.Type == "ready" {
				ready <- struct {
					port string
					err  error
				}{port: fmt.Sprintf("127.0.0.1:%d", message.Port)}
				return
			}
			if message.Type == "error" {
				ready <- struct {
					port string
					err  error
				}{err: errors.New(message.Error)}
				return
			}
		}
		ready <- struct {
			port string
			err  error
		}{err: errors.New("browser worker exited before ready")}
	}()
	var result struct {
		port string
		err  error
	}
	select {
	case result = <-ready:
	case <-time.After(browserWorkerReadyTimeout):
		stopBrowserProcess(cmd)
		return nil, errors.New("timed out waiting for browser worker")
	case <-ctx.Done():
		stopBrowserProcess(cmd)
		return nil, ctx.Err()
	}
	if result.err != nil {
		stopBrowserProcess(cmd)
		return nil, result.err
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+result.port+"/worker", http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		stopBrowserProcess(cmd)
		return nil, fmt.Errorf("connect to browser worker: %w", err)
	}
	conn.SetReadLimit(16 << 20)
	workerReady = true
	return &browserWorker{cmd: cmd, conn: conn, hub: hub, pidFile: pidFile, project: project, stop: make(chan struct{})}, nil
}

func browserWorkerPIDFile(basePath, project string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(project))
	return basePath + "." + hex.EncodeToString(digest[:8])
}

func (w *browserWorker) send(message any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.conn.WriteJSON(message)
}

func (w *browserWorker) close() {
	select {
	case <-w.stop:
		return
	default:
		close(w.stop)
	}
	if w.conn != nil {
		_ = w.conn.Close()
	}
	stopBrowserProcess(w.cmd)
	if w.pidFile != "" {
		pid := 0
		if w.cmd != nil && w.cmd.Process != nil {
			pid = w.cmd.Process.Pid
		}
		removeBrowserWorkerPIDFile(w.pidFile, pid)
	}
}

func removeBrowserWorkerPIDFile(path string, pid int) {
	if path == "" || pid <= 0 {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(pid) {
		return
	}
	_ = os.Remove(path)
}

func stopBrowserProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func (w *browserWorker) readLoop() {
	for {
		messageType, data, err := w.conn.ReadMessage()
		if err != nil {
			w.hub.workerFailed(w)
			return
		}
		if messageType == websocket.BinaryMessage {
			w.hub.broadcastBinary(data)
			continue
		}
		var message browserWorkerMessage
		if err := json.Unmarshal(data, &message); err != nil {
			continue
		}
		w.hub.handleWorkerMessage(message, data)
	}
}

func (h *browserSessionHub) workerFailed(worker *browserWorker) {
	h.mu.Lock()
	if h.workers[worker.project] != worker {
		h.mu.Unlock()
		return
	}
	delete(h.workers, worker.project)
	sessions := make([]*browserSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session.worker == worker {
			sessions = append(sessions, session)
		}
	}
	h.mu.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		if session.closed {
			session.mu.Unlock()
			continue
		}
		session.state = "error"
		session.lastError = "browser worker disconnected"
		for _, pending := range session.pending {
			select {
			case pending <- browserWorkerResult{err: errors.New("browser worker disconnected")}:
			default:
			}
		}
		session.pending = make(map[string]chan browserWorkerResult)
		session.workerRequestID = ""
		session.mu.Unlock()
		h.recordAudit(domain.BrowserAuditEvent{
			Event: "worker_failure", Project: session.project, SessionID: session.id,
			Actor: "system", Result: "error", Reason: "worker_disconnected",
		})
		session.broadcastJSON(map[string]any{"type": "error", "error": session.lastErrorValue(), "state": "error"})
	}
	worker.close()
}

func (h *browserSessionHub) handleWorkerMessage(message browserWorkerMessage, raw []byte) {
	h.mu.Lock()
	session := h.sessions[message.SessionID]
	h.mu.Unlock()
	if session == nil {
		return
	}
	if message.Type == "result" && message.RequestID != "" {
		session.mu.Lock()
		pending := session.pending[message.RequestID]
		delete(session.pending, message.RequestID)
		if session.workerRequestID == message.RequestID {
			session.workerRequestID = ""
		}
		session.mu.Unlock()
		if pending != nil {
			if message.Error != "" {
				pending <- browserWorkerResult{err: errors.New(message.Error)}
			} else {
				pending <- browserWorkerResult{data: message.Data}
			}
		}
		return
	}
	if message.Type == "approval_required" {
		session.mu.Lock()
		session.pendingApproval = &browserApproval{ID: message.ApprovalID, Action: message.Action, Target: message.Target}
		session.mu.Unlock()
		h.recordAudit(domain.BrowserAuditEvent{
			Event: "approval_requested", Project: session.project, SessionID: session.id,
			Actor: "agent", Command: browserAuditCommandName(message.Action), Result: "pending",
		})
	}
	if message.Type == "approval_approved" || message.Type == "approval_rejected" || message.Type == "approval_failed" {
		session.clearApproval(message.ApprovalID)
	}
	if message.State != "" || message.Type == "session" {
		session.mu.Lock()
		if message.State != "" {
			session.state = browserStateForOwner(message.State, session.owner)
		}
		if message.URL != "" {
			session.url = message.URL
		}
		if message.Title != "" {
			session.pageTitle = message.Title
		}
		session.mu.Unlock()
	}
	session.broadcastJSON(raw)
}

func browserStateForOwner(state, owner string) string {
	switch state {
	case "running", "user_control":
		if owner == "user" {
			return "user_control"
		}
		if owner == "agent" {
			return "running"
		}
	}
	return state
}

func (h *browserSessionHub) broadcastBinary(data []byte) {
	if len(data) < 4 {
		return
	}
	idLength := int(binary.BigEndian.Uint32(data[:4]))
	if idLength <= 0 || len(data) < 4+idLength {
		return
	}
	h.mu.Lock()
	session := h.sessions[string(data[4:4+idLength])]
	h.mu.Unlock()
	if session != nil {
		session.broadcast(websocket.BinaryMessage, data[4+idLength:])
	}
}

func (h *browserSessionHub) session(r *http.Request) (*browserSession, bool) {
	id := r.URL.Query().Get("session_id")
	if id == "" {
		id = browserSessionID(r.URL.Path)
	}
	h.mu.Lock()
	session, ok := h.sessions[id]
	h.mu.Unlock()
	if !ok || session.isClosed() {
		return nil, false
	}
	project := browserProject(r, "")
	if project == "" || session.project != project {
		return nil, false
	}
	return session, true
}

func (h *browserSessionHub) restart(ctx context.Context, session *browserSession) (*browserSession, error) {
	unlock := h.lockProjectOperation(session.project)
	defer unlock()

	h.mu.Lock()
	if current := h.sessions[session.id]; current != session {
		h.mu.Unlock()
		return nil, errors.New("browser session is no longer available")
	}
	h.mu.Unlock()
	h.removeLocked(session)
	next, _, err := h.createLocked(ctx, session.project, "about:blank", "user")
	if err != nil {
		return nil, err
	}
	return next, nil
}

func (h *browserSessionHub) hasProject(project string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, session := range h.sessions {
		if session.project == project && !session.isClosed() {
			return true
		}
	}
	return false
}

func (h *browserSessionHub) remove(session *browserSession) {
	unlock := h.lockProjectOperation(session.project)
	defer unlock()
	h.removeLocked(session)
}

func (h *browserSessionHub) removeLocked(session *browserSession) {
	h.mu.Lock()
	delete(h.sessions, session.id)
	worker := session.worker
	stopWorker := worker != nil && h.workers[session.project] == worker
	if stopWorker {
		for _, candidate := range h.sessions {
			if candidate.worker == worker && !candidate.isClosed() {
				stopWorker = false
				break
			}
		}
		if stopWorker {
			delete(h.workers, session.project)
		}
	}
	h.mu.Unlock()
	session.mu.Lock()
	session.closed = true
	session.state = "closed"
	clients := make([]*browserClient, 0, len(session.clients))
	for client := range session.clients {
		clients = append(clients, client)
	}
	session.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
	if stopWorker {
		worker.close()
	}
}

func (s *browserSession) command(ctx context.Context, actor string, command string, payload json.RawMessage) (json.RawMessage, error) {
	return s.runCommand(ctx, actor, command, payload, true)
}

func (s *browserSession) commandWithoutOwner(ctx context.Context, actor string, command string, payload json.RawMessage) (json.RawMessage, error) {
	return s.runCommand(ctx, actor, command, payload, false)
}

func (s *browserSession) runCommand(ctx context.Context, actor string, command string, payload json.RawMessage, checkOwner bool) (json.RawMessage, error) {
	s.touch()
	if err := s.acquireCommand(ctx); err != nil {
		return nil, err
	}
	defer s.releaseCommand()
	if s.stateValue() == "error" {
		return nil, errBrowserWorkerFailed
	}
	if s.workerCommandInFlight() {
		return nil, errBrowserCommandBusy
	}
	if err := authorizeBrowserCommand(actor, command); err != nil {
		return nil, err
	}
	if checkOwner {
		if err := s.authorize(actor); err != nil {
			return nil, err
		}
		if actor == "agent" {
			s.mu.Lock()
			approvalPending := s.pendingApproval != nil
			s.mu.Unlock()
			if approvalPending {
				return nil, errBrowserApprovalPending
			}
		}
	}
	s.mu.Lock()
	worker := s.worker
	s.mu.Unlock()
	if worker == nil {
		return nil, errBrowserWorkerUnavailable
	}
	return s.commandWithWorker(ctx, worker, actor, command, payload)
}

func (s *browserSession) acquireCommand(ctx context.Context) error {
	s.mu.Lock()
	if s.commandMu == nil {
		s.commandMu = make(chan struct{}, 1)
	}
	commandMu := s.commandMu
	s.mu.Unlock()
	select {
	case commandMu <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *browserSession) tryAcquireCommand() bool {
	s.mu.Lock()
	if s.closed || s.state == "error" || s.workerRequestID != "" {
		s.mu.Unlock()
		return false
	}
	if s.commandMu == nil {
		s.commandMu = make(chan struct{}, 1)
	}
	commandMu := s.commandMu
	s.mu.Unlock()
	select {
	case commandMu <- struct{}{}:
		s.mu.Lock()
		busy := s.workerRequestID != ""
		s.mu.Unlock()
		if busy {
			s.releaseCommand()
			return false
		}
		return true
	default:
		return false
	}
}

func (s *browserSession) releaseCommand() {
	s.mu.Lock()
	commandMu := s.commandMu
	s.mu.Unlock()
	if commandMu != nil {
		<-commandMu
	}
}

func (s *browserSession) workerCommandInFlight() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workerRequestID != ""
}

func (s *browserSession) finishWorkerRequest(requestID string) {
	s.mu.Lock()
	if s.workerRequestID == requestID {
		s.workerRequestID = ""
	}
	s.mu.Unlock()
}

func (s *browserSession) forgetWorkerRequest(requestID string) {
	s.mu.Lock()
	delete(s.pending, requestID)
	if s.workerRequestID == requestID {
		s.workerRequestID = ""
	}
	s.mu.Unlock()
}

func (s *browserSession) cancelWorkerSession(worker *browserWorker, requestID string) bool {
	cancelRequestID, err := randomID()
	if err != nil {
		s.forgetWorkerRequest(requestID)
		return false
	}
	cancelResult := make(chan browserWorkerResult, 1)
	s.mu.Lock()
	s.pending[cancelRequestID] = cancelResult
	s.mu.Unlock()
	message := map[string]any{
		"type":       "command",
		"session_id": s.id,
		"request_id": cancelRequestID,
		"command":    "close_session",
		"actor":      "system",
	}
	if err := worker.send(message); err != nil {
		s.forgetWorkerRequest(cancelRequestID)
		s.forgetWorkerRequest(requestID)
		return false
	}

	timer := time.NewTimer(browserSessionCancelTimeout)
	defer timer.Stop()
	select {
	case <-cancelResult:
		// close_session removes only this session inside the worker. Remove the
		// corresponding server-side session only after that command completed.
		s.forgetWorkerRequest(cancelRequestID)
		s.forgetWorkerRequest(requestID)
		s.hub.remove(s)
		return true
	case <-timer.C:
		s.forgetWorkerRequest(cancelRequestID)
		s.forgetWorkerRequest(requestID)
		return false
	}
}

func (s *browserSession) commandWithWorker(ctx context.Context, worker *browserWorker, actor string, command string, payload json.RawMessage) (json.RawMessage, error) {
	requestID, err := randomID()
	if err != nil {
		return nil, err
	}
	result := make(chan browserWorkerResult, 1)
	s.mu.Lock()
	if s.closed && command != "start_session" {
		s.mu.Unlock()
		return nil, errors.New("browser session is closed")
	}
	if s.state == "error" && command != "start_session" {
		s.mu.Unlock()
		return nil, errBrowserWorkerFailed
	}
	s.pending[requestID] = result
	s.workerRequestID = requestID
	s.mu.Unlock()
	message := map[string]any{"type": "command", "session_id": s.id, "request_id": requestID, "command": command, "actor": actor}
	if len(payload) > 0 {
		var value any
		if json.Unmarshal(payload, &value) == nil {
			message["payload"] = value
		}
	}
	if err := worker.send(message); err != nil {
		s.mu.Lock()
		delete(s.pending, requestID)
		if s.workerRequestID == requestID {
			s.workerRequestID = ""
		}
		s.mu.Unlock()
		return nil, err
	}
	timer := time.NewTimer(browserCommandTimeout)
	defer timer.Stop()
	select {
	case workerResult := <-result:
		return workerResult.data, workerResult.err
	case <-timer.C:
		// Closing the session context interrupts the Playwright request without
		// taking unrelated sessions down with it. If the worker cannot acknowledge
		// the session close, terminate it as a safety fallback.
		if !s.cancelWorkerSession(worker, requestID) {
			worker.close()
			s.hub.workerFailed(worker)
		}
		return nil, errors.New("browser command timed out")
	case <-ctx.Done():
		// Keep the session command gate held until the worker acknowledges the
		// session close. Otherwise a cancelled HTTP request could still mutate the
		// page after a successful user takeover.
		if !s.cancelWorkerSession(worker, requestID) {
			worker.close()
			s.hub.workerFailed(worker)
		}
		return nil, ctx.Err()
	}
}

func (s *browserSession) authorize(actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != actor {
		return &httpError{status: http.StatusConflict, err: fmt.Errorf("browser session is controlled by %s", s.owner)}
	}
	return nil
}

func (s *browserSession) setOwner(owner string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("browser session is closed")
	}
	if s.state == "error" {
		s.mu.Unlock()
		return errBrowserWorkerFailed
	}
	s.owner = owner
	s.lastActivity = time.Now()
	if owner == "user" && s.state == "running" {
		s.state = "user_control"
	}
	if owner == "agent" && s.state == "user_control" {
		s.state = "running"
	}
	s.mu.Unlock()
	s.broadcastJSON(map[string]any{"type": "control", "owner": owner, "state": s.stateValue()})
	return nil
}

func (s *browserSession) ownerValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owner
}

func (s *browserSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *browserSession) stateValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *browserSession) lastErrorValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

func (s *browserSession) touch() {
	s.touchAt(time.Now())
}

func (s *browserSession) touchAt(now time.Time) {
	s.mu.Lock()
	if !s.closed {
		s.lastActivity = now
	}
	s.mu.Unlock()
}

func (s *browserSession) isIdle(now time.Time, timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state == "error" || s.owner == "user" || s.pendingApproval != nil || s.workerRequestID != "" || len(s.clients) > 0 {
		return false
	}
	return !s.lastActivity.IsZero() && now.Sub(s.lastActivity) >= timeout
}

func (s *browserSession) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	approvalMode := s.approvalMode
	if approvalMode == "" {
		approvalMode = browserApprovalModeManual
	}
	var pendingApproval any
	if s.pendingApproval != nil {
		pendingApproval = *s.pendingApproval
	}
	snapshot := map[string]any{
		"id": s.id, "project": s.project, "state": s.state, "owner": s.owner,
		"url": s.url, "title": s.pageTitle, "pending_approval": pendingApproval, "approval_mode": approvalMode,
	}
	if s.lastError != "" {
		snapshot["error"] = s.lastError
	}
	return snapshot
}

func (s *browserSession) setApprovalMode(mode string) {
	s.mu.Lock()
	s.approvalMode = mode
	s.mu.Unlock()
}

func (s *browserSession) clearApproval(id string) {
	s.mu.Lock()
	if s.pendingApproval != nil && (id == "" || s.pendingApproval.ID == id) {
		s.pendingApproval = nil
	}
	s.mu.Unlock()
}

func (s *browserSession) addClient() *browserClient {
	client := &browserClient{session: s, send: make(chan browserStreamMessage, 32), stop: make(chan struct{})}
	s.mu.Lock()
	if !s.closed {
		s.lastActivity = time.Now()
	}
	latestFrame := append([]byte(nil), s.latestFrame...)
	if len(latestFrame) > 0 {
		client.send <- browserStreamMessage{messageType: websocket.BinaryMessage, data: latestFrame}
	}
	s.clients[client] = struct{}{}
	pendingApproval := s.pendingApproval
	lastError := s.lastError
	state := s.state
	s.mu.Unlock()
	if pendingApproval != nil {
		if data, err := json.Marshal(map[string]any{
			"type":        "approval_required",
			"approval_id": pendingApproval.ID,
			"action":      pendingApproval.Action,
			"target":      pendingApproval.Target,
		}); err == nil {
			client.send <- browserStreamMessage{messageType: websocket.TextMessage, data: data}
		}
	}
	if lastError != "" {
		if data, err := json.Marshal(map[string]any{"type": "error", "error": lastError, "state": state}); err == nil {
			client.send <- browserStreamMessage{messageType: websocket.TextMessage, data: data}
		}
	}
	return client
}

func (c *browserClient) close() {
	c.once.Do(func() {
		close(c.stop)
		c.session.mu.Lock()
		delete(c.session.clients, c)
		c.session.mu.Unlock()
	})
}

func (c *browserClient) writeLoop(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()
	for {
		select {
		case message := <-c.send:
			if err := conn.WriteMessage(message.messageType, message.data); err != nil {
				return
			}
		case <-c.stop:
			return
		}
	}
}

func (s *browserSession) broadcast(messageType int, data []byte) {
	s.mu.Lock()
	if messageType == websocket.BinaryMessage {
		s.latestFrame = append(s.latestFrame[:0], data...)
	}
	clients := make([]*browserClient, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()
	for _, client := range clients {
		message := browserStreamMessage{messageType: messageType, data: append([]byte(nil), data...)}
		select {
		case client.send <- message:
		default:
			// Frames are disposable; drop the oldest queued frame by dropping the
			// current one when a slow UI cannot keep up.
		}
	}
}

func (s *browserSession) broadcastJSON(value any) {
	data, err := json.Marshal(value)
	if err == nil {
		s.broadcast(websocket.TextMessage, data)
	}
}

func browserProject(r *http.Request, bodyProject string) string {
	if project := strings.TrimSpace(bodyProject); project != "" {
		return project
	}
	if project := strings.TrimSpace(r.Header.Get("X-Project")); project != "" {
		return project
	}
	return strings.TrimSpace(r.URL.Query().Get("project"))
}

func browserActor(r *http.Request) string {
	if strings.EqualFold(r.Header.Get("X-Browser-Actor"), "agent") {
		return "agent"
	}
	return "user"
}

func authorizeBrowserCommand(actor, command string) error {
	if actor == "agent" {
		switch command {
		case "input":
			return &httpError{status: http.StatusForbidden, err: errors.New("browser input is user-only")}
		case "set_approval_mode":
			return &httpError{status: http.StatusForbidden, err: errors.New("browser approval mode is user-only")}
		}
	}
	return nil
}

func authorizeBrowserRequest(r *http.Request, actor string, userOnly bool) error {
	if r.Header.Get("Origin") != "" && !sameOrigin(r) {
		return errors.New("browser request origin is not allowed")
	}
	if userOnly && actor != "user" {
		return errors.New("only the user can perform this browser action")
	}
	if actor == "agent" {
		expected := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_AGENT_TOKEN"))
		if expected != "" {
			provided := strings.TrimSpace(r.Header.Get("X-Browser-Agent-Token"))
			if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
				return errors.New("browser agent authentication failed")
			}
			return nil
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			return errors.New("browser agent requests must originate from loopback or use MYBOX_BROWSER_AGENT_TOKEN")
		}
		return nil
	}
	// User authentication is intentionally delegated to a reverse proxy. The
	// backend trusts only the TCP peer address and never X-Forwarded-For, so a
	// proxy must run on this host and forward authenticated requests over
	// loopback.
	if !isLoopbackRemoteAddr(r.RemoteAddr) {
		return errors.New("browser user requests require loopback; authenticate remote users at a reverse proxy")
	}
	return nil
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host := remoteAddr
	if parsedHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.IsLoopback()
	}
	return ip.IsLoopback()
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(rand.Reader, value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func (s *Server) getAppWithProject(ctx context.Context, project string) (*App, error) {
	request := &http.Request{Header: http.Header{"X-Project": []string{project}}}
	request = request.WithContext(ctx)
	return s.getApp(request)
}

func (s *Server) registerBrowserRoutes(e *echo.Echo, basePath string) {
	prefix := basePath + "/api/browser"
	e.GET(prefix+"/sessions", echo.WrapHandler(http.HandlerFunc(s.BrowserSessions)))
	e.POST(prefix+"/sessions", echo.WrapHandler(http.HandlerFunc(s.CreateBrowserSession)))
	e.POST(prefix+"/sessions/:id/commands", echo.WrapHandler(http.HandlerFunc(s.BrowserCommand)))
	e.POST(prefix+"/sessions/:id/approval-mode", echo.WrapHandler(http.HandlerFunc(s.BrowserApprovalMode)))
	e.POST(prefix+"/sessions/:id/control/takeover", echo.WrapHandler(http.HandlerFunc(s.BrowserTakeover)))
	e.POST(prefix+"/sessions/:id/control/return", echo.WrapHandler(http.HandlerFunc(s.BrowserReturnControl)))
	e.POST(prefix+"/sessions/:id/approvals/:approval_id/approve", echo.WrapHandler(http.HandlerFunc(s.BrowserApproval)))
	e.POST(prefix+"/sessions/:id/approvals/:approval_id/reject", echo.WrapHandler(http.HandlerFunc(s.BrowserApproval)))
	e.POST(prefix+"/sessions/:id/close", echo.WrapHandler(http.HandlerFunc(s.BrowserCloseSession)))
	e.POST(prefix+"/sessions/:id/restart", echo.WrapHandler(http.HandlerFunc(s.BrowserRestartSession)))
	e.POST(prefix+"/profiles/reset", echo.WrapHandler(http.HandlerFunc(s.BrowserProfileReset)))
	e.GET(prefix+"/stream", echo.WrapHandler(http.HandlerFunc(s.BrowserStream)))
}

func browserSessionID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "sessions" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func browserApprovalID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "approvals" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func writeJSONRaw(w http.ResponseWriter, status int, data json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeErrorStatus(w http.ResponseWriter, status int, err error) {
	writeJSONResponse(w, status, map[string]string{"error": err.Error()})
}
