package langs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type PythonHelper struct {
	BaseHelper
}

func (lh *PythonHelper) Entrypoint() string {
	return "python2 func.py"
}

func (lh *PythonHelper) HasPreBuild() bool {
	return true
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *PythonHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pbcmd := fmt.Sprintf("docker run --rm -v %s:/worker -w /worker iron/python:2-dev pip install -t packages -r requirements.txt", wd)
	fmt.Println("Running prebuild command:", pbcmd)
	parts := strings.Fields(pbcmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	cmd := exec.Command(head, parts...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}
	return nil
}

func (lh *PythonHelper) AfterBuild() error {
	return nil
}
