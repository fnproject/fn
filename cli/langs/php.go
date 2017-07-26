package langs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PhpLangHelper struct {
	BaseHelper
}

func (lh *PhpLangHelper) BuildFromImage() string {
	return "funcy/php:dev"
}
func (lh *PhpLangHelper) Entrypoint() string {
	return "php func.php"
}

func (lh *PhpLangHelper) HasPreBuild() bool {
	return true
}

func (lh *PhpLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "composer.json")) {
		return nil
	}

	pbcmd := fmt.Sprintf("docker run --rm -v %s:/worker -w /worker funcy/php:dev composer install", wd)
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

func (lh *PhpLangHelper) AfterBuild() error {
	return nil
}
