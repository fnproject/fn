package common

import (
	"io"
	"sync"
)

type clampWriter struct {
	w           io.Writer
	remaining   int64
	overflowErr error
}

func NewClampWriter(buf io.Writer, max uint64, overflowErr error) io.Writer {
	if max != 0 {
		return &clampWriter{w: buf, remaining: int64(max), overflowErr: overflowErr}
	}
	return buf
}

func (g *clampWriter) Write(p []byte) (int, error) {
	if g.remaining <= 0 {
		return 0, g.overflowErr
	}
	if int64(len(p)) > g.remaining {
		p = p[0:g.remaining]
	}

	n, err := g.w.Write(p)
	g.remaining -= int64(n)
	if g.remaining <= 0 {
		err = g.overflowErr
	}
	return n, err
}

type clampReadCloser struct {
	r           io.ReadCloser
	remaining   int64
	overflowErr error
}

// ErrorCatchingReader exposes IO errors from wrapped readers - this captures the last UI error on a read and stores it
// This is useful for unmasking JSON IO errors on unbounded input
type ErrorCatchingReader interface {
	io.Reader
	LastError() error
}

type catchingReader struct {
	rdr io.ReadCloser
	err error
}

func NewErrorCatchingReader(rdr io.ReadCloser) ErrorCatchingReader {
	return &catchingReader{
		rdr: rdr,
	}
}
func (cr *catchingReader) LastError() error {
	return cr.err
}

func (cr *catchingReader) Close() error {
	return cr.rdr.Close()
}
func (cr *catchingReader) Read(p []byte) (n int, err error) {
	n, err = cr.rdr.Read(p)
	cr.err = err
	return
}
func NewClampReadCloser(buf io.ReadCloser, max uint64, overflowErr error) io.ReadCloser {
	if max != 0 {
		return &clampReadCloser{r: buf, remaining: int64(max), overflowErr: overflowErr}
	}
	return buf
}

func (g *clampReadCloser) Close() error {
	return g.r.Close()
}

func (g *clampReadCloser) Read(p []byte) (int, error) {
	if g.remaining <= 0 {
		return 0, g.overflowErr
	}
	if int64(len(p)) > g.remaining {
		p = p[0:g.remaining]
	}

	n, err := g.r.Read(p)
	g.remaining -= int64(n)
	return n, err
}

type GhostWriter interface {
	io.Writer
	io.Closer
	Swap(r io.Writer) io.Writer
}

// ghostWriter is an io.Writer who will pass writes to an inner writer
// that may be changed at will. it is thread safe to swap or write.
type ghostWriter struct {
	cond   *sync.Cond
	inner  io.Writer
	closed bool
}

func NewGhostWriter() GhostWriter {
	return &ghostWriter{cond: sync.NewCond(new(sync.Mutex)), inner: new(waitWriter)}
}

func (g *ghostWriter) Swap(w io.Writer) (old io.Writer) {
	g.cond.L.Lock()
	old = g.inner
	g.inner = w
	g.cond.L.Unlock()
	g.cond.Broadcast()
	return old
}

func (g *ghostWriter) Close() error {
	g.cond.L.Lock()
	g.closed = true
	g.cond.L.Unlock()
	g.cond.Broadcast()
	return nil
}

func (g *ghostWriter) awaitRealWriter() (io.Writer, bool) {
	// wait for a real writer
	g.cond.L.Lock()
	for {
		if g.closed { // check this first
			g.cond.L.Unlock()
			return nil, false
		}
		if _, ok := g.inner.(*waitWriter); ok {
			g.cond.Wait()
		} else {
			break
		}
	}

	// we don't need to serialize writes but swapping g.inner could be a race if unprotected
	w := g.inner
	g.cond.L.Unlock()
	return w, true
}

func (g *ghostWriter) Write(b []byte) (int, error) {
	w, ok := g.awaitRealWriter()
	if !ok {
		return 0, io.EOF
	}

	n, err := w.Write(b)
	if err == io.ErrClosedPipe {
		// NOTE: we need to mask this error so that docker does not get an error
		// from writing the input stream and shut down the container.
		err = nil
	}
	return n, err
}

type GhostReader interface {
	io.Reader
	io.Closer
	Swap(r io.Reader) io.Reader
}

// ghostReader is an io.ReadCloser who will pass reads to an inner reader
// that may be changed at will. it is thread safe to swap or read.
// Read will wait for a 'real' reader if inner is of type *waitReader.
// Close must be called to prevent any pending readers from leaking.
type ghostReader struct {
	cond   *sync.Cond
	inner  io.Reader
	closed bool
}

func NewGhostReader() GhostReader {
	return &ghostReader{cond: sync.NewCond(new(sync.Mutex)), inner: new(waitReader)}
}

func (g *ghostReader) Swap(r io.Reader) (old io.Reader) {
	g.cond.L.Lock()
	old = g.inner
	g.inner = r
	g.cond.L.Unlock()
	g.cond.Broadcast()
	return old
}

func (g *ghostReader) Close() error {
	g.cond.L.Lock()
	g.closed = true
	g.cond.L.Unlock()
	g.cond.Broadcast()
	return nil
}

func (g *ghostReader) awaitRealReader() (io.Reader, bool) {
	// wait for a real reader
	g.cond.L.Lock()
	for {
		if g.closed { // check this first
			g.cond.L.Unlock()
			return nil, false
		}
		if _, ok := g.inner.(*waitReader); ok {
			g.cond.Wait()
		} else {
			break
		}
	}

	// we don't need to serialize reads but swapping g.inner could be a race if unprotected
	r := g.inner
	g.cond.L.Unlock()
	return r, true
}

func (g *ghostReader) Read(b []byte) (int, error) {
	r, ok := g.awaitRealReader()
	if !ok {
		return 0, io.EOF
	}

	n, err := r.Read(b)
	if err == io.ErrClosedPipe {
		// NOTE: we need to mask this error so that docker does not get an error
		// from reading the input stream and shut down the container.
		err = nil
	}
	return n, err
}

// waitReader returns io.EOF if anyone calls Read. don't call Read, this is a sentinel type
type waitReader struct{}

func (e *waitReader) Read([]byte) (int, error) {
	panic("read on waitReader should not happen")
}

// waitWriter returns io.EOF if anyone calls Write. don't call Write, this is a sentinel type
type waitWriter struct{}

func (e *waitWriter) Write([]byte) (int, error) {
	panic("write on waitWriter should not happen")
}
