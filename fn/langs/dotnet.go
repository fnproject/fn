package langs

import (
	"fmt"
	"os"
	"os/exec"
)

type DotNetLangHelper struct {
	BaseHelper
}

func (lh *DotNetLangHelper) Entrypoint() string {
	return "dotnet dotnet.dll"
}

func (lh *DotNetLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild for Go builds the binary so the final image can be as small as possible
func (lh *DotNetLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm", "-v",
		wd+":/dotnet", "-w", "/dotnet", "microsoft/dotnet:1.0.1-sdk-projectjson",
		"/bin/sh", "-c", "dotnet restore && dotnet publish -c release -b /tmp -o .",
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}
	return nil
}

func (lh *DotNetLangHelper) AfterBuild() error {
	return nil
}
