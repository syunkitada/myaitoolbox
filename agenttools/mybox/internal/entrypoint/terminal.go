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

// Terminal starts a shell for a project over a WebSocket. Browsers cannot
// send the X-Project header during a WebSocket handshake, so the project
// name is passed as a query parameter instead. Binary messages stream the
// raw PTY output to the browser; JSON messages forward input and resize
// events to the PTY.
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
	defer conn.Close()

	var writeMu sync.Mutex
	write := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, data)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = app.Project.Path
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = write(websocket.CloseMessage, websocket.FormatCloseMessage(
			websocket.CloseInternalServerErr, "failed to start shell",
		))
		return nil
	}
	defer func() { _ = ptmx.Close() }()

	// Greet the browser before streaming output.
	_ = write(websocket.TextMessage,
		[]byte("\r\n\x1b[1;32mmybox terminal\x1b[0m: "+app.Project.Path+"\r\n"))

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

	// Stream PTY output to the browser.
	go func() {
		buf := make([]byte, 8192)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				if err := write(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// Close the connection cleanly when the shell exits.
	go func() {
		_ = cmd.Wait()
		_ = write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	// Forward browser input and resize events to the PTY.
	for {
		_, data, readErr := conn.ReadMessage()
		if readErr != nil {
			return nil
		}
		var msg terminalMessage
		if unmarshalErr := json.Unmarshal(data, &msg); unmarshalErr != nil {
			continue
		}
		switch msg.Type {
		case "input":
			_, _ = ptmx.WriteString(msg.Data)
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				_ = pty.Setsize(ptmx, &pty.Winsize{
					Cols: uint16(msg.Cols),
					Rows: uint16(msg.Rows),
				})
			}
		}
	}
}
