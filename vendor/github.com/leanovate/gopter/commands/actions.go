package commands

import (
	"fmt"
	"reflect"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

type shrinkableCommand struct {
	command  Command
	shrinker gopter.Shrinker
}

func (s shrinkableCommand) shrink() gopter.Shrink {
	return s.shrinker(s.command).Map(func(command Command) shrinkableCommand {
		return shrinkableCommand{
			command:  command,
			shrinker: s.shrinker,
		}
	})
}

func (s shrinkableCommand) String() string {
	return fmt.Sprintf("%v", s.command)
}

type actions struct {
	// initialStateProvider has to reset/recreate the initial state exactly the
	// same every time.
	initialStateProvider func() State
	sequentialCommands   []shrinkableCommand
	// parallel commands will come later
}

func (a *actions) String() string {
	return fmt.Sprintf("initialState=%v sequential=%s", a.initialStateProvider(), a.sequentialCommands)
}

func (a *actions) run(systemUnderTest SystemUnderTest) (*gopter.PropResult, error) {
	state := a.initialStateProvider()
	propResult := &gopter.PropResult{Status: gopter.PropTrue}
	for _, shrinkableCommand := range a.sequentialCommands {
		if !shrinkableCommand.command.PreCondition(state) {
			return &gopter.PropResult{Status: gopter.PropFalse}, nil
		}
		result := shrinkableCommand.command.Run(systemUnderTest)
		state = shrinkableCommand.command.NextState(state)
		propResult = propResult.And(shrinkableCommand.command.PostCondition(state, result))
	}
	return propResult, nil
}

type sizedCommands struct {
	state    State
	commands []shrinkableCommand
}

func actionsShrinker(v interface{}) gopter.Shrink {
	a := v.(*actions)
	elementShrinker := gopter.Shrinker(func(v interface{}) gopter.Shrink {
		return v.(shrinkableCommand).shrink()
	})
	return gen.SliceShrinker(elementShrinker)(a.sequentialCommands).Map(func(v []shrinkableCommand) *actions {
		return &actions{
			initialStateProvider: a.initialStateProvider,
			sequentialCommands:   v,
		}
	})
}

func genActions(commands Commands) gopter.Gen {
	genInitialState := commands.GenInitialState()
	genInitialStateProvider := gopter.Gen(func(params *gopter.GenParameters) *gopter.GenResult {
		seed := params.NextInt64()
		return gopter.NewGenResult(func() State {
			paramsWithSeed := params.CloneWithSeed(seed)
			if initialState, ok := genInitialState(paramsWithSeed).Retrieve(); ok {
				return initialState
			}
			return nil
		}, gopter.NoShrinker)
	}).SuchThat(func(initialStateProvoder func() State) bool {
		state := initialStateProvoder()
		return state != nil && commands.InitialPreCondition(state)
	})
	return genInitialStateProvider.FlatMap(func(v interface{}) gopter.Gen {
		initialStateProvider := v.(func() State)
		return genSizedCommands(commands, initialStateProvider).Map(func(v sizedCommands) *actions {
			return &actions{
				initialStateProvider: initialStateProvider,
				sequentialCommands:   v.commands,
			}
		}).SuchThat(func(actions *actions) bool {
			state := actions.initialStateProvider()
			for _, shrinkableCommand := range actions.sequentialCommands {
				if !shrinkableCommand.command.PreCondition(state) {
					return false
				}
				state = shrinkableCommand.command.NextState(state)
			}
			return true
		}).WithShrinker(actionsShrinker)
	}, reflect.TypeOf((*actions)(nil)))
}

func genSizedCommands(commands Commands, initialStateProvider func() State) gopter.Gen {
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		sizedCommandsGen := gen.Const(sizedCommands{
			state:    initialStateProvider(),
			commands: make([]shrinkableCommand, 0, genParams.MaxSize),
		})
		for i := 0; i < genParams.MaxSize; i++ {
			sizedCommandsGen = sizedCommandsGen.FlatMap(func(v interface{}) gopter.Gen {
				prev := v.(sizedCommands)
				return gen.RetryUntil(commands.GenCommand(prev.state), func(command Command) bool {
					return command.PreCondition(prev.state)
				}, 100).MapResult(func(result *gopter.GenResult) *gopter.GenResult {
					value, ok := result.Retrieve()
					if !ok {
						return gopter.NewEmptyResult(reflect.TypeOf(sizedCommands{}))
					}
					command := value.(Command)
					return gopter.NewGenResult(
						sizedCommands{
							state: command.NextState(prev.state),
							commands: append(prev.commands, shrinkableCommand{
								command:  command,
								shrinker: result.Shrinker,
							}),
						},
						gopter.NoShrinker,
					)
				})
			}, reflect.TypeOf(sizedCommands{}))
		}
		return sizedCommandsGen(genParams)
	}
}
