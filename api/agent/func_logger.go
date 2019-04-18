package agent

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"

	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// the returned writer writes bytes per line to stderr
func setupLogger(c *models.Call, level string) io.WriteCloser {
	lbuf := bufPool.Get().(*bytes.Buffer)

	close := func() {
		// TODO we may want to toss out buffers that grow to grotesque size but meh they will prob get GC'd
		lbuf.Reset()
		bufPool.Put(lbuf)
	}

	stderrLogger := logrus.WithFields(logrus.Fields{"user_log": true, "app_id": c.AppID, "fn_id": c.FnID, "image": c.Image, "call_id": c.ID})
	loggo := newLogWriter(stderrLogger, level)
	linew := newLineWriterWithBuffer(lbuf, loggo)
	return &fCloser{
		Writer: linew,
		close: func() error {
			err := linew.Close()
			close()
			return err
		},
	}
}

// implements passthrough WriteCloser with overwritable Close
type fCloser struct {
	io.Writer
	close func() error
}

func (f *fCloser) Close() error { return f.close() }

type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error { return nil }

// logWriter will log (to real stderr) every call to Write as a line. it should
// be wrapped with a lineWriter so that the output makes sense.
type logWriter struct {
	level  logrus.Level
	logger logrus.FieldLogger
	closed uint32
}

func newLogWriter(logger logrus.FieldLogger, level string) io.WriteCloser {
	lv, err := logrus.ParseLevel(level)
	if err != nil {
		// TODO(reed): we should do this at the config level instead? it's an optional option to begin with tho (StderrLogger)
		lv = logrus.InfoLevel // default
	}

	return &logWriter{logger: logger, level: lv}
}

func (l *logWriter) Write(b []byte) (int, error) {
	if atomic.LoadUint32(&l.closed) == 1 {
		// we don't want to return 0/error or the container will get shut down
		return len(b), nil
	}
	l.logger.WithFields(nil).Log(l.level, string(b))
	return len(b), nil
}

func (l *logWriter) Close() error {
	atomic.StoreUint32(&l.closed, 1)
	return nil
}

// lineWriter buffers all calls to Write and will call Write
// on the underlying writer once per new line. Close must
// be called to ensure that the buffer is flushed, and a newline
// will be appended in Close if none is present.
// TODO(reed): is line writer is vulnerable to attack?
type lineWriter struct {
	b      *bytes.Buffer
	w      io.WriteCloser
	closed uint32
}

func newLineWriter(w io.WriteCloser) io.WriteCloser {
	return &lineWriter{b: new(bytes.Buffer), w: w}
}

func newLineWriterWithBuffer(b *bytes.Buffer, w io.WriteCloser) io.WriteCloser {
	return &lineWriter{b: b, w: w}
}

func (li *lineWriter) Write(ogb []byte) (int, error) {
	if atomic.LoadUint32(&li.closed) == 1 {
		// we don't want to return 0/error or the container will shut down
		return len(ogb), nil
	}
	li.b.Write(ogb) // bytes.Buffer is guaranteed, read it!

	var n int
	for {
		// read the line and advance buffer past it
		l, err := li.b.ReadBytes('\n')
		if err != nil {
			break // no more newlines in buffer (see ReadBytes contract)
		}

		// write in the line
		ns, err := li.w.Write(l)
		n += ns
		if err != nil {
			return n, err
		}
	}

	// technically we wrote all the bytes, so make things appear normal
	return len(ogb), nil
}

func (li *lineWriter) Close() error {
	atomic.StoreUint32(&li.closed, 1)

	defer li.w.Close() // MUST close this (after writing last line)

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
