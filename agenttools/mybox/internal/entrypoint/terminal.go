package entrypoint

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// terminalUpgrader upgrades plain HTTP requests to WebSocket connections.
// The origin check is relaxed because the app may be served behind reverse
// proxies and accessed from different hosts.
var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type terminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// terminalClient is a single attached WebSocket connection. It receives the
// session's output through an internal channel rather than writing directly
// to the socket so that a shell can stream to several clients at once.
type terminalClient struct {
	send chan []byte
}

// terminalSession represents a persistent shell (PTY) for a project. Unlike a
// plain WebSocket connection it lives on after the browser disconnects, so a
// client can navigate away or reload the page and later reconnect to the very
// same shell (including any running processes). A session is removed from the
// registry once its shell exits or it is explicitly destroyed.
type terminalSession struct {
	hub     *terminalHub
	project string
	id      string
	cmd     *exec.Cmd
	ptmx    *os.File

	mu      sync.Mutex
	clients map[*terminalClient]struct{}
	ring    [][]byte // bounded recent output replayed to late attachments
	done    chan struct{}
	once    sync.Once
	closed  bool
}

func (s *terminalSession) addClient(c *terminalClient) {
	s.mu.Lock()
	for _, chunk := range s.ring {
		select {
		case c.send <- chunk:
		default:
		}
	}
	s.clients[c] = struct{}{}
	s.mu.Unlock()
}

func (s *terminalSession) removeClient(c *terminalClient) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
}

func (s *terminalSession) broadcast(data []byte) {
	s.mu.Lock()
	s.ring = append(s.ring, data)
	if len(s.ring) > 512 {
		s.ring = append([][]byte(nil), s.ring[len(s.ring)-512:]...)
	}
	for c := range s.clients {
		select {
		case c.send <- data:
		default:
			// slow client; drop this chunk
		}
	}
	s.mu.Unlock()
}

// writeInput forwards keystrokes from any attached client to the PTY.
func (s *terminalSession) writeInput(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	_, _ = s.ptmx.Write(data)
}

func (s *terminalSession) resize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// destroy terminates the shell, deregisters the session and notifies every
// attached client so their sockets close cleanly. It is idempotent.
func (s *terminalSession) destroy() {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.done)
		clients := make([]*terminalClient, 0, len(s.clients))
		for c := range s.clients {
			clients = append(clients, c)
		}
		s.mu.Unlock()

		if s.hub != nil {
			s.hub.remove(s.project, s.id)
		}
		_ = s.ptmx.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		for _, c := range clients {
			select {
			case c.send <- nil:
			default:
			}
		}
	})
}

// pump reads PTY output and broadcasts it to every client while also feeding
// the ring buffer used for late attachments. It stops when the shell exits.
func (s *terminalSession) pump() {
	buf := make([]byte, 8192)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			out := make([]byte, n)
			copy(out, buf[:n])
			s.broadcast(out)
		}
		if err != nil {
			s.destroy()
			return
		}
	}
}

// watch reaps the child process; if the shell exits on its own (for example
// the user typed `exit`) the session is torn down and deregistered.
func (s *terminalSession) watch() {
	_ = s.cmd.Wait()
	s.destroy()
}

// serveClient drains a single client's channel, writing terminal output (or a
// close indication) to its WebSocket. The write mutex is shared with the ping
// goroutine because gorilla/websocket forbids concurrent writes to a socket.
func (s *terminalSession) serveClient(conn *websocket.Conn, client *terminalClient, writeMu *sync.Mutex) {
	write := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, data)
	}
	defer func() {
		s.removeClient(client)
		_ = conn.Close()
	}()
	for {
		select {
		case msg, ok := <-client.send:
			if !ok {
				return
			}
			if msg == nil {
				_ = write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
				return
			}
			if err := write(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-s.done:
			_ = write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
			return
		}
	}
}

// terminalHub keeps persistent terminal sessions keyed by project and session
// id. It is owned by the Server and lives for the process lifetime.
type terminalHub struct {
	mu       sync.Mutex
	sessions map[string]map[string]*terminalSession // project -> id -> session
}

func newTerminalHub() *terminalHub {
	return &terminalHub{sessions: map[string]map[string]*terminalSession{}}
}

func (h *terminalHub) get(project, id string) *terminalSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	proj := h.sessions[project]
	if proj == nil {
		return nil
	}
	return proj[id]
}

func (h *terminalHub) put(project, id string, sess *terminalSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[project] == nil {
		h.sessions[project] = map[string]*terminalSession{}
	}
	h.sessions[project][id] = sess
}

func (h *terminalHub) remove(project, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if proj, ok := h.sessions[project]; ok {
		delete(proj, id)
		if len(proj) == 0 {
			delete(h.sessions, project)
		}
	}
}

func (h *terminalHub) destroy(project, id string) {
	if sess := h.get(project, id); sess != nil {
		sess.destroy()
	}
}

// getOrStart returns the existing persistent session for project/id, creating
// it (and its shell) the first time. It is safe to call concurrently.
func (h *terminalHub) getOrStart(project, id, command, dir string, env []string) *terminalSession {
	if sess := h.get(project, id); sess != nil {
		return sess
	}
	sess, err := startTerminalSession(command, dir, env)
	if err != nil {
		return nil
	}
	sess.hub = h
	sess.project = project
	sess.id = id
	if existing := h.get(project, id); existing != nil {
		// Another request created it while we were starting; discard ours.
		sess.destroy()
		return existing
	}
	h.put(project, id, sess)
	return sess
}

// Terminal starts (or reattaches to) a shell for a project over a WebSocket.
// Browsers cannot send the X-Project header during a WebSocket handshake, so
// the project name is passed as a query parameter instead. Binary messages
// stream the raw PTY output to the browser; JSON messages forward input and
// resize events to the PTY.
//
// When a `session` id is provided the shell is persistent: it survives client
// disconnects and a later connection with the same id resumes it. Omitting the
// `session` id yields an ephemeral shell that ends when the client disconnects
// or its process exits.
func (s *Server) Terminal(c echo.Context) error {
	if s.readOnly {
		return echo.ErrForbidden
	}
	app, err := s.getAppByProject(c.Request().Context(), c.QueryParam("project"))
	if err != nil {
		return err
	}
	conn, err := terminalUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	project := app.Project.Name
	sessionID := c.QueryParam("session")
	command := c.QueryParam("command")
	ephemeral := sessionID == ""

	var sess *terminalSession
	if ephemeral {
		sess, err = startTerminalSession(command, app.Project.Path, os.Environ())
		if err != nil {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
				websocket.CloseInternalServerErr, "failed to start shell",
			))
			_ = conn.Close()
			return nil
		}
		// An ephemeral shell lives only as long as this client is attached.
		defer sess.destroy()
	} else {
		sess = s.terminals.getOrStart(project, sessionID, command, app.Project.Path, os.Environ())
		if sess == nil {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
				websocket.CloseInternalServerErr, "failed to start shell",
			))
			_ = conn.Close()
			return nil
		}
	}

	var writeMu sync.Mutex
	write := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, data)
	}

	client := &terminalClient{send: make(chan []byte, 512)}
	sess.addClient(client)
	go sess.serveClient(conn, client, &writeMu)

	// Greet new sessions before streaming output; reattached sessions replay
	// the banner from their ring buffer.
	if ephemeral || sess.ringLength() == 0 {
		sess.greet(app.Project.Path)
	}

	// Keep the connection alive through proxies that drop idle sockets.
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-ticker.C:
				if err := write(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, data, readErr := conn.ReadMessage()
		if readErr != nil {
			break
		}
		var msg terminalMessage
		if unmarshalErr := json.Unmarshal(data, &msg); unmarshalErr != nil {
			continue
		}
		switch msg.Type {
		case "input":
			sess.writeInput([]byte(msg.Data))
		case "resize":
			sess.resize(msg.Cols, msg.Rows)
		case "close":
			if !ephemeral {
				s.terminals.destroy(project, sessionID)
			}
		}
	}
	return nil
}

// DestroyTerminal tears down a persistent terminal session for a project.
// Used when the frontend closes a tab, ensuring the server-side shell (and any
// running processes) is actually terminated rather than lingering forever.
func (s *Server) DestroyTerminal(c echo.Context) error {
	if s.readOnly {
		return echo.ErrForbidden
	}
	project := c.Request().Header.Get("X-Project")
	if project == "" {
		project = c.QueryParam("project")
	}
	app, err := s.getAppByProject(c.Request().Context(), project)
	if err != nil {
		return err
	}
	s.terminals.destroy(app.Project.Name, c.QueryParam("session"))
	return c.NoContent(http.StatusNoContent)
}

func (s *terminalSession) greet(projectPath string) {
	s.broadcast([]byte("\r\n\x1b[1;32mmybox terminal\x1b[0m: " + projectPath + "\r\n"))
}

func (s *terminalSession) ringLength() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ring)
}

// startTerminalSession launches a shell in a PTY and wires up the session's
// pump and waiter goroutines. The session is not yet registered with the hub;
// callers decide whether it should be persistent or ephemeral.
func startTerminalSession(command, dir string, env []string) (*terminalSession, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	var cmd *exec.Cmd
	if command != "" {
		cmd = exec.Command(shell, "-c", command)
	} else {
		cmd = exec.Command(shell)
	}
	cmd.Dir = dir
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	sess := &terminalSession{
		clients: map[*terminalClient]struct{}{},
		done:    make(chan struct{}),
		ptmx:    ptmx,
		cmd:     cmd,
	}

	go sess.pump()
	go sess.watch()
	return sess, nil
}
