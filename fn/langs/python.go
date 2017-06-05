package langs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type PythonLangHelper struct {
	BaseHelper
}

func (lh *PythonLangHelper) Entrypoint() string {
	return "python2 func.py"
}

func (lh *PythonLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *PythonLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pbcmd := fmt.Sprintf("docker run --rm -v %s:/worker -w /worker funcy/python:2-dev pip install -t packages -r requirements.txt", wd)
	fmt.Println("Running prebuild command:", pbcmd)
	parts := strings.Fields(pbcmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	cmd := exec.Command(head, parts...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return dockerBuildError(err)
	}
	return nil
}

func (lh *PythonLangHelper) AfterBuild() error {
	return nil
}
