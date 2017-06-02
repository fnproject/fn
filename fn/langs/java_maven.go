package langs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// JavaMavenLangHelper provides a set of helper methods for the build lifecycle of Java Maven projects
type JavaMavenLangHelper struct {
	BaseHelper
}


// Entrypoint returns the Java runtime Docker entrypoint that will be executed when the function is run
func (lh *JavaMavenLangHelper) Entrypoint() string {
	return fmt.Sprintf("java -jar /function/target/function.jar com.example.faas.ExampleFunction::itsOn")
}

// HasPreBuild returns whether the Java runtime has a pre-build step
func (lh *JavaMavenLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild runs "mvn clean package" in the root of the project and by default expects a jar at `target/function.jar` as the
// output.  We mount `$HOME/.m2` in order to get maven the proxy config specified in .m2/settings.xml, but it has the nice
// side effect of making the users .m2/repository available.  Any new deps downloaded will be owned by root :(
func (lh *JavaMavenLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "pom.xml")) {
		return fmt.Errorf("Could not find pom.xml - are you sure this is a maven project?")
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm",
		"-v", wd+":/java", "-w", "/java",
		"-v", os.Getenv("HOME")+"/.m2:/.m2",
		"maven:3.5-jdk-8-alpine",
		"/bin/sh", "-c", "mvn -gs /.m2/settings.xml clean package",
	)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return dockerBuildError(err)
	}
	return nil
}

// AfterBuild should remove the (root-owned) target dir.
func (lh *JavaMavenLangHelper) AfterBuild() error {
	return nil
}
