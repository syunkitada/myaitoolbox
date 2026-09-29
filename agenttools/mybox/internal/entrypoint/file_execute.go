package entrypoint

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type fileExecuteStreamMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Path     string `json:"path,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Message  string `json:"message,omitempty"`
}

type fileExecuteStreamWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *fileExecuteStreamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteJSON(fileExecuteStreamMessage{Type: "output", Data: string(p)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *fileExecuteStreamWriter) send(message fileExecuteStreamMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(message)
}

// ExecuteFileStream runs an executable file and sends output chunks over a
// WebSocket as they are produced. The project and path are query parameters
// because browsers cannot set the X-Project header during a WebSocket
// handshake.
func (s *Server) ExecuteFileStream(c echo.Context) error {
	app, err := s.getAppByProject(c.Request().Context(), c.QueryParam("project"))
	if err != nil {
		return err
	}
	conn, err := terminalUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()
	clientGone := make(chan struct{})
	go func() {
		defer close(clientGone)
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				cancel()
				return
			}
		}
	}()

	writer := &fileExecuteStreamWriter{conn: conn}
	path := c.QueryParam("path")
	result, err := app.Files.ExecuteStream(ctx, path, writer)
	select {
	case <-clientGone:
		return nil
	default:
	}
	if err != nil {
		_ = writer.send(fileExecuteStreamMessage{Type: "error", Path: path, Message: err.Error()})
		return nil
	}
	if err := writer.send(fileExecuteStreamMessage{
		Type:     "exit",
		Path:     path,
		ExitCode: result.ExitCode,
		TimedOut: result.TimedOut,
	}); err != nil {
		return nil
	}
	return nil
}
