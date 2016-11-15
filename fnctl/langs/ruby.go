package langs

type RubyLangHelper struct {
}

func (lh *RubyLangHelper) Entrypoint() string {
	return "ruby func.rb"
}

func (lh *RubyLangHelper) HasPreBuild() bool {
	return false
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *RubyLangHelper) PreBuild() error {
	return nil
}

func (lh *RubyLangHelper) AfterBuild() error {
	return nil
}
