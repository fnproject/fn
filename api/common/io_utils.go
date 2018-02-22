package common

import (
	"io"
	"io/ioutil"
)

type ClampWriter struct {
	W          io.Writer
	remaining  int64
	IsOverflow bool
}

func NewClampWriter(buf io.Writer, maxResponseSize uint64) io.Writer {
	if maxResponseSize != 0 {
		return &ClampWriter{W: buf, remaining: int64(maxResponseSize)}
	}
	return buf
}

func (g *ClampWriter) Write(p []byte) (int, error) {
	if g.remaining <= 0 {
		g.IsOverflow = true
		return 0, io.EOF
	}
	if int64(len(p)) > g.remaining {
		g.IsOverflow = true
		p = p[0:g.remaining]
	}

	n, err := g.W.Write(p)
	g.remaining -= int64(n)
	return n, err
}

type ClampReader struct {
	reader     io.Reader // underlying reader
	remaining  int64     // max bytes remaining
	IsOverflow bool
	isLinger   bool
}

func NewClampReader(buf io.Reader, maxResponseSize uint64, isLinger bool) io.Reader {
	if maxResponseSize != 0 {
		return &ClampReader{reader: buf, remaining: int64(maxResponseSize), isLinger: isLinger}
	}
	return buf
}

func (l *ClampReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		l.IsOverflow = true
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[0:l.remaining]
	}
	n, err := l.reader.Read(p)
	l.remaining -= int64(n)

	if l.remaining <= 0 && l.isLinger {
		go func() {
			io.Copy(ioutil.Discard, l.reader)
		}()
	}
	return n, err
}
