package langs

import "fmt"

// GetLangHelper returns a LangHelper for the passed in language
func GetLangHelper(lang string) (LangHelper, error) {
	switch lang {
	case "go":
		return &GoLangHelper{}, nil
	case "node":
		return &NodeLangHelper{}, nil
	case "ruby":
		return &RubyLangHelper{}, nil
	}
	return nil, fmt.Errorf("No language helper found for %v", lang)
}

type LangHelper interface {
	Entrypoint() string
	HasPreBuild() bool
	PreBuild() error
	AfterBuild() error
}
