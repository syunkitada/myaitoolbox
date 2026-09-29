package entrypoint

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type testFileExecuteStreamMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Message  string `json:"message,omitempty"`
}

func TestFilesExecuteStreamSendsOutputBeforeProcessExit(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "progress.sh"), []byte("#!/bin/sh\nprintf 'first\\n'\nsleep 1\nprintf 'second\\n' >&2\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/files/execute/stream?project=" + app.Project.Name + "&path=scripts%2Fprogress.sh"
	started := time.Now()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	var first testFileExecuteStreamMessage
	require.NoError(t, conn.ReadJSON(&first))
	require.Equal(t, "output", first.Type)
	require.Contains(t, first.Data, "first\n")
	require.Less(t, time.Since(started), 900*time.Millisecond)

	var output strings.Builder
	output.WriteString(first.Data)
	var exit testFileExecuteStreamMessage
	for {
		var message testFileExecuteStreamMessage
		require.NoError(t, conn.ReadJSON(&message))
		if message.Type == "output" {
			output.WriteString(message.Data)
			continue
		}
		exit = message
		break
	}

	require.Equal(t, "exit", exit.Type)
	require.Equal(t, 0, exit.ExitCode)
	require.Contains(t, output.String(), "second\n")
}

func TestFilesExecuteStreamSendsExecutionError(t *testing.T) {
	s, app := newTestServer(t)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/files/execute/stream?project=" + app.Project.Name + "&path=missing.sh"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var message testFileExecuteStreamMessage
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, "error", message.Type)
	require.Contains(t, message.Message, "not found")
}
