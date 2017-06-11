// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBufferedClientFlushSize(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	for _, tt := range statsdPacketTests {
		// set flush length to the size of the expected output packet
		// so we can ensure a flush happens right away.
		// set flush time sufficiently high so that it never matters for this
		// test
		c, err := NewBufferedClient(l.LocalAddr().String(), tt.Prefix, 10*time.Second, len(tt.Expected)+1)
		if err != nil {
			c.Close()
			t.Fatal(err)
		}
		method := reflect.ValueOf(c).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Stat),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			c.Close()
			t.Fatal(errInter.(error))
		}

		data := make([]byte, len(tt.Expected)+16)
		_, _, err = l.ReadFrom(data)
		if err != nil {
			c.Close()
			t.Fatal(err)
		}

		data = bytes.TrimRight(data, "\x00\n")
		if bytes.Equal(data, []byte(tt.Expected)) != true {
			t.Fatalf("%s got '%s' expected '%s'", tt.Method, data, tt.Expected)
		}
		c.Close()
	}
}

func TestBufferedClientFlushTime(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	for _, tt := range statsdPacketTests {
		// set flush length to the size of the expected output packet
		// so we can ensure a flush happens right away.
		// set flush time sufficiently high so that it never matters for this
		// test
		c, err := NewBufferedClient(l.LocalAddr().String(), tt.Prefix, 1*time.Microsecond, 1024)
		if err != nil {
			c.Close()
			t.Fatal(err)
		}
		method := reflect.ValueOf(c).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Stat),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			c.Close()
			t.Fatal(errInter.(error))
		}

		time.Sleep(1 * time.Millisecond)

		data := make([]byte, len(tt.Expected)+16)
		_, _, err = l.ReadFrom(data)
		if err != nil {
			c.Close()
			t.Fatal(err)
		}

		data = bytes.TrimRight(data, "\x00\n")
		if bytes.Equal(data, []byte(tt.Expected)) != true {
			t.Fatalf("%s got '%s' expected '%s'", tt.Method, data, tt.Expected)
		}
		c.Close()
	}
}

func TestBufferedClientBigPacket(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	c, err := NewBufferedClient(l.LocalAddr().String(), "test", 10*time.Millisecond, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for _, tt := range statsdPacketTests {
		if tt.Prefix != "test" {
			continue
		}
		method := reflect.ValueOf(c).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Stat),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			t.Fatal(errInter.(error))
		}
	}

	expected := ""
	for _, tt := range statsdPacketTests {
		if tt.Prefix != "test" {
			continue
		}
		expected = expected + tt.Expected + "\n"
	}

	expected = strings.TrimSuffix(expected, "\n")

	time.Sleep(12 * time.Millisecond)
	data := make([]byte, 1024)
	_, _, err = l.ReadFrom(data)
	if err != nil {
		t.Fatal(err)
	}

	data = bytes.TrimRight(data, "\x00")
	if bytes.Equal(data, []byte(expected)) != true {
		t.Fatalf("got '%s' expected '%s'", data, expected)
	}
}

func TestFlushOnClose(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	c, err := NewBufferedClient(l.LocalAddr().String(), "test", 1*time.Second, 1024)
	if err != nil {
		t.Fatal(err)
	}

	c.Inc("count", int64(1), 1.0)
	c.Close()

	expected := "test.count:1|c"

	data := make([]byte, 1024)
	_, _, err = l.ReadFrom(data)
	if err != nil {
		t.Fatal(err)
	}

	data = bytes.TrimRight(data, "\x00")
	if bytes.Equal(data, []byte(expected)) != true {
		fmt.Println(data)
		fmt.Println([]byte(expected))
		t.Fatalf("got '%s' expected '%s'", data, expected)
	}
}

func ExampleClient_buffered() {
	// first create a client
	client, err := NewBufferedClient("127.0.0.1:8125", "test-client", 10*time.Millisecond, 0)
	// handle any errors
	if err != nil {
		log.Fatal(err)
	}
	// make sure to clean up
	defer client.Close()

	// Send a stat
	err = client.Inc("stat1", 42, 1.0)
	// handle any errors
	if err != nil {
		log.Printf("Error sending metric: %+v", err)
	}
}
