package twitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDemux_Handle(t *testing.T) {
	messages, expectedCounts := exampleMessages()
	counts := &counter{}
	demux := newCounterDemux(counts)
	for _, message := range messages {
		demux.Handle(message)
	}
	assert.Equal(t, expectedCounts, counts)
}

func TestDemux_HandleChan(t *testing.T) {
	messages, expectedCounts := exampleMessages()
	counts := &counter{}
	demux := newCounterDemux(counts)
	ch := make(chan interface{})
	// stream messages into channel
	go func() {
		for _, msg := range messages {
			ch <- msg
		}
		close(ch)
	}()
	// handle channel messages until exhausted
	demux.HandleChan(ch)
	assert.Equal(t, expectedCounts, counts)
}

// counter counts stream messages by type for testing.
type counter struct {
	all              int
	tweet            int
	dm               int
	statusDeletion   int
	locationDeletion int
	streamLimit      int
	statusWithheld   int
	userWithheld     int
	streamDisconnect int
	stallWarning     int
	friendsList      int
	event            int
	other            int
}

// newCounterDemux returns a Demux which counts message types.
func newCounterDemux(counter *counter) Demux {
	demux := NewSwitchDemux()
	demux.All = func(interface{}) {
		counter.all++
	}
	demux.Tweet = func(*Tweet) {
		counter.tweet++
	}
	demux.DM = func(*DirectMessage) {
		counter.dm++
	}
	demux.StatusDeletion = func(*StatusDeletion) {
		counter.statusDeletion++
	}
	demux.LocationDeletion = func(*LocationDeletion) {
		counter.locationDeletion++
	}
	demux.StreamLimit = func(*StreamLimit) {
		counter.streamLimit++
	}
	demux.StatusWithheld = func(*StatusWithheld) {
		counter.statusWithheld++
	}
	demux.UserWithheld = func(*UserWithheld) {
		counter.userWithheld++
	}
	demux.StreamDisconnect = func(*StreamDisconnect) {
		counter.streamDisconnect++
	}
	demux.Warning = func(*StallWarning) {
		counter.stallWarning++
	}
	demux.FriendsList = func(*FriendsList) {
		counter.friendsList++
	}
	demux.Event = func(*Event) {
		counter.event++
	}
	demux.Other = func(interface{}) {
		counter.other++
	}
	return demux
}

// examples messages returns a test stream of messages and the expected
// counts of each message type.
func exampleMessages() (messages []interface{}, expectedCounts *counter) {
	var (
		tweet            = &Tweet{}
		dm               = &DirectMessage{}
		statusDeletion   = &StatusDeletion{}
		locationDeletion = &LocationDeletion{}
		streamLimit      = &StreamLimit{}
		statusWithheld   = &StatusWithheld{}
		userWithheld     = &UserWithheld{}
		streamDisconnect = &StreamDisconnect{}
		stallWarning     = &StallWarning{}
		friendsList      = &FriendsList{}
		event            = &Event{}
		otherA           = func() {}
		otherB           = struct{}{}
	)
	messages = []interface{}{tweet, dm, statusDeletion, locationDeletion,
		streamLimit, statusWithheld, userWithheld, streamDisconnect,
		stallWarning, friendsList, event, otherA, otherB}
	expectedCounts = &counter{
		all:              len(messages),
		tweet:            1,
		dm:               1,
		statusDeletion:   1,
		locationDeletion: 1,
		streamLimit:      1,
		statusWithheld:   1,
		userWithheld:     1,
		streamDisconnect: 1,
		stallWarning:     1,
		friendsList:      1,
		event:            1,
		other:            2,
	}
	return messages, expectedCounts
}
