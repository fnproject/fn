package langs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type GoLangHelper struct {
	BaseHelper
}

func (lh *GoLangHelper) Entrypoint() string {
	return "./func"
}

func (lh *GoLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *GoLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	// todo: this won't work if the function is more complex since the import paths won't match up, need to fix
	pbcmd := fmt.Sprintf("docker run --rm -v %s:/go/src/github.com/x/y -w /go/src/github.com/x/y iron/go:dev go build -o func", wd)
	fmt.Println("Running prebuild command:", pbcmd)
	parts := strings.Fields(pbcmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	cmd := exec.Command(head, parts...)
	// cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}
	return nil
}

func (lh *GoLangHelper) AfterBuild() error {
	return os.Remove("func")

}
