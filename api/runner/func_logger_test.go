package runner

import (
	"bytes"
	"io"
	"testing"
)

type nopCloser struct {
	io.Writer
}

func (n nopCloser) Close() error { return nil }

func TestLimitWriter(t *testing.T) {
	var b bytes.Buffer
	const max = 5
	lw := newLimitWriter(max, nopCloser{&b})

	lw.Write([]byte("yo"))

	if b.Len() != 2 {
		t.Fatal("expected 2 bytes in buffer, got:", b.Len())
	}

	n, _ := lw.Write([]byte("dawg"))

	// can't check b.Len() really since the overage message is written in
	if n != 3 {
		t.Fatalf("limit writer allowed writing over the limit or n was wrong. n: %d", n)
	}

	n, err := lw.Write([]byte("yodawg"))

	if n != 0 || err == nil {
		t.Fatalf("limit writer wrote after limit exceeded, n > 0 or err is nil. n: %d err: %v", n, err)
	}

	// yes should const this. yes i'm wrong. yes you're wrong. no it doesn't matter.
	if !bytes.HasPrefix(b.Bytes(), []byte("yodaw\n-----max")) {
		t.Fatal("expected buffer to be 'yodawg', got:", b.String())
	}
}

func TestLineWriter(t *testing.T) {
	var b bytes.Buffer
	lw := newLineWriter(&b)

	lw.Write([]byte("yo"))

	if b.Len() != 0 {
		t.Fatal("expected no bytes to be written, got bytes")
	}

	lw.Write([]byte("\ndawg"))

	if b.Len() != 3 {
		t.Fatal("expected 3 bytes to be written in, got:", b.Len())
	}

	lw.Write([]byte("\ndawgy\ndawg"))

	if b.Len() != 14 {
		t.Fatal("expected 14 bytes to be written in, got:", b.Len())
	}

	lw.Close()

	if b.Len() != 19 {
		t.Fatal("expected 19 bytes to be written in, got:", b.Len())
	}

	if !bytes.HasSuffix(b.Bytes(), []byte("\n")) {
		t.Fatal("line writer close is broked, expected new line")
	}
}
