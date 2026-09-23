package entrypoint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOsc52Stripper(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text passes", "hello world", "hello world"},
		{"osc52 dropped bel", "\x1b]52;c;SGVsbG8=\x07", ""},
		{"osc52 dropped st", "\x1b]52;c;SGVsbG8=\x1b\\", ""},
		{"osc52 splitted across calls", "\x1b]52;c;SGVsbG8=", ""},
		{"other osc passes bel", "\x1b]2;title\x07", "\x1b]2;title\x07"},
		{"other osc passes st", "\x1b]0;t\x1b\\", "\x1b]0;t\x1b\\"},
		{"osc52 interleaved with text", "a\x1b]52;c;eA==\x07b", "ab"},
		{"csi not osc", "\x1b[?1003h", "\x1b[?1003h"},
		{"esc single byte not osc", "\x1bM", "\x1bM"},
		{"command 5 not 52", "\x1b]5;\x07", "\x1b]5;\x07"},
		{"non-numeric partly matches", "\x1b]52x\x07", "\x1b]52x\x07"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &osc52Stripper{}
			got := string(st.filter([]byte(tt.in)))
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOsc52StripperSplitAcrossCalls(t *testing.T) {
	st := &osc52Stripper{}
	chunks := [][]byte{
		[]byte("a\x1b]5"),
		[]byte("2;c;SG"),
		[]byte("VsbG8=\x07"),
		[]byte("b"),
	}
	var got []byte
	for _, c := range chunks {
		got = append(got, st.filter(c)...)
	}
	require.Equal(t, "ab", string(got))

	// Reuse: the stripper state must not leak across sessions.
	st2 := &osc52Stripper{}
	require.Equal(t, "keep\x1b]0;t\x07",
		string(st2.filter([]byte("keep\x1b]0;t\x07"))))
}

func TestOsc52StripperStateAcrossSessions(t *testing.T) {
	st := &osc52Stripper{}
	// First session: a 52 sequence is split mid-argument.
	st.filter([]byte("\x1b]52;c;"))
	// A second session reuses a fresh instance so nothing carries over.
	st2 := &osc52Stripper{}
	require.Equal(t, "x",
		string(st2.filter([]byte("x\x1b]52;c;YQ==\x07"))))
}

// TestOsc52RingVsLive verifies that live clients receive OSC 52 verbatim while
// the ring buffer (replayed to reloading clients) is stripped of it.
func TestOsc52RingVsLive(t *testing.T) {
	s := &terminalSession{
		clients: map[*terminalClient]struct{}{},
		modes:   newTerminalModes(),
	}
	live := &terminalClient{send: make(chan []byte, 8), stop: make(chan struct{})}
	s.clients[live] = struct{}{}

	// OSC 52 split across two broadcasts, plus an OSC 0 title that must survive.
	s.broadcast([]byte("a\x1b]52;c;SGVsbG8=\x07b\x1b]0;t\x07"))

	// Live: raw chunks.
	require.Equal(t, "a\x1b]52;c;SGVsbG8=\x07b\x1b]0;t\x07", string(<-live.send))

	// Ring: OSC 52 gone, OSC 0 intact.
	s.broadcast([]byte("\x1b]52;c;ZXh0cmE=\x07c"))
	var ring []byte
	for _, chunk := range s.ring {
		ring = append(ring, chunk...)
	}
	require.Equal(t, "ab\x1b]0;t\x07c", string(ring))
	require.NotContains(t, string(ring), "\x1b]52", "ring must be stripped of OSC 52")

	// A reattaching client replays only the stripped ring.
	var replayed []byte
	for _, chunk := range s.ring {
		replayed = append(replayed, chunk...)
	}
	require.NotContains(t, string(replayed), "\x1b]52", "replay must not contain OSC 52")
}
