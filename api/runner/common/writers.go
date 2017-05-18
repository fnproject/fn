package common

import (
	"bytes"
	"errors"
	"io"
)

// lineWriter will break apart a stream of data into individual lines.
// Downstream writer will be called for each complete new line. When Flush
// is called, a newline will be appended if there isn't one at the end.
// Not thread-safe
type LineWriter struct {
	b *bytes.Buffer
	w io.Writer
}

func NewLineWriter(w io.Writer) *LineWriter {
	return &LineWriter{
		w: w,
		b: bytes.NewBuffer(make([]byte, 0, 1024)),
	}
}

func (li *LineWriter) Write(p []byte) (int, error) {
	n, err := li.b.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, errors.New("short write")
	}

	for {
		b := li.b.Bytes()
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break
		}

		l := b[:i+1]
		ns, err := li.w.Write(l)
		if err != nil {
			return ns, err
		}
		li.b.Next(len(l))
	}

	return n, nil
}

func (li *LineWriter) Flush() (int, error) {
	b := li.b.Bytes()
	if len(b) == 0 {
		return 0, nil
	}

	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return li.w.Write(b)
}

// HeadLinesWriter stores upto the first N lines in a buffer that can be
// retrieved via Head().
type HeadLinesWriter struct {
	buffer bytes.Buffer
	max    int
}

func NewHeadLinesWriter(max int) *HeadLinesWriter {
	return &HeadLinesWriter{
		buffer: bytes.Buffer{},
		max:    max,
	}
}

// Writes start failing once the writer has reached capacity.
// In such cases the return value is the actual count written (may be zero) and io.ErrShortWrite.
func (h *HeadLinesWriter) Write(p []byte) (n int, err error) {
	var afterNewLine int
	for h.max > 0 && afterNewLine < len(p) {
		idx := bytes.IndexByte(p[afterNewLine:], '\n')
		if idx == -1 {
			h.buffer.Write(p[afterNewLine:])
			afterNewLine = len(p)
		} else {
			h.buffer.Write(p[afterNewLine : afterNewLine+idx+1])
			afterNewLine = afterNewLine + idx + 1
			h.max--
		}
	}

	if afterNewLine == len(p) {
		return afterNewLine, nil
	}

	return afterNewLine, io.ErrShortWrite
}

// The returned bytes alias the buffer, the same restrictions as
// bytes.Buffer.Bytes() apply.
func (h *HeadLinesWriter) Head() []byte {
	return h.buffer.Bytes()
}

// TailLinesWriter stores upto the last N lines in a buffer that can be retrieved
// via Tail(). The truncation is only performed when more bytes are received
// after '\n', so the buffer contents for both these writes are identical.
//
// tail writer that captures last 3 lines.
// 'a\nb\nc\nd\n' -> 'b\nc\nd\n'
// 'a\nb\nc\nd' -> 'b\nc\nd'
type TailLinesWriter struct {
	buffer             bytes.Buffer
	max                int
	newlineEncountered bool
	// Tail is not idempotent without this.
	tailCalled bool
}

func NewTailLinesWriter(max int) *TailLinesWriter {
	return &TailLinesWriter{
		buffer: bytes.Buffer{},
		max:    max,
	}
}

// Write always succeeds! This is because all len(p) bytes are written to the
// buffer before it is truncated.
func (t *TailLinesWriter) Write(p []byte) (n int, err error) {
	if t.tailCalled {
		return 0, errors.New("Tail() has already been called.")
	}

	var afterNewLine int
	for afterNewLine < len(p) {
		// This is at the top of the loop so it does not operate on trailing
		// newlines. That is handled by Tail() where we have full knowledge that it
		// is indeed the true trailing newline (if any).
		if t.newlineEncountered {
			if t.max > 0 {
				// we still have capacity
				t.max--
			} else {
				// chomp a newline.
				t.chompNewline()
			}
		}

		idx := bytes.IndexByte(p[afterNewLine:], '\n')
		if idx == -1 {
			t.buffer.Write(p[afterNewLine:])
			afterNewLine = len(p)
			t.newlineEncountered = false
		} else {
			t.buffer.Write(p[afterNewLine : afterNewLine+idx+1])
			afterNewLine = afterNewLine + idx + 1
			t.newlineEncountered = true
		}

	}
	return len(p), nil
}

func (t *TailLinesWriter) chompNewline() {
	b := t.buffer.Bytes()
	idx := bytes.IndexByte(b, '\n')
	if idx >= 0 {
		t.buffer.Next(idx + 1)
	} else {
		// pretend a trailing newline exists. In the call in Write() this will
		// never be hit.
		t.buffer.Truncate(0)
	}
}

// The returned bytes alias the buffer, the same restrictions as
// bytes.Buffer.Bytes() apply.
//
// Once Tail() is called, further Write()s error.
func (t *TailLinesWriter) Tail() []byte {
	if !t.tailCalled {
		t.tailCalled = true
		if t.max <= 0 {
			t.chompNewline()
		}
	}
	return t.buffer.Bytes()
}
