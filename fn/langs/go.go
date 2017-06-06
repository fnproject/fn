package langs

type GoLangHelper struct {
	BaseHelper
}

func (lh *GoLangHelper) BuildFromImage() string {
	return "funcy/go:dev"
}

func (lh *GoLangHelper) RunFromImage() string {
	return "funcy/go"
}

func (h *GoLangHelper) DockerfileBuildCmds() []string {
	r := []string{}
	// more info on Go multi-stage builds: https://medium.com/travis-on-docker/multi-stage-docker-builds-for-creating-tiny-go-images-e0e1867efe5a
	// For now we assume that dependencies are vendored already, but we could vendor them
	// inside the container. Maybe we should check for /vendor dir and if it doesn't exist,
	// either run `dep init` if no Gopkg.toml/lock found or `dep ensure` if it's there.
	r = append(r, "ADD . /src")
	// if exists("Gopkg.toml") {
	// r = append(r,
	// 	"RUN go get -u github.com/golang/dep/cmd/dep",
	// 	"RUN cd /src && dep ensure",
	// )
	// }
	r = append(r, "RUN cd /src && go build -o func")
	return r
}

func (h *GoLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /src/func /function/",
	}
}

func (lh *GoLangHelper) Entrypoint() string {
	return "./func"
}
