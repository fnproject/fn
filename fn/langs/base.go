package langs

import (
	"errors"
	"fmt"
	"os"
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
		return &PythonLangHelper{}
	case "php":
		return &PhpLangHelper{}
	case "rust":
		return &RustLangHelper{}
	case "dotnet":
		return &DotNetLangHelper{}
	case "lambda-nodejs4.3", "lambda-node-4":
		return &LambdaNodeHelper{}
	case "java":
		return &JavaLangHelper{}
	}
	return nil
}

type LangHelper interface {
	// BuildFromImage is the base image to build off, typically funcy/LANG:dev
	BuildFromImage() string
	// RunFromImage is the base image to use for deployment (usually smaller than the build images)
	RunFromImage() string
	// If set to false, it will use a single Docker build step, rather than multi-stage
	IsMultiStage() bool
	// Dockerfile build lines for building dependencies or anything else language specific
	DockerfileBuildCmds() []string
	// DockerfileCopyCmds will run in second/final stage of multi-stage build to copy artifacts form the build stage
	DockerfileCopyCmds() []string
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

func (h *BaseHelper) BuildFromImage() string        { return "" }
func (h *BaseHelper) RunFromImage() string          { return h.BuildFromImage() }
func (h *BaseHelper) IsMultiStage() bool            { return true }
func (h *BaseHelper) DockerfileBuildCmds() []string { return []string{} }
func (h *BaseHelper) DockerfileCopyCmds() []string  { return []string{} }
func (h *BaseHelper) Entrypoint() string            { return "" }
func (h *BaseHelper) Cmd() string                   { return "" }
func (h *BaseHelper) HasPreBuild() bool             { return false }
func (h *BaseHelper) PreBuild() error               { return nil }
func (h *BaseHelper) AfterBuild() error             { return nil }
func (h *BaseHelper) HasBoilerplate() bool          { return false }
func (h *BaseHelper) GenerateBoilerplate() error    { return nil }

// exists checks if a file exists
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
