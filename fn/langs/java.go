package langs

import (
	"fmt"
	"io/ioutil"
	"os"
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

// BuildFromImage returns the Docker image used to compile the Java function
func (lh *JavaLangHelper) BuildFromImage() string {
	return "funcy/java:dev"
}

// RunFromImage returns the Docker image used to run the Java function
func (lh *JavaLangHelper) RunFromImage() string {
	return "funcy/java"
}

// DockerfileBuildCmds returns the build stage steps to compile the Java function
func (lh *JavaLangHelper) DockerfileBuildCmds() []string {
	return []string{
		fmt.Sprintf("ADD %s . /src/", mainClassFile),
		fmt.Sprintf("RUN cd /src && javac %s", mainClassFile),
	}
}

// DockerfileCopyCmds returns the Docker COPY command to copy the compiled Java function classes
func (h *JavaLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /src/ /function/",
	}
}

// Entrypoint returns the Java runtime Docker entrypoint that will be executed when the function is run
func (lh *JavaLangHelper) Entrypoint() string {
	return fmt.Sprintf("java %s", mainClass)
}

// HasPreBuild returns whether the Java runtime has a pre-build step
func (lh *JavaLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild ensures that the expected Java source file is there before the build is executed. Returns an error if
// `mainClassFile` is not in the working directory
func (lh *JavaLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, mainClassFile)) {
		return fmt.Errorf("could not find function: for Java, class with main method must be "+
			"called %s", mainClassFile)
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
