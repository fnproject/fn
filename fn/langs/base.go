package langs

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
	case "rust":
		return &RustLangHelper{}
	case "dotnet":
		return &DotNetLangHelper{}
	case "lambda-nodejs4.3":
		return &LambdaNodeHelper{}
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
}

// BaseHelper is empty implementation of LangHelper for embedding in implementations.
type BaseHelper struct {
}

func (h *BaseHelper) Cmd() string { return "" }
