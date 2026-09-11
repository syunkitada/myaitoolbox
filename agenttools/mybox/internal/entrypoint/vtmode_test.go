package entrypoint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminalModesFeed(t *testing.T) {
	m := newTerminalModes()

	m.feed([]byte("plain text \x1b[?1003h hello"))
	require.True(t, m.modes[1003], "1003 should be enabled")
	require.False(t, m.modes[1000], "1000 should stay off")

	m.feed([]byte("\x1b[?1000h\x1b[?1002h\x1b[?2004h"))
	require.True(t, m.modes[1000])
	require.True(t, m.modes[1002])
	require.True(t, m.modes[2004])

	m.feed([]byte("\x1b[?1003l"))
	require.False(t, m.modes[1003], "1003 should be disabled after DECRST")

	// DECSCUSR: CSI Ps SP q
	m.feed([]byte("\x1b[5 q"))
	require.Equal(t, "5", m.cursorStyle)

	// modifyOtherKeys: CSI > 4 ; n m
	m.feed([]byte("\x1b[>4;2m"))
	require.Equal(t, 2, m.modifyOther)
	m.feed([]byte("\x1b[>4;0m"))
	require.Equal(t, 0, m.modifyOther)

	// Cursor visibility tracking via DECSET 25.
	require.True(t, m.modes[25], "cursor defaults to visible")
	m.feed([]byte("\x1b[?25l"))
	require.False(t, m.modes[25])
}

func TestTerminalModesSplitSequences(t *testing.T) {
	m := newTerminalModes()

	// A DECSET split so the mode number straddles two reads.
	m.feed([]byte("\x1b[?10"))
	require.False(t, m.modes[1002], "incomplete sequence must not be applied yet")
	m.feed([]byte("02h"))
	require.True(t, m.modes[1002])

	// Split at the very start (ESC alone in one read).
	m.feed([]byte("\x1b"))
	m.feed([]byte("[?1003h rest"))
	require.True(t, m.modes[1003])

	// DECSCUSR split before the final byte.
	m.feed([]byte("\x1b[3 "))
	m.feed([]byte("q"))
	require.Equal(t, "3", m.cursorStyle)
}

func TestTerminalModesIgnoresOSC(t *testing.T) {
	m := newTerminalModes()

	// An OSC title that embeds a CSI-looking string must not confuse the
	// parser; the mode toggle inside the title is inert text.
	m.feed([]byte("\x1b]0;\x1b[?1003h\x07"))
	require.False(t, m.modes[1003], "mode toggle inside OSC must be ignored")

	// OSC terminated by ST (ESC \).
	m.feed([]byte("\x1b]0;title\x1b[?1002h\x1b\\"))
	require.False(t, m.modes[1002])
}

func TestTerminalModesReconcile(t *testing.T) {
	m := newTerminalModes()
	m.feed([]byte("\x1b[?1003h\x1b[?1006h\x1b[?2004h"))
	got := string(m.reconcile())

	require.Contains(t, got, "\x1b[?1003h", "running app's 1003 must be re-asserted on")
	require.Contains(t, got, "\x1b[?1006h")
	require.Contains(t, got, "\x1b[?2004h")
	require.Contains(t, got, "\x1b[?1000l", "off modes must be asserted off")
	require.Contains(t, got, "\x1b[?25h", "cursor defaults visible")
	require.Contains(t, got, "\x1b[0 q", "default cursor style when unknown")
	require.Contains(t, got, "\x1b[>4;0m", "modifyOtherKeys off by default")

	// The alternate-screen (1049) and sync (2026) modes are never asserted so
	// they cannot clear or freeze the visible screen.
	require.NotContains(t, got, "1049")
	require.NotContains(t, got, "2026")

	// SGR (1006) must not be undone by asserting 1016 (SGR pixels) as "off":
	// xterm treats DECRST 1006/1016 as "reset encoding", clobbering it.
	require.NotContains(t, got, "\x1b[?1016l", "1016 reset must not undo SGR encoding")
}

func TestTerminalModesReconcileCursorStyle(t *testing.T) {
	m := newTerminalModes()
	m.feed([]byte("\x1b[?1003h\x1b[6 q\x1b[>4;2m"))
	got := string(m.reconcile())
	require.Contains(t, got, "\x1b[6 q")
	require.Contains(t, got, "\x1b[>4;2m")
}

func TestTerminalModesReconcileEncoding(t *testing.T) {
	// SGR pixels (1016) selected by the app must be asserted, not SGR.
	m := newTerminalModes()
	m.feed([]byte("\x1b[?1003h\x1b[?1016h"))
	got := string(m.reconcile())
	require.Contains(t, got, "\x1b[?1016h")
	require.NotContains(t, got, "\x1b[?1006h")

	// SGR (1006) selected: exactly one encoding selector, no 1016 reset after it.
	m = newTerminalModes()
	m.feed([]byte("\x1b[?1003h\x1b[?1006h"))
	got = string(m.reconcile())
	i1006 := indexOf(got, "\x1b[?1006h")
	i1016 := indexOf(got, "1016")
	require.Contains(t, got, "\x1b[?1006h")
	require.True(t, i1006 >= 0, "SGR selector present")
	require.Equal(t, -1, i1016, "no 1016 assertion alongside 1006")

	// Neither encoding set: falls back to X10 reset only.
	m = newTerminalModes()
	m.feed([]byte("\x1b[?1003h"))
	got = string(m.reconcile())
	require.Contains(t, got, "\x1b[?1006l")
	require.NotContains(t, got, "\x1b[?1006h")
	require.NotContains(t, got, "\x1b[?1016h")
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
