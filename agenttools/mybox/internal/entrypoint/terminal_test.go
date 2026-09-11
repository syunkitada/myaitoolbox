package entrypoint

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestTerminal(t *testing.T) {
	s, app := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/terminal?project=" + app.Project.Name
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	readUntil := func(sub string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		var acc strings.Builder
		for {
			_, msg, err := conn.ReadMessage()
			require.NoError(t, err)
			acc.Write(msg)
			if strings.Contains(acc.String(), sub) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timeout waiting for %q; got %q", sub, acc.String())
			}
		}
	}

	// The greeting is pushed before the shell prompt so it is a stable marker.
	readUntil("mybox terminal")

	send := func(v string) {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(v)))
	}

	send(`{"type":"resize","cols":80,"rows":24}`)
	send(`{"type":"input","data":"printf 'TERM_TEST_OK\n'\r"}`)
	readUntil("TERM_TEST_OK")

	send(`{"type":"input","data":"exit\r"}`)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			closeErr, ok := err.(*websocket.CloseError)
			require.True(t, ok, "expected a websocket close, got: %v", err)
			require.Equal(t, websocket.CloseNormalClosure, closeErr.Code)
			return
		}
	}
}

// TestTerminalPersistentSession verifies that a session identified by a
// `session` query parameter survives a client disconnect: reconnecting with
// the same id resumes the *same* shell (shell variables set earlier persist),
// and a new connection after from-scratch destroy starts a fresh shell.
func TestTerminalPersistentSession(t *testing.T) {
	s, app := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/terminal?project=" + app.Project.Name + "&session=sess-1"

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		return conn
	}

	readUntil := func(conn *websocket.Conn, sub string) string {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		var acc strings.Builder
		for {
			if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatalf("set read deadline: %v", err)
			}
			_, msg, err := conn.ReadMessage()
			require.NoError(t, err)
			acc.Write(msg)
			if strings.Contains(acc.String(), sub) {
				return acc.String()
			}
			if time.Now().After(deadline) {
				t.Fatalf("timeout waiting for %q; got %q", sub, acc.String())
			}
		}
	}

	send := func(conn *websocket.Conn, v string) {
		t.Helper()
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(v)))
	}

	// First connection establishes the persistent shell and sets a variable.
	conn1 := dial()
	readUntil(conn1, "mybox terminal")
	send(conn1, `{"type":"input","data":"export PERSIST_VAR=hello123\r"}`)
	send(conn1, `{"type":"input","data":"echo R:$PERSIST_VAR\r"}`)
	require.Contains(t, readUntil(conn1, "R:hello123"), "R:hello123")

	// Disconnect. The shell must stay alive on the server.
	require.NoError(t, conn1.Close())

	// Reconnect with the same id: the same shell is resumed, so the variable
	// set before the disconnect is still present.
	conn2 := dial()
	defer conn2.Close()
	readUntil(conn2, "mybox terminal") // replayed from history
	send(conn2, `{"type":"input","data":"echo R2:$PERSIST_VAR\r"}`)
	require.Contains(t, readUntil(conn2, "R2:hello123"), "R2:hello123")

	// Explicitly destroy the session, then a new connection must start a fresh
	// shell with no memory of the variable.
	send(conn2, `{"type":"close"}`)
	require.NoError(t, conn2.Close())
	time.Sleep(200 * time.Millisecond)

	conn3 := dial()
	defer conn3.Close()
	readUntil(conn3, "mybox terminal")
	send(conn3, `{"type":"input","data":"echo R3:$PERSIST_VAR\r"}`)
	got := readUntil(conn3, "R3:")
	require.Contains(t, got, "R3:")
	require.NotContains(t, got, "hello123")
}

func TestTerminalReadOnly(t *testing.T) {
	s, _ := newTestServer(t, true)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	url := ts.URL + "/api/terminal?project=test"
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 403, resp.StatusCode)
}

func TestTerminalUnknownProject(t *testing.T) {
	s, _ := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	url := ts.URL + "/api/terminal?project=missing"
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t, resp.StatusCode >= 400, "expected failure status, got %d", resp.StatusCode)
}

// TestTerminalReattachMouseTrackingReset verifies that a client reattaching to
// a persistent session receives the mouse-tracking reset after the ring buffer
// replay. A previously-run application that enabled mouse tracking (DECSET
// 1003) leaves the enable sequence in the ring; without an explicit reset the
// fresh terminal would stay in mouse tracking mode and swallow every mouse
// event. The reset must arrive after the stale sequence so the final state is
// "mouse tracking off".
func TestTerminalReattachMouseTrackingReset(t *testing.T) {
	s, app := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/terminal?project=" + app.Project.Name + "&session=sess-2"

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		return conn
	}
	send := func(conn *websocket.Conn, v string) {
		t.Helper()
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(v)))
	}
	// recv reads a single message and returns it as a string (control bytes
	// pass through unchanged).
	recv := func(conn *websocket.Conn) string {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		return string(msg)
	}
	// drain reads messages until one contains sub.
	drain := func(conn *websocket.Conn, sub string) {
		t.Helper()
		for msg := recv(conn); !strings.Contains(msg, sub); msg = recv(conn) {
		}
	}

	// First client: let an "application" in the shell emit any-event mouse
	// tracking (DECSET 1003) into the session output. `printf '%b' '\33...'`
	// avoids embedding a raw ESC in the typed command, which readline would
	// consume as a control-sequence prefix.
	conn1 := dial()
	drain(conn1, "mybox terminal")
	send(conn1, `{"type":"input","data":"printf '%b' '\\033[?1003h'\r"}`)
	drain(conn1, "\x1b[?1003h")
	require.NoError(t, conn1.Close())

	// Give the session a moment to notice the disconnect and return the shell
	// to the foreground (printf finished long ago).
	time.Sleep(200 * time.Millisecond)

	// Second client reattaches to the same session: it replays the ring
	// (including the stale DECSET 1003) and must then receive the reset that
	// disables mouse tracking again. Track the order of the enable and reset.
	conn2 := dial()
	defer conn2.Close()
	var sawEnable, sawReset bool
	order := []string{}
	deadline := time.Now().Add(10 * time.Second)
	for !sawReset {
		msg := recv(conn2)
		if !sawEnable && strings.Contains(msg, "[?1003h") {
			sawEnable = true
			order = append(order, "enable")
		}
		if strings.Contains(msg, "[?1003l") {
			sawReset = true
			order = append(order, "reset")
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for mouse-tracking reset; order=%v", order)
		}
	}
	require.True(t, sawEnable, "expected stale DECSET 1003 in replayed history")
	require.Equal(t, []string{"enable", "reset"}, order,
		"mouse-tracking reset must come after the replayed enable")
}

// TestTerminalReattachKeepsAppMouseMode verifies the counterpart of the reset
// test: when a real application is still running in the foreground (like herdr,
// a "mouse-first" TUI), reattaching must re-assert its mouse-tracking mode
// rather than resetting it away. A blind reset disables mouse reporting in the
// browser terminal, and since the application enabled it exactly once at
// startup and never re-emits it, its mouse input would stay dead.
func TestTerminalReattachKeepsAppMouseMode(t *testing.T) {
	s, app := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/terminal?project=" + app.Project.Name + "&session=sess-3"

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		return conn
	}
	send := func(conn *websocket.Conn, v string) {
		t.Helper()
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(v)))
	}
	recv := func(conn *websocket.Conn) string {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		return string(msg)
	}
	drain := func(conn *websocket.Conn, sub string) {
		t.Helper()
		for msg := recv(conn); !strings.Contains(msg, sub); msg = recv(conn) {
		}
	}

	// Start a foreground program that enables any-event mouse tracking and then
	// stays alive, like a running TUI. `printf '%b' '\33...'` avoids embedding a
	// raw ESC in the typed command (readline would consume it).
	conn1 := dial()
	drain(conn1, "mybox terminal")
	send(conn1, `{"type":"input","data":"sh -c \"printf '%b' '\\033[?1003h'; sleep 30\"\r"}`)
	drain(conn1, "\x1b[?1003h")
	require.NoError(t, conn1.Close())
	time.Sleep(200 * time.Millisecond)

	// Reattach while the foreground program is still running: the enable must
	// be re-asserted and never followed by a reset that would choke the app's
	// mouse input. Consume the whole replay+sync stream so a reset hiding in a
	// later message is caught rather than masked by the ring's earlier enable.
	conn2 := dial()
	defer conn2.Close()
	sawReassert := false
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		_ = conn2.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		_, data, err := conn2.ReadMessage()
		if err != nil {
			break
		}
		msg := string(data)
		if strings.Contains(msg, "[?1003l") {
			t.Fatalf("mouse tracking was reset while a foreground app still needs it")
		}
		if strings.Contains(msg, "[?1003h") {
			sawReassert = true
		}
	}
	require.True(t, sawReassert,
		"mouse-tracking mode was not re-asserted for the still-running app")
}

// TestTerminalReattachCommandShellKeepsAppMouseMode covers the case where the
// persistent session was started with a `command` (the frontend's "new
// terminal with command" flow): the shell runs `$SHELL -c <command>`, so the
// foreground application (here a mouse-enabling TUI that stays alive) shares
// the shell's process group and `tcgetpgrp` looks exactly like an idle shell.
// The reattach logic must still recognize that something is running and keep
// its mouse mode instead of resetting it away.
func TestTerminalReattachCommandShellKeepsAppMouseMode(t *testing.T) {
	s, app := newTestServer(t, false)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	cmd := url.QueryEscape(`printf '%b' '\033[?1003h'; sleep 30`)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") +
		"/api/terminal?project=" + app.Project.Name + "&session=sess-cmd&command=" + cmd

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		return conn
	}
	recv := func(conn *websocket.Conn) string {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		return string(msg)
	}
	drain := func(conn *websocket.Conn, sub string) {
		t.Helper()
		for msg := recv(conn); !strings.Contains(msg, sub); msg = recv(conn) {
		}
	}

	conn1 := dial()
	drain(conn1, "\x1b[?1003h")
	require.NoError(t, conn1.Close())
	time.Sleep(200 * time.Millisecond)

	// The command shell is still alive running `sleep`, so mouse tracking must
	// be carried over — never reset away. Consume the whole replay+sync stream
	// (a reset would arrive as a later message, past the ring's enable).
	conn2 := dial()
	defer conn2.Close()
	sawReassert := false
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		_ = conn2.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		_, data, err := conn2.ReadMessage()
		if err != nil {
			break
		}
		msg := string(data)
		if strings.Contains(msg, "[?1003l") {
			t.Fatalf("mouse tracking was reset while the command shell still runs the app")
		}
		if strings.Contains(msg, "[?1003h") {
			sawReassert = true
		}
	}
	require.True(t, sawReassert,
		"mouse-tracking mode was not carried over for a command-started app")
}
