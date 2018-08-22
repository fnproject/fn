package commands

import "github.com/leanovate/gopter"

// SystemUnderTest resembles the system under test, which may be any kind
// of stateful unit of code
type SystemUnderTest interface{}

// State resembles the state the system under test is expected to be in
type State interface{}

// Result resembles the result of a command that may or may not be checked
type Result interface{}

// Command is any kind of command that may be applied to the system under test
type Command interface {
	// Run applies the command to the system under test
	Run(systemUnderTest SystemUnderTest) Result
	// NextState calculates the next expected state if the command is applied
	NextState(state State) State
	// PreCondition checks if the state is valid before the command is applied
	PreCondition(state State) bool
	// PostCondition checks if the state is valid after the command is applied
	PostCondition(state State, result Result) *gopter.PropResult
	// String gets a (short) string representation of the command
	String() string
}

// ProtoCommand is a prototype implementation of the Command interface
type ProtoCommand struct {
	Name              string
	RunFunc           func(systemUnderTest SystemUnderTest) Result
	NextStateFunc     func(state State) State
	PreConditionFunc  func(state State) bool
	PostConditionFunc func(state State, result Result) *gopter.PropResult
}

// Run applies the command to the system under test
func (p *ProtoCommand) Run(systemUnderTest SystemUnderTest) Result {
	if p.RunFunc != nil {
		return p.RunFunc(systemUnderTest)
	}
	return nil
}

// NextState calculates the next expected state if the command is applied
func (p *ProtoCommand) NextState(state State) State {
	if p.NextStateFunc != nil {
		return p.NextStateFunc(state)
	}
	return state
}

// PreCondition checks if the state is valid before the command is applied
func (p *ProtoCommand) PreCondition(state State) bool {
	if p.PreConditionFunc != nil {
		return p.PreConditionFunc(state)
	}
	return true
}

// PostCondition checks if the state is valid after the command is applied
func (p *ProtoCommand) PostCondition(state State, result Result) *gopter.PropResult {
	if p.PostConditionFunc != nil {
		return p.PostConditionFunc(state, result)
	}
	return &gopter.PropResult{Status: gopter.PropTrue}
}

func (p *ProtoCommand) String() string {
	return p.Name
}
