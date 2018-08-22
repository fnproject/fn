package commands_test

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/commands"
	"github.com/leanovate/gopter/gen"
)

type counter struct {
	value int
}

func (c *counter) Get() int {
	return c.value
}

func (c *counter) Inc() int {
	c.value++
	return c.value
}

func (c *counter) Dec() int {
	c.value--
	return c.value
}

var GetCommand = &commands.ProtoCommand{
	Name: "GET",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		return systemUnderTest.(*counter).Get()
	},
	PreConditionFunc: func(state commands.State) bool {
		_, ok := state.(int)
		return ok
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if state.(int) != result.(int) {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
}

var IncCommand = &commands.ProtoCommand{
	Name: "INC",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		return systemUnderTest.(*counter).Inc()
	},
	NextStateFunc: func(state commands.State) commands.State {
		return state.(int) + 1
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if state.(int) != result.(int) {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
}

var DecCommand = &commands.ProtoCommand{
	Name: "DEC",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		return systemUnderTest.(*counter).Dec()
	},
	PreConditionFunc: func(state commands.State) bool {
		return state.(int) > 0
	},
	NextStateFunc: func(state commands.State) commands.State {
		return state.(int) - 1
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if state.(int) != result.(int) {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
}

type counterCommands struct {
}

func (c *counterCommands) NewSystemUnderTest(initialState commands.State) commands.SystemUnderTest {
	return &counter{value: initialState.(int)}
}

func (c *counterCommands) DestroySystemUnderTest(commands.SystemUnderTest) {
}

func (c *counterCommands) GenInitialState() gopter.Gen {
	return gen.Int()
}

func (c *counterCommands) InitialPreCondition(state commands.State) bool {
	return state.(int) >= 0
}

func (c *counterCommands) GenCommand(state commands.State) gopter.Gen {
	return gen.OneConstOf(GetCommand, IncCommand, DecCommand)
}

func TestCommands(t *testing.T) {
	parameters := gopter.DefaultTestParameters()

	prop := commands.Prop(&counterCommands{})

	result := prop.Check(parameters)
	if !result.Passed() {
		t.Errorf("Invalid result: %v", result)
	}
}
