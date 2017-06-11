// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import (
	"bytes"
	"testing"
	"time"
)

type mockSender struct {
	closeCallCount int
}

func (m *mockSender) Send(data []byte) (int, error) {
	return 0, nil
}

func (m *mockSender) Close() error {
	m.closeCallCount++
	return nil
}

func TestClose(t *testing.T) {
	mockSender := &mockSender{}
	sender := &BufferedSender{
		flushBytes:    512,
		flushInterval: 1 * time.Second,
		sender:        mockSender,
		buffer:        bytes.NewBuffer(make([]byte, 0, 512)),
		shutdown:      make(chan chan error),
	}

	sender.Close()
	if mockSender.closeCallCount != 0 {
		t.Fatalf("expected close to have been called zero times, but got %d", mockSender.closeCallCount)
	}

	sender.Start()
	if !sender.running {
		t.Fatal("sender failed to start")
	}

	sender.Close()
	if mockSender.closeCallCount != 1 {
		t.Fatalf("expected close to have been called once, but got %d", mockSender.closeCallCount)
	}
}

func TestCloseConcurrent(t *testing.T) {
	mockSender := &mockSender{}
	sender := &BufferedSender{
		flushBytes:    512,
		flushInterval: 1 * time.Second,
		sender:        mockSender,
		buffer:        bytes.NewBuffer(make([]byte, 0, 512)),
		shutdown:      make(chan chan error),
	}
	sender.Start()

	const N = 10
	c := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			sender.Close()
			c <- struct{}{}
		}()
	}

	for i := 0; i < N; i++ {
		<-c
	}

	if mockSender.closeCallCount != 1 {
		t.Errorf("expected close to have been called once, but got %d", mockSender.closeCallCount)
	}
}

func TestCloseDuringSendConcurrent(t *testing.T) {
	mockSender := &mockSender{}
	sender := &BufferedSender{
		flushBytes:    512,
		flushInterval: 1 * time.Second,
		sender:        mockSender,
		buffer:        bytes.NewBuffer(make([]byte, 0, 512)),
		shutdown:      make(chan chan error),
	}
	sender.Start()

	const N = 10
	c := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			for {
				_, err := sender.Send([]byte("stat:1|c"))
				if err != nil {
					c <- struct{}{}
					return
				}
			}
		}()
	}

	// senders should error out now
	// we should not receive any panics
	sender.Close()
	for i := 0; i < N; i++ {
		<-c
	}

	if mockSender.closeCallCount != 1 {
		t.Errorf("expected close to have been called once, but got %d", mockSender.closeCallCount)
	}
}
