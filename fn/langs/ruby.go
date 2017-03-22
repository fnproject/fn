package langs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RubyLangHelper struct {
	BaseHelper
}

func (lh *RubyLangHelper) Entrypoint() string {
	return "ruby func.rb"
}

func (lh *RubyLangHelper) HasPreBuild() bool {
	return true
}

func (lh *RubyLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "Gemfile")) {
		return nil
	}

	pbcmd := fmt.Sprintf("docker run --rm -v %s:/worker -w /worker iron/ruby:dev bundle install --standalone --clean", wd)
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

func (lh *RubyLangHelper) AfterBuild() error {
	return nil
}

func exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
