package twitter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStopped(t *testing.T) {
	done := make(chan struct{})
	assert.False(t, stopped(done))
	close(done)
	assert.True(t, stopped(done))
}

func TestSleepOrDone_Sleep(t *testing.T) {
	wait := time.Nanosecond * 20
	done := make(chan struct{})
	completed := make(chan struct{})
	go func() {
		sleepOrDone(wait, done)
		close(completed)
	}()
	// wait for goroutine SleepOrDone to sleep
	assertDone(t, completed, defaultTestTimeout)
}

func TestSleepOrDone_Done(t *testing.T) {
	wait := time.Second * 5
	done := make(chan struct{})
	completed := make(chan struct{})
	go func() {
		sleepOrDone(wait, done)
		close(completed)
	}()
	// close done, interrupting SleepOrDone
	close(done)
	// assert that SleepOrDone exited, closing completed
	assertDone(t, completed, defaultTestTimeout)
}

func TestScanLines(t *testing.T) {
	cases := []struct {
		input   []byte
		atEOF   bool
		advance int
		token   []byte
	}{
		{[]byte("Line 1\r\n"), false, 8, []byte("Line 1")},
		{[]byte("Line 1\n"), false, 0, nil},
		{[]byte("Line 1"), false, 0, nil},
		{[]byte(""), false, 0, nil},
		{[]byte("Line 1\r\n"), true, 8, []byte("Line 1")},
		{[]byte("Line 1\n"), true, 7, []byte("Line 1")},
		{[]byte("Line 1"), true, 6, []byte("Line 1")},
		{[]byte(""), true, 0, nil},
	}
	for _, c := range cases {
		advance, token, _ := scanLines(c.input, c.atEOF)
		assert.Equal(t, c.advance, advance)
		assert.Equal(t, c.token, token)
	}
}
