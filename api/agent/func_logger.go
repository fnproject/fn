package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
	logPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// setupLogger returns an io.ReadWriteCloser which may write to multiple io.Writer's,
// and may be read from the returned io.Reader (singular). After Close is called,
// the Reader is not safe to read from, nor the Writer to write to.
func setupLogger(logger logrus.FieldLogger) io.ReadWriteCloser {
	lbuf := bufPool.Get().(*bytes.Buffer)
	dbuf := logPool.Get().(*bytes.Buffer)

	close := func() error {
		// TODO we may want to toss out buffers that grow to grotesque size but meh they will prob get GC'd
		lbuf.Reset()
		dbuf.Reset()
		bufPool.Put(lbuf)
		logPool.Put(dbuf)
		return nil
	}

	// we don't need to limit the log writer, but we do need it to dispense lines
	linew := newLineWriterWithBuffer(lbuf, &logWriter{logger})

	const MB = 1 * 1024 * 1024 // pick a number any number.. TODO configurable ?

	// we don't need to log per line to db, but we do need to limit it
	limitw := &nopCloser{newLimitWriter(MB, dbuf)}

	// TODO / NOTE: we want linew to be first because limitw may error if limit
	// is reached but we still want to log. we should probably ignore hitting the
	// limit error since we really just want to not write too much to db and
	// that's handled as is. put buffers back last to avoid misuse, if there's
	// an error they won't get put back and that's really okay too.
	mw := multiWriteCloser{linew, limitw, &fCloser{close}}
	return &rwc{mw, dbuf}
}

// implements io.ReadWriteCloser, keeps the buffer for all its handy methods
type rwc struct {
	io.WriteCloser
	*bytes.Buffer
}

// these are explicit to override the *bytes.Buffer's methods
func (r *rwc) Write(b []byte) (int, error) { return r.WriteCloser.Write(b) }
func (r *rwc) Close() error                { return r.WriteCloser.Close() }

// implements passthrough Write & closure call in Close
type fCloser struct {
	close func() error
}

func (f *fCloser) Write(b []byte) (int, error) { return len(b), nil }
func (f *fCloser) Close() error                { return f.close() }

type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error { return nil }

// multiWriteCloser returns the first write or close that returns a non-nil
// err, if no non-nil err is returned, then the returned bytes written will be
// from the last call to write.
type multiWriteCloser []io.WriteCloser

func (m multiWriteCloser) Write(b []byte) (n int, err error) {
	for _, mw := range m {
		n, err = mw.Write(b)
		if err != nil {
			return n, err
		}
	}
	return n, err
}

func (m multiWriteCloser) Close() (err error) {
	for _, mw := range m {
		err = mw.Close()
		if err != nil {
			return err
		}
	}
	return err
}

// logWriter will log (to real stderr) every call to Write as a line. it should
// be wrapped with a lineWriter so that the output makes sense.
type logWriter struct {
	logrus.FieldLogger
}

func (l *logWriter) Write(b []byte) (int, error) {
	l.Debug(string(b))
	return len(b), nil
}

// lineWriter buffers all calls to Write and will call Write
// on the underlying writer once per new line. Close must
// be called to ensure that the buffer is flushed, and a newline
// will be appended in Close if none is present.
type lineWriter struct {
	b *bytes.Buffer
	w io.Writer
}

func newLineWriter(w io.Writer) io.WriteCloser {
	return &lineWriter{b: new(bytes.Buffer), w: w}
}

func newLineWriterWithBuffer(b *bytes.Buffer, w io.Writer) io.WriteCloser {
	return &lineWriter{b: b, w: w}
}

func (li *lineWriter) Write(ogb []byte) (int, error) {
	li.b.Write(ogb) // bytes.Buffer is guaranteed, read it!

	for {
		b := li.b.Bytes()
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break // no more newlines in buffer
		}

		// write in this line and advance buffer past it
		l := b[:i+1]
		ns, err := li.w.Write(l)
		if err != nil {
			return ns, err
		}
		li.b.Next(len(l))
	}

	// technically we wrote all the bytes, so make things appear normal
	return len(ogb), nil
}

func (li *lineWriter) Close() error {
	// flush the remaining bytes in the buffer to underlying writer, adding a
	// newline if needed
	b := li.b.Bytes()
	if len(b) == 0 {
		return nil
	}

	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	_, err := li.w.Write(b)
	return err
}

// io.Writer that allows limiting bytes written to w
type limitWriter struct {
	n, max int
	io.Writer
}

func newLimitWriter(max int, w io.Writer) io.Writer {
	return &limitWriter{max: max, Writer: w}
}

func (l *limitWriter) Write(b []byte) (int, error) {
	if l.n >= l.max {
		return 0, errors.New("max log size exceeded, truncating log")
	}
	if l.n+len(b) >= l.max {
		// cut off to prevent gigantic line attack
		b = b[:l.max-l.n]
	}
	n, err := l.Writer.Write(b)
	l.n += n
	if l.n >= l.max {
		// write in truncation message to log once
		l.Writer.Write([]byte(fmt.Sprintf("\n-----max log size %d bytes exceeded, truncating log-----\n", l.max)))
	}
	return n, err
}
