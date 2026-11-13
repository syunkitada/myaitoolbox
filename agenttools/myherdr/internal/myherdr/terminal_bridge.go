package myherdr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const maxTerminalMessageBytes = 64 * 1024
const maxTerminalErrorBytes = 4096

type terminalStream struct {
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	command  *exec.Cmd
	done     chan struct{}
	writeMu  sync.Mutex
	waitOnce sync.Once
	waitErr  error
	stderr   *boundedBuffer
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	length := len(data)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = buffer.buffer.Write(data)
	}
	return length, nil
}

func (buffer *boundedBuffer) String() string {
	return buffer.buffer.String()
}

type terminalFrame struct {
	Type  string `json:"type"`
	Bytes string `json:"bytes"`
	Data  string `json:"data"`
}

type browserTerminalMessage struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Bytes     string `json:"bytes"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Direction string `json:"direction"`
	Lines     int    `json:"lines"`
	Source    string `json:"source"`
	Action    string `json:"action"`
	Button    string `json:"button"`
	Column    int    `json:"column"`
	Row       int    `json:"row"`
	Modifiers int    `json:"modifiers"`
}

type herdrTerminalMessage struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Bytes     string `json:"bytes,omitempty"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
	Direction string `json:"direction,omitempty"`
	Lines     int    `json:"lines,omitempty"`
	Source    string `json:"source,omitempty"`
	Action    string `json:"action,omitempty"`
	Button    string `json:"button,omitempty"`
	Column    int    `json:"column,omitempty"`
	Row       int    `json:"row,omitempty"`
	Modifiers int    `json:"modifiers,omitempty"`
}

func (stream *terminalStream) writeMessage(message herdrTerminalMessage) error {
	stream.writeMu.Lock()
	defer stream.writeMu.Unlock()
	encoder := json.NewEncoder(stream.stdin)
	if err := encoder.Encode(message); err != nil {
		return errors.New("herdr terminal stream closed")
	}
	return nil
}

func startTerminalStream(ctx context.Context, target terminalTarget, mode string, cols, rows int) (*terminalStream, error) {
	args := []string{"terminal", "session", mode, target.Pane, "--cols", strconv.Itoa(cols), "--rows", strconv.Itoa(rows)}
	var command *exec.Cmd
	if target.Remote != nil {
		ssh, err := exec.LookPath("ssh")
		if err != nil {
			return nil, errors.New("OpenSSH client is not installed or not on PATH")
		}
		for _, arg := range args {
			if !sshArgumentPattern.MatchString(arg) {
				return nil, errors.New("refusing an unsafe argument for remote Herdr")
			}
		}
		commandArgs := []string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "--", target.Remote.Host, "env", "HERDR_SESSION=" + target.Remote.HerdrSession, "herdr"}
		commandArgs = append(commandArgs, args...)
		command = exec.CommandContext(ctx, ssh, commandArgs...)
	} else {
		binary, err := exec.LookPath("herdr")
		if err != nil {
			return nil, errors.New("herdr is not installed or not on PATH; install it from https://herdr.dev/")
		}
		command = exec.CommandContext(ctx, binary, args...)
		command.Env = herdrEnvironment(target.Session)
	}

	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, errors.New("cannot open the Herdr terminal input stream")
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, errors.New("cannot open the Herdr terminal output stream")
	}
	stderr := &boundedBuffer{limit: maxTerminalErrorBytes}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, errors.New("cannot start the Herdr terminal stream; confirm the session and pane are available")
	}
	stream := &terminalStream{stdin: stdin, stdout: stdout, command: command, done: make(chan struct{}), stderr: stderr}
	return stream, nil
}

func (stream *terminalStream) stop() {
	_ = stream.stdin.Close()
	_ = stream.stdout.Close()
	if stream.command.Process != nil {
		_ = stream.command.Process.Kill()
	}
	stream.waitProcess()
}

func (stream *terminalStream) waitProcess() {
	stream.waitOnce.Do(func() {
		stream.waitErr = stream.command.Wait()
		close(stream.done)
	})
	<-stream.done
}

func streamTerminalFrames(ctx context.Context, stream *terminalStream, socket *websocket.Conn, writeMu *sync.Mutex) error {
	scanner := bufio.NewScanner(stream.stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var frame terminalFrame
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			return errors.New("herdr returned an invalid terminal frame")
		}
		switch frame.Type {
		case "terminal.frame":
			encoded := frame.Bytes
			if encoded == "" {
				encoded = frame.Data
			}
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return errors.New("herdr returned invalid terminal frame data")
			}
			writeMu.Lock()
			_ = socket.SetWriteDeadline(deadlineForWebsocket())
			err = socket.WriteMessage(websocket.BinaryMessage, data)
			writeMu.Unlock()
			if err != nil {
				return err
			}
		case "terminal.closed":
			writeMu.Lock()
			_ = socket.SetWriteDeadline(deadlineForWebsocket())
			err := socket.WriteJSON(map[string]string{"type": "closed"})
			writeMu.Unlock()
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Herdr terminal stream: %w", err)
	}
	stream.waitProcess()
	if stream.waitErr != nil && ctx.Err() == nil {
		message := strings.TrimSpace(stream.stderr.String())
		if message == "" {
			message = "Herdr closed the terminal stream unexpectedly; confirm the session and pane are available."
		}
		writeMu.Lock()
		_ = socket.SetWriteDeadline(deadlineForWebsocket())
		_ = socket.WriteJSON(map[string]string{"type": "error", "message": message})
		writeMu.Unlock()
		return stream.waitErr
	}
	return nil
}

func receiveTerminalInput(ctx context.Context, stream *terminalStream, socket *websocket.Conn, writeMu *sync.Mutex, mode string) error {
	socket.SetReadLimit(maxTerminalMessageBytes)
	for {
		messageType, data, err := socket.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.TextMessage {
			return errors.New("terminal input must use a JSON text frame")
		}
		var message browserTerminalMessage
		if err := json.Unmarshal(data, &message); err != nil {
			return errors.New("terminal input is invalid")
		}
		if mode != "control" {
			return errors.New("this terminal connection is read-only")
		}
		outgoing, err := validateTerminalInput(message)
		if err != nil {
			return err
		}
		if err := stream.writeMessage(outgoing); err != nil {
			return err
		}
		if message.Type == "release" {
			writeMu.Lock()
			_ = socket.SetWriteDeadline(deadlineForWebsocket())
			_ = socket.WriteJSON(map[string]string{"type": "closed"})
			writeMu.Unlock()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func validateTerminalInput(message browserTerminalMessage) (herdrTerminalMessage, error) {
	switch message.Type {
	case "input":
		if message.Text == "" || len(message.Text) > maxTerminalMessageBytes {
			return herdrTerminalMessage{}, errors.New("terminal input is empty or too large")
		}
		return herdrTerminalMessage{Type: "terminal.input", Text: message.Text}, nil
	case "resize":
		if message.Cols < 2 || message.Cols > 500 || message.Rows < 1 || message.Rows > 300 {
			return herdrTerminalMessage{}, errors.New("terminal size is outside the supported range")
		}
		return herdrTerminalMessage{Type: "terminal.resize", Cols: message.Cols, Rows: message.Rows}, nil
	case "scroll":
		if (message.Direction != "up" && message.Direction != "down") || message.Lines < 1 || message.Lines > 100 {
			return herdrTerminalMessage{}, errors.New("terminal scroll request is invalid")
		}
		if message.Source != "wheel" && message.Source != "page_key" {
			return herdrTerminalMessage{}, errors.New("terminal scroll source is invalid")
		}
		return herdrTerminalMessage{Type: "terminal.scroll", Direction: message.Direction, Lines: message.Lines, Source: message.Source}, nil
	case "mouse":
		if !validMouseAction(message.Action) || !validMouseButton(message.Button) || message.Column < 0 || message.Column > 500 || message.Row < 0 || message.Row > 300 || message.Modifiers < 0 || message.Modifiers > 7 {
			return herdrTerminalMessage{}, errors.New("terminal mouse request is invalid")
		}
		return herdrTerminalMessage{Type: "terminal.mouse", Action: message.Action, Button: message.Button, Column: message.Column, Row: message.Row, Modifiers: message.Modifiers}, nil
	case "release":
		return herdrTerminalMessage{Type: "terminal.release"}, nil
	default:
		return herdrTerminalMessage{}, errors.New("unsupported terminal input type")
	}
}

func validMouseAction(action string) bool {
	return action == "down" || action == "up" || action == "drag" || action == "move"
}

func validMouseButton(button string) bool {
	return button == "left" || button == "right" || button == "middle"
}

func deadlineForWebsocket() time.Time {
	return time.Now().Add(10 * time.Second)
}

func serveTerminalConnection(ctx context.Context, socket *websocket.Conn, target terminalTarget, mode string) {
	defer func() { _ = socket.Close() }()
	cols, rows := 100, 30
	stream, err := startTerminalStream(ctx, target, mode, cols, rows)
	if err != nil {
		_ = socket.WriteJSON(map[string]string{"type": "error", "message": err.Error()})
		return
	}
	defer stream.stop()
	var socketWriteMu sync.Mutex
	framesDone := make(chan error, 1)
	go func() { framesDone <- streamTerminalFrames(ctx, stream, socket, &socketWriteMu) }()
	inputDone := make(chan error, 1)
	go func() { inputDone <- receiveTerminalInput(ctx, stream, socket, &socketWriteMu, mode) }()

	select {
	case <-ctx.Done():
	case <-framesDone:
	case <-inputDone:
	}
}
