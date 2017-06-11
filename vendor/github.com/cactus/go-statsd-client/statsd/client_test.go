// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import (
	"bytes"
	"log"
	"net"
	"reflect"
	"testing"
	"time"
)

var statsdPacketTests = []struct {
	Prefix   string
	Method   string
	Stat     string
	Value    interface{}
	Rate     float32
	Expected string
}{
	{"test", "Gauge", "gauge", int64(1), 1.0, "test.gauge:1|g"},
	{"test", "Inc", "count", int64(1), 0.999999, "test.count:1|c|@0.999999"},
	{"test", "Inc", "count", int64(1), 1.0, "test.count:1|c"},
	{"test", "Dec", "count", int64(1), 1.0, "test.count:-1|c"},
	{"test", "Timing", "timing", int64(1), 1.0, "test.timing:1|ms"},
	{"test", "TimingDuration", "timing", 1500 * time.Microsecond, 1.0, "test.timing:1.5|ms"},
	{"test", "TimingDuration", "timing", 3 * time.Microsecond, 1.0, "test.timing:0.003|ms"},
	{"test", "Set", "strset", "pickle", 1.0, "test.strset:pickle|s"},
	{"test", "SetInt", "intset", int64(1), 1.0, "test.intset:1|s"},
	{"test", "GaugeDelta", "gauge", int64(1), 1.0, "test.gauge:+1|g"},
	{"test", "GaugeDelta", "gauge", int64(-1), 1.0, "test.gauge:-1|g"},

	{"", "Gauge", "gauge", int64(1), 1.0, "gauge:1|g"},
	{"", "Inc", "count", int64(1), 0.999999, "count:1|c|@0.999999"},
	{"", "Inc", "count", int64(1), 1.0, "count:1|c"},
	{"", "Dec", "count", int64(1), 1.0, "count:-1|c"},
	{"", "Timing", "timing", int64(1), 1.0, "timing:1|ms"},
	{"", "TimingDuration", "timing", 1500 * time.Microsecond, 1.0, "timing:1.5|ms"},
	{"", "Set", "strset", "pickle", 1.0, "strset:pickle|s"},
	{"", "SetInt", "intset", int64(1), 1.0, "intset:1|s"},
	{"", "GaugeDelta", "gauge", int64(1), 1.0, "gauge:+1|g"},
	{"", "GaugeDelta", "gauge", int64(-1), 1.0, "gauge:-1|g"},
}

func TestClient(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, tt := range statsdPacketTests {
		c, err := NewClient(l.LocalAddr().String(), tt.Prefix)
		if err != nil {
			t.Fatal(err)
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

		data := make([]byte, 128)
		_, _, err = l.ReadFrom(data)
		if err != nil {
			c.Close()
			t.Fatal(err)
		}

		data = bytes.TrimRight(data, "\x00")
		if bytes.Equal(data, []byte(tt.Expected)) != true {
			c.Close()
			t.Fatalf("%s got '%s' expected '%s'", tt.Method, data, tt.Expected)
		}
		c.Close()
	}
}

func TestNilClient(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, tt := range statsdPacketTests {
		var c *Client

		method := reflect.ValueOf(c).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Stat),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			t.Fatal(errInter.(error))
		}

		data := make([]byte, 128)
		n, _, err := l.ReadFrom(data)
		// this is expected to error, since there should
		// be no udp data sent, so the read will time out
		if err == nil || n != 0 {
			c.Close()
			t.Fatal(err)
		}
		c.Close()
	}
}

func TestNoopClient(t *testing.T) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, tt := range statsdPacketTests {
		c, err := NewNoopClient(l.LocalAddr().String(), tt.Prefix)
		if err != nil {
			t.Fatal(err)
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

		data := make([]byte, 128)
		n, _, err := l.ReadFrom(data)
		// this is expected to error, since there should
		// be no udp data sent, so the read will time out
		if err == nil || n != 0 {
			c.Close()
			t.Fatal(err)
		}
		c.Close()
	}
}

func newUDPListener(addr string) (*net.UDPConn, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	l.SetDeadline(time.Now().Add(100 * time.Millisecond))
	l.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	l.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	return l.(*net.UDPConn), nil
}

func ExampleClient() {
	// first create a client
	client, err := NewClient("127.0.0.1:8125", "test-client")
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

func ExampleClient_noop() {
	// use interface so we can sub noop client if needed
	var client Statter
	var err error

	// first try to create a real client
	client, err = NewClient("not-resolvable:8125", "test-client")
	// Let us say real client creation fails, but you don't care enough about
	// stats that you don't want your program to run. Just log an error and
	// make a NoopClient instead
	if err != nil {
		log.Println("Remote endpoint did not resolve. Disabling stats", err)
		client, err = NewNoopClient()
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
