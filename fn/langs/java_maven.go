package langs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"errors"
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

// PreBuild runs "mvn package" in the root of the project. A local .m2 directory is created the first time this is run
// so that any pulled dependencies are cached in between builds. The local .m2 directory contains a hardlink to user's
// own settings.xml file and this is mounted into the build container.
func (lh *JavaMavenLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "pom.xml")) {
		return errors.New("Could not find pom.xml - are you sure this is a maven project?")
	}

	err = createLocalM2Dir(wd)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm",
		"-v", wd+":/java", "-w", "/java",
		"-v", wd+"/.m2:/root/.m2",
		"maven:3.5-jdk-8-alpine",
		"/bin/sh", "-c", "mvn package",
	)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return dockerBuildError(err)
	}
	return nil
}

// createLocalM2Dir creates a .m2 directory in the function's working directory and creates a hard link to the user's
// settings.xml file.
func createLocalM2Dir(wd string) error {
	usersSettingsFile := filepath.Join(os.Getenv("HOME"), ".m2/settings.xml")
	localSettingsFile := filepath.Join(wd, ".m2/settings.xml")

	if exists(localSettingsFile) {
		return nil
	}

	if !exists(usersSettingsFile) {
		return fmt.Errorf("Unable to find user's settings.xml at %s", usersSettingsFile)
	}

	if !exists(filepath.Dir(localSettingsFile)) {
		if err := os.Mkdir(filepath.Dir(localSettingsFile), 0755); err != nil {
			return fmt.Errorf("Unable to create a local .m2 directory: %s", err)
		}
	}

	return os.Link(usersSettingsFile, localSettingsFile)
}

// AfterBuild removes the target directory by mounting the working directory into a container and removingit. This is
// done inside a container as the folder is owned by root.
func (lh *JavaMavenLangHelper) AfterBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm",
		"-v", wd+":/root",
		"maven:3.5-jdk-8-alpine",
		"/bin/sh", "-c", "rm -r /root/target",
	)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error occured trying to delete the target directory: %s", err)
	}

	return nil
}
