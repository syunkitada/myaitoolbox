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
