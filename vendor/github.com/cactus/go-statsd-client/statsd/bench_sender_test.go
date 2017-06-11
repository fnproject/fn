// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import (
	"bytes"
	"testing"
	"time"
)

func BenchmarkSenderSmall(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	s, err := NewSimpleSender(l.LocalAddr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	data := []byte("test.gauge:1|g\n")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			s.Send(data)
		}
	})
}

func BenchmarkSenderLarge(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	s, err := NewSimpleSender(l.LocalAddr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	data := bytes.Repeat([]byte("test.gauge:1|g\n"), 50)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			s.Send(data)
		}
	})
}

func BenchmarkBufferedSenderSmall(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	s, err := NewBufferedSender(l.LocalAddr().String(), 300*time.Millisecond, 1432)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	data := []byte("test.gauge:1|g\n")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			s.Send(data)
		}
	})
}
func BenchmarkBufferedSenderLarge(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	s, err := NewBufferedSender(l.LocalAddr().String(), 300*time.Millisecond, 1432)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	data := bytes.Repeat([]byte("test.gauge:1|g\n"), 50)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			s.Send(data)
		}
	})
}
