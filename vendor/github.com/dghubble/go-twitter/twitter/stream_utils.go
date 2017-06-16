package twitter

import (
	"strings"
	"time"
)

// stopped returns true if the done channel receives, false otherwise.
func stopped(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// sleepOrDone pauses the current goroutine until the done channel receives
// or until at least the duration d has elapsed, whichever comes first. This
// is similar to time.Sleep(d), except it can be interrupted.
func sleepOrDone(d time.Duration, done <-chan struct{}) {
	select {
	case <-time.After(d):
		return
	case <-done:
		return
	}
}

// scanLines is a split function for a Scanner that returns each line of text
// stripped of the end-of-line marker "\r\n" used by Twitter Streaming APIs.
// This differs from the bufio.ScanLines split function which considers the
// '\r' optional.
// https://dev.twitter.com/streaming/overview/processing
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := strings.Index(string(data), "\r\n"); i >= 0 {
		// We have a full '\r\n' terminated line.
		return i + 2, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\n' {
		return data[0 : len(data)-1]
	}
	return data
}
