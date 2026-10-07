package entrypoint

import (
	"bytes"
	"encoding/json"
	"net/http"
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
	ID       string `json:"id,omitempty"`
	Data     string `json:"data,omitempty"`
	Status   string `json:"status,omitempty"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

func startTestFileExecution(t *testing.T, ts *httptest.Server, project, path string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/files/execute/runs", bytes.NewBufferString(`{"path":"`+path+`"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project", project)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var started struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&started))
	require.NotEmpty(t, started.ID)
	return started.ID
}

func TestFileExecutionContinuesAfterClientDisconnect(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "background.sh"), []byte("#!/bin/sh\nsleep 0.3\nprintf 'finished\\n'\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	startedID := startTestFileExecution(t, ts, app.Project.Name, "scripts/background.sh")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/files/execute/stream?project=" + app.Project.Name + "&run_id=" + startedID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	var running testFileExecuteStreamMessage
	require.NoError(t, conn.ReadJSON(&running))
	require.Equal(t, "state", running.Type)
	require.Equal(t, "running", running.Status)
	require.NoError(t, conn.Close())

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "scripts", "background.sh")); err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("background execution did not finish in time")
		}
		listReq, listErr := http.NewRequest(http.MethodGet, ts.URL+"/api/files/execute/runs", nil)
		require.NoError(t, listErr)
		listReq.Header.Set("X-Project", app.Project.Name)
		listRes, listErr := http.DefaultClient.Do(listReq)
		require.NoError(t, listErr)
		var runs []testFileExecuteStreamMessage
		require.NoError(t, json.NewDecoder(listRes.Body).Decode(&runs))
		_ = listRes.Body.Close()
		if len(runs) == 1 && runs[0].Status != "running" {
			require.Equal(t, "completed", runs[0].Status)
			require.Contains(t, runs[0].Output, "finished\n")
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	var completed testFileExecuteStreamMessage
	require.NoError(t, conn.ReadJSON(&completed))
	require.Equal(t, "state", completed.Type)
	require.Equal(t, "completed", completed.Status)
	require.Contains(t, completed.Output, "finished\n")
}

func TestFileExecutionCanBeStoppedAndDismissed(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "stop.sh"), []byte("#!/bin/sh\nsleep 10\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	runID := startTestFileExecution(t, ts, app.Project.Name, "scripts/stop.sh")

	stopReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/files/execute/runs/"+runID+"/stop", nil)
	require.NoError(t, err)
	stopReq.Header.Set("X-Project", app.Project.Name)
	stopRes, err := http.DefaultClient.Do(stopReq)
	require.NoError(t, err)
	defer func() { _ = stopRes.Body.Close() }()
	require.Equal(t, http.StatusOK, stopRes.StatusCode)
	var stopped testFileExecuteStreamMessage
	require.NoError(t, json.NewDecoder(stopRes.Body).Decode(&stopped))
	require.Equal(t, "stopped", stopped.Status)

	dismissReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/files/execute/runs/"+runID, nil)
	require.NoError(t, err)
	dismissReq.Header.Set("X-Project", app.Project.Name)
	dismissRes, err := http.DefaultClient.Do(dismissReq)
	require.NoError(t, err)
	defer func() { _ = dismissRes.Body.Close() }()
	require.Equal(t, http.StatusNoContent, dismissRes.StatusCode)

	listReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/files/execute/runs", nil)
	require.NoError(t, err)
	listReq.Header.Set("X-Project", app.Project.Name)
	listRes, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer func() { _ = listRes.Body.Close() }()
	var runs []testFileExecuteStreamMessage
	require.NoError(t, json.NewDecoder(listRes.Body).Decode(&runs))
	require.Empty(t, runs)
}

func TestFileExecutionRejectsTooManyConcurrentJobs(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "long.sh"), []byte("#!/bin/sh\nsleep 10\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Shutdown()

	for range fileExecutionConcurrentLimit {
		startTestFileExecution(t, ts, app.Project.Name, "scripts/long.sh")
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/files/execute/runs", bytes.NewBufferString(`{"path":"scripts/long.sh"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project", app.Project.Name)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusTooManyRequests, res.StatusCode)
}

func TestFileExecutionRejectsInvalidPathBeforeStarting(t *testing.T) {
	s, app := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/files/execute/runs", bytes.NewBufferString(`{"path":"missing.sh"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project", app.Project.Name)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusNotFound, res.StatusCode)

	listReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/files/execute/runs", nil)
	require.NoError(t, err)
	listReq.Header.Set("X-Project", app.Project.Name)
	listRes, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer func() { _ = listRes.Body.Close() }()
	var runs []testFileExecuteStreamMessage
	require.NoError(t, json.NewDecoder(listRes.Body).Decode(&runs))
	require.Empty(t, runs)
}

func TestFilesExecuteStreamSendsOutputBeforeProcessExit(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "progress.sh"), []byte("#!/bin/sh\nprintf 'first\\n'\nsleep 1\nprintf 'second\\n' >&2\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	runID := startTestFileExecution(t, ts, app.Project.Name, "scripts/progress.sh")
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/files/execute/stream?project=" + app.Project.Name + "&run_id=" + runID
	started := time.Now()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	var first testFileExecuteStreamMessage
	require.NoError(t, conn.ReadJSON(&first))
	require.Equal(t, "state", first.Type)
	require.Equal(t, "running", first.Status)
	require.Less(t, time.Since(started), 900*time.Millisecond)

	var output strings.Builder
	output.WriteString(first.Output)
	for !strings.Contains(output.String(), "first\n") {
		var message testFileExecuteStreamMessage
		require.NoError(t, conn.ReadJSON(&message))
		if message.Type == "output" {
			output.WriteString(message.Data)
			continue
		}
		require.Equal(t, "running", message.Status)
	}
	require.Contains(t, output.String(), "first\n")
	var completed testFileExecuteStreamMessage
	for {
		var message testFileExecuteStreamMessage
		require.NoError(t, conn.ReadJSON(&message))
		if message.Type == "output" {
			output.WriteString(message.Data)
			continue
		}
		completed = message
		break
	}

	require.Equal(t, "state", completed.Type)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 0, completed.ExitCode)
	require.Contains(t, output.String(), "second\n")
}

func TestFilesExecuteStreamSendsExecutionError(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "fail.sh"), []byte("#!/bin/sh\nexit 7\n"), 0o755))

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	runID := startTestFileExecution(t, ts, app.Project.Name, "scripts/fail.sh")
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/files/execute/stream?project=" + app.Project.Name + "&run_id=" + runID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var message testFileExecuteStreamMessage
	for {
		require.NoError(t, conn.ReadJSON(&message))
		if message.Status != "running" {
			break
		}
	}
	require.Equal(t, "state", message.Type)
	require.Equal(t, "failed", message.Status)
	require.Equal(t, 7, message.ExitCode)
}
