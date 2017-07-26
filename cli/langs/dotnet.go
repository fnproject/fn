package langs

import (
	"os"
	"os/exec"
)

type DotNetLangHelper struct {
	BaseHelper
}

func (lh *DotNetLangHelper) BuildFromImage() string {
	return "microsoft/dotnet:1.0.1-sdk-projectjson"
}
func (lh *DotNetLangHelper) RunFromImage() string {
	return "microsoft/dotnet:runtime"
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
		return dockerBuildError(err)
	}
	return nil
}

func (lh *DotNetLangHelper) AfterBuild() error {
	return nil
}
