// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

type testSliceWriter struct {
	b [][]byte
}

func (tsw *testSliceWriter) Write(p []byte) (n int, err error) {
	l := make([]byte, len(p))
	copy(l, p)
	tsw.b = append(tsw.b, l)
	return len(p), nil
}

func TestLineWriter(t *testing.T) {
	tsw := &testSliceWriter{}
	lw := NewLineWriter(tsw)

	lineCount := 7
	lw.Write([]byte("0 line\n1 line\n2 line\n\n4 line"))
	lw.Write([]byte("+more\n5 line\n"))
	lw.Write([]byte("6 line"))

	lw.Flush()

	if len(tsw.b) != lineCount {
		t.Errorf("Expected %v individual rows; got %v", lineCount, len(tsw.b))
	}

	for x := 0; x < len(tsw.b); x++ {
		l := fmt.Sprintf("%v line\n", x)
		if x == 3 {
			if len(tsw.b[x]) != 1 {
				t.Errorf("Expected slice with only newline; got %v", tsw.b[x])
			}
			continue
		} else if x == 4 {
			l = "4 line+more\n"
		}
		if !bytes.Equal(tsw.b[x], []byte(l)) {
			t.Errorf("Expected slice %s equal to %s", []byte(l), tsw.b[x])
		}
	}
}

func TestHeadWriter(t *testing.T) {
	data := []byte("the quick\n brown\n fox jumped\n over the\n lazy dog.")
	w := NewHeadLinesWriter(3)
	_, err := w.Write(data[:4])
	if err != nil {
		t.Errorf("Expected nil error on small write")
	}

	if !bytes.Equal(w.Head(), []byte("the ")) {
		t.Errorf("Expected 4 bytes in head, got '%s'", w.Head())
	}

	n, err := w.Write(data[4:16])
	if n != len(data[4:16]) || err != nil {
		t.Errorf("HeadWriter Write() does not satisfy contract about failing writes.")
	}

	if !bytes.Equal(w.Head(), []byte("the quick\n brown")) {
		t.Errorf("unexpected contents of head, got '%s'", w.Head())
	}

	n, err = w.Write(data[16:])
	if n != (29-16) || err != io.ErrShortWrite {
		t.Errorf("HeadWriter Write() does not satisfy contract about failing writes.")
	}
	if !bytes.Equal(w.Head(), data[:29]) {
		t.Errorf("unexpected contents of head, got '%s'", w.Head())
	}
}

func testTail(t *testing.T, n int, output []byte, writes ...[]byte) {
	w := NewTailLinesWriter(n)
	for _, slice := range writes {
		written, err := w.Write(slice)
		if written != len(slice) || err != nil {
			t.Errorf("Tail Write() should always succeed, but failed, input=%s, input length = %d, written=%d, err=%s", slice, len(slice), written, err)
		}
	}
	if !bytes.Equal(w.Tail(), output) {
		t.Errorf("Output did not match for tail writer of length %d: Expected '%s', got '%s'", n, output, w.Tail())
	}
}

func TestTailWriter(t *testing.T) {
	inputs := [][]byte{[]byte("a\nb\n"), []byte("gh"), []byte("\n")}
	testTail(t, 2, []byte("b\ngh\n"), inputs...)
}

func TestZeroAndOneTailWriter(t *testing.T) {
	// zero line writer, with only single line added to it should return empty buffer.
	testTail(t, 0, []byte(""), []byte("Hello World\n"))
	testTail(t, 0, []byte(""), []byte("Hello World"))

	b1 := []byte("Hello World")
	testTail(t, 1, b1, b1)

	b1 = []byte("Hello World\n")
	testTail(t, 1, b1, b1)

	b2 := []byte("Yeah!\n")
	testTail(t, 1, b2, b1, b2)

	b1 = []byte("Flat write")
	b2 = []byte("Yeah!\n")
	j := bytes.Join([][]byte{b1, b2}, []byte{})
	testTail(t, 1, j, b1, b2)
}

func TestTailWriterTrailing(t *testing.T) {
	input1 := []byte("a\nb\nc\nd\ne")
	input2 := []byte("a\nb\nc\nd\ne\n")
	w1 := NewTailLinesWriter(4)
	w1.Write(input1)
	w2 := NewTailLinesWriter(4)
	w2.Write(input2)
	if !bytes.Equal(w1.Tail(), []byte("b\nc\nd\ne")) {
		t.Errorf("Tail not working correctly, got '%s'", w1.Tail())
	}

	t2 := w2.Tail()
	if !bytes.Equal(w1.Tail(), t2[:len(t2)-1]) {
		t.Errorf("Tailwriter does not transition correctly over trailing newline. '%s', '%s'", w1.Tail(), t2)
	}
}
