package langs

type NodeLangHelper struct {
	BaseHelper
}

func (lh *NodeLangHelper) BuildFromImage() string {
	return "funcy/node:dev"
}
func (lh *NodeLangHelper) RunFromImage() string {
	return "funcy/node"
}

func (lh *NodeLangHelper) Entrypoint() string {
	return "node func.js"
}

func (h *NodeLangHelper) DockerfileBuildCmds() []string {
	r := []string{}
	if exists("package.json") {
		r = append(r,
			"ADD package.json /function/",
			"RUN npm install",
		)
	}
	return r
}

func (h *NodeLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"ADD . /function/",
		"COPY --from=build-stage /function/node_modules/ /function/node_modules/",
	}
}
