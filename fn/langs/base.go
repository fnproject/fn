package langs

import (
	"errors"
	"os"
	"fmt"
)

var (
	ErrBoilerplateExists = errors.New("Function boilerplate already exists")
)

// GetLangHelper returns a LangHelper for the passed in language
func GetLangHelper(lang string) LangHelper {
	switch lang {
	case "go":
		return &GoLangHelper{}
	case "node":
		return &NodeLangHelper{}
	case "ruby":
		return &RubyLangHelper{}
	case "python":
		return &PythonHelper{}
	case "php":
		return &PhpLangHelper{}
	case "rust":
		return &RustLangHelper{}
	case "dotnet":
		return &DotNetLangHelper{}
	case "lambda-nodejs4.3":
		return &LambdaNodeHelper{}
	case "java":
		return &JavaLangHelper{}
	}
	return nil
}

type LangHelper interface {
	// Entrypoint sets the Docker Entrypoint. One of Entrypoint or Cmd is required.
	Entrypoint() string
	// Cmd sets the Docker command. One of Entrypoint or Cmd is required.
	Cmd() string
	HasPreBuild() bool
	PreBuild() error
	AfterBuild() error
	// HasBoilerplate indicates whether a language has support for generating function boilerplate.
	HasBoilerplate() bool
	// GenerateBoilerplate generates basic function boilerplate. Returns ErrBoilerplateExists if the function file
	// already exists.
	GenerateBoilerplate() error
}

// BaseHelper is empty implementation of LangHelper for embedding in implementations.
type BaseHelper struct {
}

func (h *BaseHelper) Cmd() string { return "" }
func (h *BaseHelper) HasBoilerplate() bool { return false }
func (h *BaseHelper) GenerateBoilerplate() error { return nil }

func exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func dockerBuildError(err error) error {
	return fmt.Errorf("error running docker build: %v", err)
}