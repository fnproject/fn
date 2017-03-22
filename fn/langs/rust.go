package langs

import (
	"fmt"
	"os"
	"os/exec"
)

type RustLangHelper struct {
	BaseHelper
}

func (lh *RustLangHelper) Entrypoint() string {
	return "/function/target/release/func"
}

func (lh *RustLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild for rust builds the binary so the final image can be as small as possible
func (lh *RustLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm", "-v",
		wd+":/app", "-w", "/app", "corey/rust-alpine",
		"/bin/sh", "-c", "cargo build --release",
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}
	return nil
}

func (lh *RustLangHelper) AfterBuild() error {
	return os.RemoveAll("target")
}
