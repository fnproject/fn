package langs

import "fmt"

// JavaMavenLangHelper provides a set of helper methods for the build lifecycle of Java Maven projects
type JavaMavenLangHelper struct {
	BaseHelper
}

// Entrypoint returns the Java runtime Docker entrypoint that will be executed when the function is run
func (lh *JavaMavenLangHelper) Entrypoint() string {
	// TODO need mechanism to determine the java user function dynamically
	userFunction := "com.example.faas.ExampleFunction::itsOn"
	return fmt.Sprintf("java -jar /function/target/function.jar %s", userFunction)
}

func (lh *JavaMavenLangHelper) HasLocalBuildCmd() bool {
	return true
}

func (lh *JavaMavenLangHelper) LocalBuildCmd() []string {
	return []string{
		"mvn clean package",
	}
}

func (lh *JavaMavenLangHelper) HasPreBuild() bool { return false }

func (lh *JavaMavenLangHelper) PreBuild() error { return nil }

func (lh *JavaMavenLangHelper) AfterBuild() error { return nil }
