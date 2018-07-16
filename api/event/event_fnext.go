package event

import (
	"fmt"
	"github.com/fnproject/fn/api/common"
)

const (
	ExtIoFnProjectDeadline = "ioFnProjectDeadline"
)

func (ce *Event) SetDeadline(dateTime common.DateTime) {
	err := ce.SetExtension(ExtIoFnProjectDeadline, dateTime)
	if err != nil {
		panic(fmt.Sprintf("unexpeted error setting deadline extension %s", err))
	}
}
