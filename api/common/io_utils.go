package common

import (
	"io"
)

type clampWriter struct {
	w           io.Writer
	remaining   int64
	overflowErr error
}

func NewClampWriter(buf io.Writer, maxResponseSize uint64, overflowErr error) io.Writer {
	if maxResponseSize != 0 {
		return &clampWriter{w: buf, remaining: int64(maxResponseSize), overflowErr: overflowErr}
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
