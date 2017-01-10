package main

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
)

func TestMainCommands(t *testing.T) {
	testCommands := []string{
		"init",
		"apps",
		"routes",
		"images",
		"lambda",
		"version",
		"build",
		"bump",
		"deploy",
		"run",
		"push",
	}

	fnTestBin := path.Join(os.TempDir(), "fn-test")

	exec.Command("go", "build", "-o", fnTestBin).Run()

	for _, cmd := range testCommands {
		res, err := exec.Command(fnTestBin, strings.Split(cmd, " ")...).CombinedOutput()
		if bytes.Contains(res, []byte("command not found")) {
			t.Error(err)
		}
	}

	os.Remove(fnTestBin)
}
