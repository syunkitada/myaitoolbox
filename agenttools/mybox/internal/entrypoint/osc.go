package entrypoint

// osc52Stripper removes OSC 52 (clipboard) sequences from a byte stream.
//
// OSC 52 asks the emulator to put text on the system clipboard
// (`ESC ] 52 ; c ; <base64> ST|BEL`). The frontend answers by showing a
// "Copy from terminal" modal so the user can accept or reject the paste. The
// payload is not part of the visible screen state, so carrying it over in the
// ring buffer would make a client that reloads the page re-open a modal for a
// selection made long before the reload. Live broadcasts are left untouched
// (an active copy still shows the modal); only the ring is filtered.
//
// The filter is stateful because PTY reads can split a sequence at any byte.
type osc52Stripper struct {
	state int
	osc   []byte // digits of the OSC command of the sequence being examined
}

const (
	oscIdle    = iota
	oscEsc     // saw ESC, next byte decides whether an OSC begins
	oscIn      // inside ESC ] …, buffering the command digits
	oscInEsc   // inside ESC ] …, saw ESC (possible ST terminator)
	oscPass    // passing through a known non-52 OSC until its terminator
	oscPassEsc // passing through a non-52 OSC, saw ESC
	oscDrop    // dropping a confirmed OSC 52 sequence until its terminator
	oscDropEsc // dropping an OSC 52 sequence, saw ESC
)

func (o *osc52Stripper) filter(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for _, b := range data {
		switch o.state {
		case oscIdle:
			if b == 0x1b {
				o.state = oscEsc
			} else {
				out = append(out, b)
			}
		case oscEsc:
			if b == ']' {
				o.state = oscIn
				o.osc = o.osc[:0]
			} else {
				out = append(out, 0x1b, b)
				o.state = oscIdle
			}
		case oscIn:
			switch {
			case b >= '0' && b <= '9':
				o.osc = append(o.osc, b)
			case b == ';':
				if string(o.osc) == "52" {
					o.state = oscDrop
				} else {
					out = append(out, 0x1b, ']')
					out = append(out, o.osc...)
					out = append(out, ';')
					o.state = oscPass
				}
			case b == 0x07: // BEL terminates an OSC
				out = append(out, 0x1b, ']')
				out = append(out, o.osc...)
				out = append(out, b)
				o.state = oscIdle
			case b == 0x1b:
				o.state = oscInEsc
			default: // not a digit/`;`/terminator: not a command we track
				out = append(out, 0x1b, ']')
				out = append(out, o.osc...)
				out = append(out, b)
				o.state = oscPass
			}
		case oscInEsc:
			if b == '\\' { // ST terminates an OSC
				out = append(out, 0x1b, ']')
				out = append(out, o.osc...)
				out = append(out, 0x1b, b)
				o.state = oscIdle
			} else {
				// The ESC was not an ST; treat it as part of the examined
				// command region and re-process this byte there.
				out = append(out, o.filter([]byte{b})...)
			}
		case oscPass:
			switch b {
			case 0x07:
				out = append(out, b)
				o.state = oscIdle
			case 0x1b:
				out = append(out, b)
				o.state = oscPassEsc
			default:
				out = append(out, b)
			}
		case oscPassEsc:
			out = append(out, b)
			if b == '\\' {
				o.state = oscIdle
			} else {
				o.state = oscPass
			}
		case oscDrop:
			switch b {
			case 0x1b:
				o.state = oscDropEsc
			case 0x07:
				o.state = oscIdle
			}
		case oscDropEsc:
			if b == '\\' {
				o.state = oscIdle
			} else {
				o.state = oscDrop
			}
		}
	}
	return out
}
