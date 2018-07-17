package event

import (
	"fmt"
	"github.com/fnproject/fn/api/common"
)

const (
	ExtIoFnProjectDeadline = "ioFnProjectDeadline"
	ExtIoFnProjectCallID   = "ioFnProjectCallID"
)

// SetDeadline is a helper that sets the fn deadline extension on the event
func (ce *Event) SetDeadline(dateTime common.DateTime) {
	err := ce.SetExtension(ExtIoFnProjectDeadline, dateTime)
	if err != nil {
		panic(fmt.Sprintf("unexpected error setting deadline extension %s", err))
	}
}

// GetDeadline is a helper that reads the fn deadline extension from the event
func (ce *Event) GetDeadline() (common.DateTime, error) {
	var dt common.DateTime
	err := ce.ReadExtension(ExtIoFnProjectDeadline, &dt)
	if err != nil {
		return common.NewDateTime(), err

	}
	return dt, nil
}

// SetCallID is a helper that sets the fn call ID  extension on the event
func (ce *Event) SetCallID(callID string) {
	err := ce.SetExtension(ExtIoFnProjectCallID, callID)
	if err != nil {
		panic(fmt.Sprintf("unexpeted error setting callID extension %s", err))
	}
}

// GetCallID is a helper that reads the fn call ID from the event
func (ce *Event) GetCallID() (string, error) {
	var callID string
	err := ce.ReadExtension(ExtIoFnProjectCallID, &callID)
	if err != nil {
		return "", err

	}
	return callID, nil
}
