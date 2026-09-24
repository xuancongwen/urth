package telnet

import (
	"bufio"
	"errors"
	"io"
)

// Telnet protocol bytes we need to recognise in order to ignore them.
const (
	iac  = 255 // interpret as command
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250 // subnegotiation begin
	se   = 240 // subnegotiation end
)

// MaxLineLen caps a single input line. Longer lines are truncated.
const MaxLineLen = 512

// ErrLineTooLong is returned by ReadLine when the line exceeded MaxLineLen. The
// truncated line is still returned.
var ErrLineTooLong = errors.New("line too long")

// LineReader reads newline-terminated text from a telnet client while
// discarding option negotiation. It does not negotiate anything itself: nearly
// every MUD client is happy to talk plain text.
type LineReader struct {
	r *bufio.Reader
}

// NewLineReader wraps r.
func NewLineReader(r io.Reader) *LineReader {
	return &LineReader{r: bufio.NewReaderSize(r, 1024)}
}

// ReadLine returns the next line with CR, LF, NUL and telnet commands removed.
// It returns io.EOF when the client hangs up. A line that exceeds MaxLineLen
// is returned truncated along with ErrLineTooLong; the rest of that line is
// discarded.
func (lr *LineReader) ReadLine() (string, error) {
	buf := make([]byte, 0, 80)
	tooLong := false
	for {
		b, err := lr.r.ReadByte()
		if err != nil {
			if len(buf) > 0 && err == io.EOF {
				// Client sent a partial line then hung up. Deliver what we have.
				return string(buf), nil
			}
			return "", err
		}
		switch b {
		case '\n':
			if tooLong {
				return string(buf), ErrLineTooLong
			}
			return string(buf), nil
		case '\r', 0:
			continue
		case iac:
			if err := lr.skipCommand(); err != nil {
				return "", err
			}
			// An escaped IAC IAC would be a literal 255; drop it, not text.
			continue
		default:
			if len(buf) >= MaxLineLen {
				tooLong = true
				continue
			}
			buf = append(buf, b)
		}
	}
}

// skipCommand consumes the bytes following an IAC.
func (lr *LineReader) skipCommand() error {
	cmd, err := lr.r.ReadByte()
	if err != nil {
		return err
	}
	switch cmd {
	case will, wont, do, dont:
		_, err = lr.r.ReadByte() // option byte
		return err
	case sb:
		// Discard until IAC SE.
		prevIAC := false
		for {
			b, err := lr.r.ReadByte()
			if err != nil {
				return err
			}
			if prevIAC && b == se {
				return nil
			}
			prevIAC = b == iac && !prevIAC
		}
	default:
		// Two-byte command (NOP, GA, AYT, escaped IAC, ...). Already consumed.
		return nil
	}
}
