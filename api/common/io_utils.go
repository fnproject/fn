package common

import (
	"io"
	"net/http"
	"sync"
)

// StripHopHeaders removes transport related headers.
func StripHopHeaders(hdr http.Header) {
	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, h := range hopHeaders {
		hv := hdr.Get(h)
		if hv == "" {
			continue
		}
		if h == "Te" && hv == "trailers" {
			// Issue 21096: tell backend applications that
			// care about trailer support that we support
			// trailers. (We do, but we don't go out of
			// our way to advertise that unless the
			// incoming client request thought it was
			// worth mentioning)
			continue
		}
		hdr.Del(h)
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
// stolen: net/http/httputil/reverseproxy.go
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// NoopReadWriteCloser implements io.ReadWriteCloser, discarding all bytes, Read always returns EOF
type NoopReadWriteCloser struct{}

var _ io.ReadWriteCloser = NoopReadWriteCloser{}

// Read implements io.Reader
func (n NoopReadWriteCloser) Read(b []byte) (int, error) { return 0, io.EOF }

// Write implements io.Writer
func (n NoopReadWriteCloser) Write(b []byte) (int, error) { return len(b), nil }

// Close implements io.Closer
func (n NoopReadWriteCloser) Close() error { return nil }

type clampWriter struct {
	w           io.Writer
	remaining   int64
	overflowErr error
}

// NewClamWriter creates a clamp writer that will limit the number of bytes written to to an underlying stream
// This allows up to max bytes to be written to the underlying stream , writes that exceed this will return overflowErr
//
// # Setting a  max of 0 sets no limit
//
// If a write spans the last remaining bytes available the number of bytes up-to the limit will be written and the
// overflow error will be returned
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
	overflowing := false
	if int64(len(p)) > g.remaining {
		p = p[0:g.remaining]
		overflowing = true
	}

	n, err := g.w.Write(p)

	g.remaining -= int64(n)
	if n == len(p) && overflowing {
		err = g.overflowErr
	}
	return n, err
}

type clampReadCloser struct {
	r           io.ReadCloser
	remaining   int64
	overflowErr error
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
		if _, ok := g.inner.(*waitWriter); ok || g.inner == nil {
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
		if _, ok := g.inner.(*waitReader); ok || g.inner == nil {
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
