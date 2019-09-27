package agent

import (
	"fmt"
	"github.com/fnproject/fn/fnext"
	"github.com/stretchr/testify/mock"
)

type MockAgent struct {
	mock.Mock
}

func (m *MockAgent) GetCall(opts ...CallOpt) (Call, error) {

	var newCall *call
	var err error

	args := m.Called(opts)

	switch len(args) {
	case 0:
		newCall = &call{}
	case 1:
		c, ok := args.Get(0).(call)
		if ok {
			newCall = &c
		} else {
			newCall = &call{}
			err = args.Error(0)
		}
	case 2:
		var c call
		if c, ok := args.Get(0).(call); !ok {
			panic(fmt.Sprintf("assert: arguments: wrong argument type %v setup for GetCall", c))
		}
		newCall = &c
		err = args.Error(1)
	default:
		panic(fmt.Sprintf("unsupported number of args to GetCall mock %d", len(args)))
	}
	for _, opt := range opts {
		if oerr := opt(newCall); oerr != nil {
			return nil, oerr
		}
	}
	return newCall, err
}

func (m *MockAgent) Submit(c Call) error {
	return m.Called(c).Error(0)
}

func (m *MockAgent) Close() error {
	return m.Called().Error(0)
}

func (m *MockAgent) AddCallListener(f fnext.CallListener) {
	m.Called(f)
}
