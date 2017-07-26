package langs

type LambdaNodeHelper struct {
	BaseHelper
}

func (lh *LambdaNodeHelper) BuildFromImage() string {
	return "funcy/lambda:node-4"
}

func (lh *LambdaNodeHelper) IsMultiStage() bool {
	return false
}

func (lh *LambdaNodeHelper) Cmd() string {
	return "func.handler"
}

func (h *LambdaNodeHelper) DockerfileBuildCmds() []string {
	r := []string{}
	if exists("package.json") {
		r = append(r,
			"ADD package.json /function/",
			"RUN npm install",
		)
	}
	// single stage build for this one, so add files
	r = append(r, "ADD . /function/")
	return r
}
