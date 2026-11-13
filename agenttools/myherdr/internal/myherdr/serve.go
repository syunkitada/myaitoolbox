package myherdr

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed web/* web/vendor/*
var webAssets embed.FS

type serveState struct {
	PID   int    `json:"pid"`
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type serveLock interface {
	Close() error
}

var errServeLockUnsupported = errors.New("myherdr serve is unsupported on this platform")

type serveApplication struct {
	project       Project
	port          int
	token         string
	controllers   map[string]struct{}
	controllerMu  sync.Mutex
	connections   map[*websocket.Conn]struct{}
	connectionsMu sync.Mutex
	connectionsWG sync.WaitGroup
	closing       bool
}

type paneListResponse struct {
	Panes       []Pane `json:"panes"`
	Recommended string `json:"recommended,omitempty"`
}

func Serve(ctx context.Context, project Project, port int, output io.Writer) error {
	if port < 0 || port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	runtimeDir, err := runtimeDirectory()
	if err != nil {
		return err
	}
	lock, err := acquireServeLock(filepath.Join(runtimeDir, "serve.lock"))
	if err != nil {
		if errors.Is(err, errServeLockUnsupported) {
			return err
		}
		return errors.New("myherdr serve is already running for this user")
	}
	defer func() { _ = lock.Close() }()

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("listen on 127.0.0.1:%d: %w", port, err)
	}
	defer func() { _ = listener.Close() }()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("could not determine the local HTTP port")
	}
	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return fmt.Errorf("create serve instance token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes[:])
	statePath := filepath.Join(runtimeDir, "serve.json")
	state := serveState{PID: os.Getpid(), Port: address.Port, Token: token}
	if err := writeServeState(statePath, state); err != nil {
		return err
	}
	defer removeServeState(statePath, state)

	app := &serveApplication{
		project:     project,
		port:        address.Port,
		token:       token,
		controllers: make(map[string]struct{}),
		connections: make(map[*websocket.Conn]struct{}),
	}
	server := &http.Server{
		Handler:           app.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          logDiscard(),
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if _, err := fmt.Fprintf(output, "myherdr serve listening at http://127.0.0.1:%d/\n", address.Port); err != nil {
		app.closeConnections()
		_ = server.Close()
		return err
	}

	select {
	case <-ctx.Done():
		app.closeConnections()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			app.waitConnections(shutdownCtx)
			return err
		}
		app.waitConnections(shutdownCtx)
		return nil
	case err := <-serveErr:
		app.closeConnections()
		_ = server.Close()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		app.waitConnections(waitCtx)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve local terminal web app: %w", err)
	}
}

func runtimeDirectory() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve runtime directory: %w", err)
		}
	}
	directory := filepath.Join(base, "myherdr", "runtime")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", fmt.Errorf("create runtime directory: %w", err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return "", fmt.Errorf("secure runtime directory: %w", err)
	}
	return directory, nil
}

func writeServeState(path string, state serveState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".serve-*.tmp")
	if err != nil {
		return fmt.Errorf("create runtime state: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure runtime state: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write runtime state: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync runtime state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close runtime state: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("publish runtime state: %w", err)
	}
	return nil
}

func removeServeState(path string, expected serveState) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var current serveState
	if json.Unmarshal(data, &current) == nil && current == expected {
		_ = os.Remove(path)
	}
}

func FindServeInstance() (serveState, error) {
	runtimeDir, err := runtimeDirectory()
	if err != nil {
		return serveState{}, err
	}
	data, err := os.ReadFile(filepath.Join(runtimeDir, "serve.json"))
	if err != nil {
		return serveState{}, errors.New("myherdr serve is not running; start it in a Herdr pane with `myherdr serve`")
	}
	var state serveState
	if err := json.Unmarshal(data, &state); err != nil || state.Port < 1 || state.Port > 65535 || len(state.Token) != 64 {
		return serveState{}, errors.New("myherdr serve is not running; start it in a Herdr pane with `myherdr serve`")
	}
	client := http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", state.Port))
	if err != nil {
		return serveState{}, errors.New("myherdr serve is not responding; start it in a Herdr pane with `myherdr serve`")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return serveState{}, errors.New("myherdr serve is not responding; start it in a Herdr pane with `myherdr serve`")
	}
	responseToken, readErr := io.ReadAll(io.LimitReader(response.Body, 128))
	if readErr != nil || strings.TrimSpace(string(responseToken)) != state.Token {
		return serveState{}, errors.New("myherdr serve is not responding; start it in a Herdr pane with `myherdr serve`")
	}
	return state, nil
}

func (app *serveApplication) handler() http.Handler {
	static, _ := fs.Sub(webAssets, "web")
	files := http.StripPrefix("/assets/", http.FileServer(http.FS(static)))
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(request *http.Request) bool {
			return request.Header.Get("Origin") == app.origin()
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, app.token+"\n")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/sessions/") {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/sessions/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/")
			if len(parts) != 1 || ValidateSessionName(parts[0]) != nil {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		data, err := webAssets.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "web app is unavailable", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	})
	mux.Handle("/assets/", files)
	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name, ok := routeSessionName(r.URL.Path, "/api/sessions/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		queryCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		panes, err := listPanes(queryCtx, app.project, name)
		if err != nil {
			http.Error(w, safeWebError(err), http.StatusBadGateway)
			return
		}
		response := paneListResponse{Panes: panes}
		codexPanes := make([]Pane, 0, 1)
		for _, pane := range panes {
			if pane.IsCodex {
				codexPanes = append(codexPanes, pane)
			}
		}
		if len(codexPanes) == 1 {
			response.Recommended = codexPanes[0].ID
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Origin") != app.origin() {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}
		name := r.URL.Query().Get("session")
		paneID := r.URL.Query().Get("pane")
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "control"
		}
		if ValidateSessionName(name) != nil || ValidatePaneID(paneID) != nil || (mode != "control" && mode != "observe") {
			http.Error(w, "invalid terminal target", http.StatusBadRequest)
			return
		}
		queryCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		panes, err := listPanes(queryCtx, app.project, name)
		cancel()
		if err != nil {
			http.Error(w, safeWebError(err), http.StatusBadGateway)
			return
		}
		found := false
		for _, pane := range panes {
			if pane.ID == paneID {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "pane is not available in this Herdr session", http.StatusNotFound)
			return
		}
		target, err := app.terminalTarget(name, paneID)
		if err != nil {
			http.Error(w, safeWebError(err), http.StatusBadGateway)
			return
		}
		controllerKey := name + ":" + paneID
		if mode == "control" && !app.acquireController(controllerKey) {
			http.Error(w, "another browser connection already controls this pane", http.StatusConflict)
			return
		}
		if mode == "control" {
			defer app.releaseController(controllerKey)
		}
		socket, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if !app.registerConnection(socket) {
			_ = socket.Close()
			return
		}
		defer app.unregisterConnection(socket)
		serveTerminalConnection(r.Context(), socket, target, mode)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws://127.0.0.1:"+strconv.Itoa(app.port)+"; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if r.Host != net.JoinHostPort("127.0.0.1", strconv.Itoa(app.port)) {
			http.Error(w, "host is not allowed", http.StatusMisdirectedRequest)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != app.origin() {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (app *serveApplication) origin() string {
	return fmt.Sprintf("http://127.0.0.1:%d", app.port)
}

func routeSessionName(path, prefix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[1] != "panes" {
		return "", false
	}
	name := parts[0]
	if strings.Contains(name, "%") || ValidateSessionName(name) != nil {
		return "", false
	}
	return name, true
}

func (app *serveApplication) terminalTarget(name, pane string) (terminalTarget, error) {
	target := terminalTarget{Session: name, Pane: pane}
	remote, isRemote, err := app.project.remoteSession(name)
	if err != nil {
		return terminalTarget{}, err
	}
	if isRemote {
		target.Remote = remote
	}
	return target, nil
}

func (app *serveApplication) acquireController(key string) bool {
	app.controllerMu.Lock()
	defer app.controllerMu.Unlock()
	if _, exists := app.controllers[key]; exists {
		return false
	}
	app.controllers[key] = struct{}{}
	return true
}

func (app *serveApplication) releaseController(key string) {
	app.controllerMu.Lock()
	defer app.controllerMu.Unlock()
	delete(app.controllers, key)
}

func (app *serveApplication) registerConnection(socket *websocket.Conn) bool {
	app.connectionsMu.Lock()
	defer app.connectionsMu.Unlock()
	if app.closing {
		return false
	}
	app.connections[socket] = struct{}{}
	app.connectionsWG.Add(1)
	return true
}

func (app *serveApplication) unregisterConnection(socket *websocket.Conn) {
	app.connectionsMu.Lock()
	delete(app.connections, socket)
	app.connectionsMu.Unlock()
	app.connectionsWG.Done()
}

func (app *serveApplication) closeConnections() {
	app.connectionsMu.Lock()
	app.closing = true
	connections := make([]*websocket.Conn, 0, len(app.connections))
	for socket := range app.connections {
		connections = append(connections, socket)
	}
	app.connectionsMu.Unlock()
	for _, socket := range connections {
		_ = socket.Close()
	}
}

func (app *serveApplication) waitConnections(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		app.connectionsWG.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
	case <-done:
	}
}

func safeWebError(err error) string {
	if err == nil {
		return "Herdr session is unavailable"
	}
	return err.Error()
}

func logDiscard() *log.Logger {
	return log.New(io.Discard, "", 0)
}
