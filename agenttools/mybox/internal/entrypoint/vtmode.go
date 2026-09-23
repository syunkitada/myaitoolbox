package entrypoint

import (
	"bytes"
	"strconv"
)

// terminalModes records the terminal-emulator modes observed in the raw PTY
// output (the escape sequences emitted by the shell or a foreground
// application) so that a client reattaching to a persistent session can be
// brought back in sync with what the running application expects.
//
// A fresh xterm.js in the browser only learns about e.g. mouse tracking from
// the escape sequences it receives. The ring buffer replayed to a late
// attachment may no longer contain an application's one-shot startup
// sequences (it enabled mouse tracking long ago and never re-emits it), so
// replaying history alone is not enough — the current mode state must be
// re-asserted on top. Drawing and cursor-positioning sequences are skipped;
// only the modes that govern input handling and cursor appearance are kept.
//
// The parser is a minimal VT state machine: it buffers incomplete escape
// sequences across feed() calls because PTY reads can split anywhere.
type terminalModes struct {
	modes map[int]bool // DECSET/DECRST private modes currently set (e.g. 1003)

	cursorStyle string // DECSCUSR parameter, e.g. "0", "5"; "" = unknown/default
	modifyOther int    // XTerm modifyOtherKeys level (0 = off)

	// Parser state, kept between feed() calls.
	state byte
	buf   []byte
}

const (
	vtIdle = iota
	vtEsc
	vtCsi
	vtOsc
	vtOscEsc
)

func newTerminalModes() *terminalModes {
	return &terminalModes{modes: map[int]bool{25: true}, cursorStyle: ""}
}

// feed processes a chunk of raw PTY output and updates the tracked state.
func (t *terminalModes) feed(data []byte) {
	combined := make([]byte, 0, len(data)+len(t.buf))
	combined = append(combined, t.buf...)
	combined = append(combined, data...)
	t.buf = t.buf[:0]

	for i := 0; i < len(combined); {
		b := combined[i]
		switch t.state {
		case vtIdle:
			if b == 0x1b {
				t.state = vtEsc
			}
			i++
		case vtEsc:
			switch b {
			case '[':
				t.state = vtCsi
				t.buf = t.buf[:0]
			case ']':
				t.state = vtOsc
				t.buf = t.buf[:0]
			default:
				// ESC followed by a single byte (e.g. ESC M, ESC 7, charset
				// selection) — nothing we track.
				t.state = vtIdle
			}
			i++
		case vtCsi:
			t.buf = append(t.buf, b)
			i++
			if b >= 0x40 && b <= 0x7e { // final byte
				t.parseCSI(t.buf)
				t.end()
			}
		case vtOsc:
			t.buf = append(t.buf, b)
			i++
			switch b {
			case 0x07: // BEL terminates OSC
				t.end()
			case 0x1b:
				t.state = vtOscEsc
			}
		case vtOscEsc:
			t.buf = append(t.buf, b)
			i++
			if b == '\\' { // ST (ESC \) terminates OSC
				t.end()
			} else {
				t.state = vtOsc
			}
		}
	}
}

func (t *terminalModes) end() {
	t.state = vtIdle
	t.buf = t.buf[:0]
}

// parseCSI handles a complete CSI sequence (the leading ESC [ is excluded).
func (t *terminalModes) parseCSI(csi []byte) {
	if len(csi) < 2 {
		return
	}
	final := csi[len(csi)-1]
	body := csi[:len(csi)-1]
	if len(body) == 0 || body[0] == '<' || body[0] == '=' {
		return
	}

	params := parseCSIParams(body)
	if len(params) == 0 {
		return
	}

	switch final {
	case 'h', 'l': // DECSET/DECRST — CSI ? Ps ; Ps h/l
		if body[0] != '?' {
			return
		}
		for _, p := range params {
			if p < 0 {
				continue
			}
			t.modes[p] = final == 'h'
		}
	case 'm': // DECKPM — CSI > Ps ; Ps m, e.g. >4;2 (modifyOtherKeys)
		if body[0] != '>' {
			return
		}
		if len(params) >= 2 && params[0] == 4 {
			t.modifyOther = params[len(params)-1]
		}
	case 'q': // DECSCUSR — CSI Ps SP q
		last := body[len(body)-1]
		if last != ' ' && last != '"' {
			return
		}
		if len(params) > 0 && params[len(params)-1] >= 0 {
			t.cursorStyle = strconv.Itoa(params[len(params)-1])
		}
	}
}

// parseCSIParams extracts the numeric parameters from a CSI body, ignoring the
// private/intermediate leader characters ('?', '>', ' ', ...).
func parseCSIParams(body []byte) []int {
	var out []int
	n := -1
	digit := false
	for _, c := range body {
		switch {
		case c >= '0' && c <= '9':
			if n < 0 {
				n = 0
			}
			n = n*10 + int(c-'0')
			digit = true
		case c == ';':
			out = append(out, n)
			n = -1
			digit = false
		}
	}
	if digit || n >= 0 {
		out = append(out, n)
	}
	return out
}

// trackedModes lists the private modes reconcile() manages. Unknown modes are
// reported as off so a fresh client never inherits a stale "on" from the ring
// replay; 1049 (alternate screen) and 2026 (synchronized updates) are excluded
// on purpose — forcing them can clear or freeze the visible screen.
var trackedModes = []int{9, 25, 1000, 1002, 1003, 1004, 1005, 1006, 1015, 1016, 2004}

// reconcile returns the escape sequences that bring a fresh terminal into the
// tracked state of a running application. It is used when a process other than
// the shell owns the foreground, so mouse reporting, bracketed paste, cursor
// appearance etc. match exactly what that application enabled.
func (t *terminalModes) reconcile() []byte {
	var b bytes.Buffer
	for _, m := range trackedModes {
		if m == 1006 || m == 1016 {
			// 1006 (SGR) and 1016 (SGR pixels) select the same encoding in
			// xterm; DECRST of either one resets encoding back to X10. A plain
			// "assert inactive as l" for 1016 would silently nullify a running
			// app's SGR encoding set by 1006 h earlier in the loop, so the
			// active encoding is declared once below instead.
			continue
		}
		b.WriteString("\x1b[?")
		b.WriteString(strconv.Itoa(m))
		if t.modes[m] {
			b.WriteByte('h')
		} else {
			b.WriteByte('l')
		}
	}
	// The active encoding, asserted as a single selector.
	switch {
	case t.modes[1016]:
		b.WriteString("\x1b[?1016h") // SGR pixels
	case t.modes[1006]:
		b.WriteString("\x1b[?1006h") // SGR
	default:
		b.WriteString("\x1b[?1006l") // X10, xterm's baseline
	}
	if t.cursorStyle == "" {
		b.WriteString("\x1b[0 q")
	} else {
		b.WriteString("\x1b[")
		b.WriteString(t.cursorStyle)
		b.WriteString(" q")
	}
	if t.modifyOther <= 0 {
		b.WriteString("\x1b[>4;0m")
	} else {
		b.WriteString("\x1b[>4;")
		b.WriteString(strconv.Itoa(t.modifyOther))
		b.WriteByte('m')
	}
	return b.Bytes()
}

// terminalShellReset returns the terminal to the state of an idle shell: every
// input mode off, cursor visible and default shaped, alternate screen left.
// It is applied when the shell itself owns the foreground, i.e. no application
// is running, so stale modes leftover from a killed or suspended program are
// cleared rather than resurrected by the ring replay. Bracketed paste (2004)
// is deliberately left alone — an idle shell (readline/zle) enables it itself
// and clearing it would make multi-line pastes execute immediately.
var terminalShellReset = []byte(
	"\x1b[?9l\x1b[?25h\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1004l\x1b[?1005l" +
		"\x1b[?1006l\x1b[?1015l\x1b[?1016l\x1b[?1049l\x1b[?2026l\x1b[0 q\x1b[>4;0m")
