package langs

type PythonLangHelper struct {
	BaseHelper
}

func (lh *PythonLangHelper) BuildFromImage() string {
	return "funcy/python:2-dev"
}

func (lh *PythonLangHelper) RunFromImage() string {
	return "funcy/python:2-dev"
}

func (lh *PythonLangHelper) Entrypoint() string {
	return "python2 func.py"
}

func (h *PythonLangHelper) DockerfileBuildCmds() []string {
	r := []string{}
	if exists("requirements.txt") {
		r = append(r,
			"ADD requirements.txt /function/",
			"RUN pip install -r requirements.txt",
			"ADD . /function/",
		)
	}
	return r
}

func (h *PythonLangHelper) IsMultiStage() bool {
	return false
}

// The multi-stage build didn't work, pip seems to be required for it to load the modules
// func (h *PythonLangHelper) DockerfileCopyCmds() []string {
// return []string{
// "ADD . /function/",
// "COPY --from=build-stage /root/.cache/pip/ /root/.cache/pip/",
// }
// }
