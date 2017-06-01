package langs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// JavaLangHelper provides a set of helper methods for the build lifecycle of the Java runtime
type JavaLangHelper struct {
	BaseHelper
}

const (
	mainClass     = "Func"
	mainClassFile = mainClass + ".java"
)

// Entrypoint returns the Java runtime Docker entrypoint that will be executed when the function is run
func (lh *JavaLangHelper) Entrypoint() string {
	return fmt.Sprintf("java %s", mainClass)
}

// HasPreBuild returns whether the Java runtime has a pre-build step
func (lh *JavaLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild executes the pre-build step for the Java runtime which involves compiling the relevant classes. It expects
// the entrypoint to the function, in other words or the class with the main method (not to be confused with the Docker
// entrypoint from Entrypoint()) to be Function.java
func (lh *JavaLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, mainClassFile)) {
		return fmt.Errorf("could not find function: for Java, class with main method must be "+
			"called %s", mainClassFile)
	}

	cmd := exec.Command(
		"docker", "run",
		"--rm", "-v", wd+":/java", "-w", "/java",
		"funcy/java:dev",
		"/bin/sh", "-c", "javac "+mainClassFile,
	)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return dockerBuildError(err)
	}
	return nil
}

// AfterBuild removes all compiled class files from the host machine
func (lh *JavaLangHelper) AfterBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(wd, "*.class"))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}

// HasPreBuild returns whether the Java runtime has boilerplate that can be generated.
func (lh *JavaLangHelper) HasBoilerplate() bool { return true }

const javaFunctionBoilerplate = `import java.io.*;

public class Func {

	/**
	 * This is the entrypoint to your function. Input will be via STDIN.
	 * Any output sent to STDOUT will be sent back as the function result.
	 */
    public static void main(String[] args) throws IOException {
        BufferedReader bufferedReader = new BufferedReader(new InputStreamReader(System.in));

        String name = bufferedReader.readLine();
        name = (name == null) ? "world"  : name;

        System.out.println("Hello, " + name + "!");
    }

}
`

// GenerateBoilerplate will generate function boilerplate (Function.java) for java if it does not exist.
// Returns ErrBoilerplateExists if the function file already exists
func (lh *JavaLangHelper) GenerateBoilerplate() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pathToFunctionFile := filepath.Join(wd, mainClassFile)
	if exists(filepath.Join(wd, mainClassFile)) {
		return ErrBoilerplateExists
	}
	return ioutil.WriteFile(pathToFunctionFile, []byte(javaFunctionBoilerplate), os.FileMode(0644))
}
