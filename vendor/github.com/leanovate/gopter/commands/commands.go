package commands

import (
	"reflect"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Commands provide an entry point for testing a stateful system
type Commands interface {
	// NewSystemUnderTest should create a new/isolated system under test
	NewSystemUnderTest(initialState State) SystemUnderTest
	// DestroySystemUnderTest may perform any cleanup tasks to destroy a system
	DestroySystemUnderTest(SystemUnderTest)
	// GenInitialState provides a generator for the initial State.
	// IMPORTANT: The generated state itself may be mutable, but this generator
	// is supposed to generate a clean and reproductable state every time.
	// Do not use an external random generator and be especially vary about
	// `gen.Const(<pointer to some mutable struct>)`.
	GenInitialState() gopter.Gen
	// GenCommand provides a generator for applicable commands to for a state
	GenCommand(state State) gopter.Gen
	// InitialPreCondition checks if the initial state is valid
	InitialPreCondition(state State) bool
}

// ProtoCommands is a prototype implementation of the Commands interface
type ProtoCommands struct {
	NewSystemUnderTestFunc     func(initialState State) SystemUnderTest
	DestroySystemUnderTestFunc func(SystemUnderTest)
	InitialStateGen            gopter.Gen
	GenCommandFunc             func(State) gopter.Gen
	InitialPreConditionFunc    func(State) bool
}

// NewSystemUnderTest should create a new/isolated system under test
func (p *ProtoCommands) NewSystemUnderTest(initialState State) SystemUnderTest {
	if p.NewSystemUnderTestFunc != nil {
		return p.NewSystemUnderTestFunc(initialState)
	}
	return nil
}

// DestroySystemUnderTest may perform any cleanup tasks to destroy a system
func (p *ProtoCommands) DestroySystemUnderTest(systemUnderTest SystemUnderTest) {
	if p.DestroySystemUnderTestFunc != nil {
		p.DestroySystemUnderTestFunc(systemUnderTest)
	}
}

// GenCommand provides a generator for applicable commands to for a state
func (p *ProtoCommands) GenCommand(state State) gopter.Gen {
	if p.GenCommandFunc != nil {
		return p.GenCommandFunc(state)
	}
	return gen.Fail(reflect.TypeOf((*Command)(nil)).Elem())
}

// GenInitialState provides a generator for the initial State
func (p *ProtoCommands) GenInitialState() gopter.Gen {
	return p.InitialStateGen.SuchThat(func(state State) bool {
		return p.InitialPreCondition(state)
	})
}

// InitialPreCondition checks if the initial state is valid
func (p *ProtoCommands) InitialPreCondition(state State) bool {
	if p.InitialPreConditionFunc != nil {
		return p.InitialPreConditionFunc(state)
	}
	return true
}

// Prop creates a gopter.Prop from Commands
func Prop(commands Commands) gopter.Prop {
	return prop.ForAll(func(actions *actions) (*gopter.PropResult, error) {
		systemUnderTest := commands.NewSystemUnderTest(actions.initialStateProvider())
		defer commands.DestroySystemUnderTest(systemUnderTest)

		return actions.run(systemUnderTest)
	}, genActions(commands))
}
