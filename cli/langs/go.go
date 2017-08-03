package langs

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

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
	r = append(r, "ADD . /go/src/func/")
	// if exists("Gopkg.toml") {
	// r = append(r,
	// 	"RUN go get -u github.com/golang/dep/cmd/dep",
	// 	"RUN cd /src && dep ensure",
	// )
	// }
	r = append(r, "RUN cd /go/src/func/ && go build -o func")
	return r
}

func (h *GoLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /go/src/func/func /function/",
	}
}

func (lh *GoLangHelper) Entrypoint() string {
	return "./func"
}

// HasPreBuild returns whether the Java runtime has boilerplate that can be generated.
func (lh *GoLangHelper) HasBoilerplate() bool { return true }

// GenerateBoilerplate will generate function boilerplate for a Java runtime. The default boilerplate is for a Maven
// project.
func (lh *GoLangHelper) GenerateBoilerplate() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	codeFile := filepath.Join(wd, "func.go")
	if exists(codeFile) {
		return ErrBoilerplateExists
	}
	testFile := filepath.Join(wd, "test.json")
	if exists(testFile) {
		return ErrBoilerplateExists
	}

	if err := ioutil.WriteFile(codeFile, []byte(helloGoSrcBoilerplate), os.FileMode(0644)); err != nil {
		return err
	}

	if err := ioutil.WriteFile(testFile, []byte(testBoilerPlate), os.FileMode(0644)); err != nil {
		return err
	}
	return nil
}

const (
	helloGoSrcBoilerplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Person struct {
	Name string
}

func main() {
	p := &Person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	mapD := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
	mapB, _ := json.Marshal(mapD)
	fmt.Println(string(mapB))
}
`

	// Could use same test for most langs
	testBoilerPlate = `{
    "tests": [
        {
            "input": {
                "body": {
                    "name": "Johnny"
                }
            },
            "output": {
                "body": {
                    "message": "Hello Johnny"
                }
            }
        },
        {
            "input": {
                "body": ""
            },
            "output": {
                "body": {
                    "message": "Hello World"
                }
            }
        }
    ]
}
`
)
