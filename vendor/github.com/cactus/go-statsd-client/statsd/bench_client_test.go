// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import (
	"testing"
)

func BenchmarkClientInc(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	c, err := NewClient(l.LocalAddr().String(), "test")
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			c.Inc("benchinc", 1, 1)
		}
	})
}

func BenchmarkClientIncSample(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	c, err := NewClient(l.LocalAddr().String(), "test")
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			c.Inc("benchinc", 1, 0.3)
		}
	})
}

func BenchmarkClientSetInt(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	c, err := NewClient(l.LocalAddr().String(), "test")
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			c.SetInt("setint", 1, 1)
		}
	})
}

func BenchmarkClientSetIntSample(b *testing.B) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	c, err := NewClient(l.LocalAddr().String(), "test")
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//i := 0; i < b.N; i++ {
			c.SetInt("setint", 1, 0.3)
		}
	})
}
