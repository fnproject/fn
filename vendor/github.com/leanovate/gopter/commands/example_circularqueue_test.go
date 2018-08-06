package commands_test

import (
	"fmt"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/commands"
	"github.com/leanovate/gopter/gen"
)

// *****************************************
// Production code (i.e. the implementation)
// *****************************************

type Queue struct {
	inp  int
	outp int
	size int
	buf  []int
}

func New(n int) *Queue {
	return &Queue{
		inp:  0,
		outp: 0,
		size: n + 1,
		buf:  make([]int, n+1),
	}
}

func (q *Queue) Put(n int) int {
	if q.inp == 4 && n > 0 { // Intentional spooky bug
		q.buf[q.size-1] *= n
	}
	q.buf[q.inp] = n
	q.inp = (q.inp + 1) % q.size
	return n
}

func (q *Queue) Get() int {
	ans := q.buf[q.outp]
	q.outp = (q.outp + 1) % q.size
	return ans
}

func (q *Queue) Size() int {
	return (q.inp - q.outp + q.size) % q.size
}

func (q *Queue) Init() {
	q.inp = 0
	q.outp = 0
}

// *****************************************
//               Test code
// *****************************************

// cbState holds the expected state (i.e. its the commands.State)
type cbState struct {
	size         int
	elements     []int
	takenElement int
}

func (st *cbState) TakeFront() {
	st.takenElement = st.elements[0]
	st.elements = append(st.elements[:0], st.elements[1:]...)
}

func (st *cbState) PushBack(value int) {
	st.elements = append(st.elements, value)
}

func (st *cbState) String() string {
	return fmt.Sprintf("State(size=%d, elements=%v)", st.size, st.elements)
}

// Get command simply invokes the Get function on the queue and compares the
// result with the expected state.
var genGetCommand = gen.Const(&commands.ProtoCommand{
	Name: "Get",
	RunFunc: func(q commands.SystemUnderTest) commands.Result {
		return q.(*Queue).Get()
	},
	NextStateFunc: func(state commands.State) commands.State {
		state.(*cbState).TakeFront()
		return state
	},
	// The implementation implicitly assumes that Get is never called on an
	// empty Queue, therefore the command requires a corresponding pre-condition
	PreConditionFunc: func(state commands.State) bool {
		return len(state.(*cbState).elements) > 0
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if result.(int) != state.(*cbState).takenElement {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
})

// Put command puts a value into the queue by using the Put function. Since
// the Put function has an int argument the Put command should have a
// corresponding parameter.
type putCommand int

func (value putCommand) Run(q commands.SystemUnderTest) commands.Result {
	return q.(*Queue).Put(int(value))
}

func (value putCommand) NextState(state commands.State) commands.State {
	state.(*cbState).PushBack(int(value))
	return state
}

// The implementation implicitly assumes that that Put is never called if
// the capacity is exhausted, therefore the command requires a corresponding
// pre-condition.
func (putCommand) PreCondition(state commands.State) bool {
	s := state.(*cbState)
	return len(s.elements) < s.size
}

func (putCommand) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	st := state.(*cbState)
	if result.(int) != st.elements[len(st.elements)-1] {
		return &gopter.PropResult{Status: gopter.PropFalse}
	}
	return &gopter.PropResult{Status: gopter.PropTrue}
}

func (value putCommand) String() string {
	return fmt.Sprintf("Put(%d)", value)
}

// We want to have a generator for put commands for arbitrary int values.
// In this case the command is actually shrinkable, e.g. if the property fails
// by putting a 1000, it might already fail for a 500 as well ...
var genPutCommand = gen.Int().Map(func(value int) commands.Command {
	return putCommand(value)
}).WithShrinker(func(v interface{}) gopter.Shrink {
	return gen.IntShrinker(int(v.(putCommand))).Map(func(value int) putCommand {
		return putCommand(value)
	})
})

// Size command is simple again, it just invokes the Size function and
// compares compares the result with the expected state.
// The Size function can be called any time, therefore this command does not
// require a pre-condition.
var genSizeCommand = gen.Const(&commands.ProtoCommand{
	Name: "Size",
	RunFunc: func(q commands.SystemUnderTest) commands.Result {
		return q.(*Queue).Size()
	},
	PostConditionFunc: func(state commands.State, result commands.Result) *gopter.PropResult {
		if result.(int) != len(state.(*cbState).elements) {
			return &gopter.PropResult{Status: gopter.PropFalse}
		}
		return &gopter.PropResult{Status: gopter.PropTrue}
	},
})

// cbCommands implements the command.Commands interface, i.e. is
// responsible for creating/destroying the system under test and generating
// commands and initial states (cbState)
var cbCommands = &commands.ProtoCommands{
	NewSystemUnderTestFunc: func(initialState commands.State) commands.SystemUnderTest {
		s := initialState.(*cbState)
		q := New(s.size)
		for e := range s.elements {
			q.Put(e)
		}
		return q
	},
	DestroySystemUnderTestFunc: func(sut commands.SystemUnderTest) {
		sut.(*Queue).Init()
	},
	InitialStateGen: gen.IntRange(1, 30).Map(func(size int) *cbState {
		return &cbState{
			size:     size,
			elements: make([]int, 0, size),
		}
	}),
	InitialPreConditionFunc: func(state commands.State) bool {
		s := state.(*cbState)
		return len(s.elements) >= 0 && len(s.elements) <= s.size
	},
	GenCommandFunc: func(state commands.State) gopter.Gen {
		return gen.OneGenOf(genGetCommand, genPutCommand, genSizeCommand)
	},
}

// Kudos to @jamesd for providing this real world example.
// ... of course he did not implemented the bug, that was evil me
//
// The bug only occures on the following conditions:
//  - the queue size has to be greater than 4
//  - the queue has to be filled entirely once
//  - Get operations have to be at least 5 elements behind put
//  - The Put at the end of the queue and 5 elements later have to be non-zero
//
// Lets see what gopter has to say:
//
// The output of this example will be
//  ! circular buffer: Falsified after 96 passed tests.
//  ARG_0: initialState=State(size=7, elements=[]) sequential=[Put(0) Put(0)
//     Get Put(0) Get Put(0) Put(0) Get Put(0) Get Put(0) Get Put(-1) Put(0)
//     Put(0) Put(0) Put(0) Get Get Put(2) Get]
//  ARG_0_ORIGINAL (85 shrinks): initialState=State(size=7, elements=[])
//     sequential=[Put(-1855365712) Put(-1591723498) Get Size Size
//     Put(-1015561691) Get Put(397128011) Size Get Put(1943174048) Size
//     Put(1309500770) Size Get Put(-879438231) Size Get Put(-1644094687) Get
//     Put(-1818606323) Size Put(488620313) Size Put(-1219794505)
//     Put(1166147059) Get Put(11390361) Get Size Put(-1407993944) Get Get Size
//     Put(1393923085) Get Put(1222853245) Size Put(2070918543) Put(1741323168)
//     Size Get Get Size Put(2019939681) Get Put(-170089451) Size Get Get Size
//     Size Put(-49249034) Put(1229062846) Put(642598551) Get Put(1183453167)
//     Size Get Get Get Put(1010460728) Put(6828709) Put(-185198587) Size Size
//     Get Put(586459644) Get Size Put(-1802196502) Get Size Put(2097590857) Get
//     Get Get Get Size Put(-474576011) Size Get Size Size Put(771190414) Size
//     Put(-1509199920) Get Put(967212411) Size Get Put(578995532) Size Get Size
//     Get]
//
// Though this is not the minimal possible combination of command, its already
// pretty close.
func Example_circularqueue() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	properties := gopter.NewProperties(parameters)

	properties.Property("circular buffer", commands.Prop(cbCommands))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! circular buffer: Falsified after 96 passed tests.
	// ARG_0: initialState=State(size=7, elements=[]) sequential=[Put(0) Put(0)
	//    Get Put(0) Get Put(0) Put(0) Get Put(0) Get Put(0) Get Put(-1) Put(0)
	//    Put(0) Put(0) Put(0) Get Get Put(2) Get]
	// ARG_0_ORIGINAL (85 shrinks): initialState=State(size=7, elements=[])
	//    sequential=[Put(-1855365712) Put(-1591723498) Get Size Size
	//    Put(-1015561691) Get Put(397128011) Size Get Put(1943174048) Size
	//    Put(1309500770) Size Get Put(-879438231) Size Get Put(-1644094687) Get
	//    Put(-1818606323) Size Put(488620313) Size Put(-1219794505)
	//    Put(1166147059) Get Put(11390361) Get Size Put(-1407993944) Get Get Size
	//    Put(1393923085) Get Put(1222853245) Size Put(2070918543) Put(1741323168)
	//    Size Get Get Size Put(2019939681) Get Put(-170089451) Size Get Get Size
	//    Size Put(-49249034) Put(1229062846) Put(642598551) Get Put(1183453167)
	//    Size Get Get Get Put(1010460728) Put(6828709) Put(-185198587) Size Size
	//    Get Put(586459644) Get Size Put(-1802196502) Get Size Put(2097590857) Get
	//    Get Get Get Size Put(-474576011) Size Get Size Size Put(771190414) Size
	//    Put(-1509199920) Get Put(967212411) Size Get Put(578995532) Size Get Size
	//    Get]
}
