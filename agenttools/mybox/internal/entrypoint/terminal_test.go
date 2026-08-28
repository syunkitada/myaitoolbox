package entrypoint

import (
	"net/http"
	"net/http/httptest"
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

func TestTerminalReadOnly(t *testing.T) {	s, _ := newTestServer(t, true)

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
