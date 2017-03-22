package langs

type LambdaNodeHelper struct {
	BaseHelper
}

func (lh *LambdaNodeHelper) Entrypoint() string {
	return ""
}

func (lh *LambdaNodeHelper) Cmd() string {
	return "func.handler"
}

func (lh *LambdaNodeHelper) HasPreBuild() bool {
	return false
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *LambdaNodeHelper) PreBuild() error {
	return nil
}

func (lh *LambdaNodeHelper) AfterBuild() error {
	return nil
}
