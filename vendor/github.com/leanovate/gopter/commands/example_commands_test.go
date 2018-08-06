package commands_test

import (
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/commands"
	"github.com/leanovate/gopter/gen"
)

type BuggyCounter struct {
	n int
}

func (c *BuggyCounter) Inc() {
	c.n++
}

func (c *BuggyCounter) Dec() {
	if c.n > 3 {
		// Intentional error
		c.n -= 2
	} else {
		c.n--
	}
}

func (c *BuggyCounter) Get() int {
	return c.n
}

func (c *BuggyCounter) Reset() {
	c.n = 0
}

var GetBuggyCommand = &commands.ProtoCommand{
	Name: "GET",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		return systemUnderTest.(*BuggyCounter).Get()
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if state.(int) != result.(int) {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
}

var IncBuggyCommand = &commands.ProtoCommand{
	Name: "INC",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		systemUnderTest.(*BuggyCounter).Inc()
		return nil
	},
	NextStateFunc: func(state commands.State) commands.State {
		return state.(int) + 1
	},
}

var DecBuggyCommand = &commands.ProtoCommand{
	Name: "DEC",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		systemUnderTest.(*BuggyCounter).Dec()
		return nil
	},
	NextStateFunc: func(state commands.State) commands.State {
		return state.(int) - 1
	},
}

var ResetBuggyCommand = &commands.ProtoCommand{
	Name: "RESET",
	RunFunc: func(systemUnderTest commands.SystemUnderTest) commands.Result {
		systemUnderTest.(*BuggyCounter).Reset()
		return nil
	},
	NextStateFunc: func(state commands.State) commands.State {
		return 0
	},
}

var buggyCounterCommands = &commands.ProtoCommands{
	NewSystemUnderTestFunc: func(initialState commands.State) commands.SystemUnderTest {
		return &BuggyCounter{}
	},
	InitialStateGen: gen.Const(0),
	InitialPreConditionFunc: func(state commands.State) bool {
		return state.(int) == 0
	},
	GenCommandFunc: func(state commands.State) gopter.Gen {
		return gen.OneConstOf(GetBuggyCommand, IncBuggyCommand, DecBuggyCommand, ResetBuggyCommand)
	},
}

// Demonstrates the usage of the commands package to find a bug in a counter
// implementation that only occures if the counter is above 3.
//
// The output of this example will be
//  ! buggy counter: Falsified after 45 passed tests.
//  ARG_0: initial=0 sequential=[INC INC INC INC DEC GET]
//  ARG_0_ORIGINAL (9 shrinks): initial=0 sequential=[DEC RESET GET GET GET
//     RESET DEC DEC INC INC RESET RESET DEC INC RESET INC INC GET INC INC DEC
//     DEC GET RESET INC INC DEC INC INC INC RESET RESET INC INC GET INC DEC GET
//     DEC GET INC RESET INC INC RESET]
// I.e. gopter found an invalid state with a rather long sequence of arbitrary
// commonds/function calls, and then shrinkted that sequence down to
//  INC INC INC INC DEC GET
// which is indeed the minimal set of commands one has to perform to find the
// bug.
func Example_buggyCounter() {
	parameters := gopter.DefaultTestParameters()
	parameters.Rng.Seed(1234) // Just for this example to generate reproducable results

	properties := gopter.NewProperties(parameters)

	properties.Property("buggy counter", commands.Prop(buggyCounterCommands))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! buggy counter: Falsified after 43 passed tests.
	// ARG_0: initialState=0 sequential=[INC INC INC INC DEC GET]
	// ARG_0_ORIGINAL (8 shrinks): initialState=0 sequential=[RESET GET GET GET
	//    RESET DEC DEC INC INC RESET RESET DEC INC RESET INC INC GET INC INC DEC
	//    DEC GET RESET INC INC DEC INC INC INC RESET RESET INC INC GET INC DEC GET
	//    DEC GET INC RESET INC INC]
}
