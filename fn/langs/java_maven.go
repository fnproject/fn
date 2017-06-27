package langs

import (
	"fmt"
	"os"
	"path/filepath"
	"errors"
	"bytes"
	"strings"
	"net/url"
	"io/ioutil"
)

// JavaMavenLangHelper provides a set of helper methods for the build lifecycle of Java Maven projects
type JavaMavenLangHelper struct {
	BaseHelper
}

// BuildFromImage returns the Docker image used to compile the Maven function project
func (lh *JavaMavenLangHelper) BuildFromImage() string {
	return "maven:3.5-jdk-8-alpine"
}

// RunFromImage returns the Docker image used to run the Maven built function
func (lh *JavaMavenLangHelper) RunFromImage() string {
	return "funcy/java"
}


// DockerfileBuildCmds returns the build stage steps to compile the Maven function project
func (lh *JavaMavenLangHelper) DockerfileBuildCmds() []string {
	return []string{
		fmt.Sprintf("ENV MAVEN_OPTS %s", mavenOpts()),
		"ADD pom.xml /function/pom.xml",
		"RUN [\"mvn\", \"package\", \"dependency:go-offline\", \"-DstripVersion=true\", \"-Dmdep.prependGroupId=true\"," +
			" \"dependency:copy-dependencies\"]",
		"ADD src /function/src",
		"RUN [\"mvn\", \"package\"]",
	}
}

func mavenOpts() string {
	var opts bytes.Buffer

	if parsedURL, err := url.Parse(os.Getenv("http_proxy")); err == nil {
		opts.WriteString(fmt.Sprintf("-Dhttp.proxyHost=%s ", parsedURL.Hostname()))
		opts.WriteString(fmt.Sprintf("-Dhttp.proxyPort=%s ", parsedURL.Port()))
	}

	if parsedURL, err := url.Parse(os.Getenv("https_proxy")); err == nil {
		opts.WriteString(fmt.Sprintf("-Dhttps.proxyHost=%s ", parsedURL.Hostname()))
		opts.WriteString(fmt.Sprintf("-Dhttps.proxyPort=%s ", parsedURL.Port()))
	}

	nonProxyHost := os.Getenv("no_proxy")
	opts.WriteString(fmt.Sprintf("-Dhttp.nonProxyHosts=%s ", strings.Replace(nonProxyHost, ",", "|", -1)))

	opts.WriteString("-Dmaven.repo.local=/usr/share/maven/ref/repository")

	return opts.String()
}

// DockerfileCopyCmds returns the Docker COPY command to copy the compiled Java function classes
func (lh *JavaMavenLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /function/target/*.jar /function/app/",
		"COPY --from=build-stage /function/target/dependency/*.jar /function/lib/",
	}
}

// HasPreBuild returns whether the Java Maven runtime has a pre-build step
func (lh *JavaMavenLangHelper) HasPreBuild() bool {
	return true
}

// PreBuild ensures that the expected the function is based is a maven project
func (lh *JavaMavenLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "pom.xml")) {
		return errors.New("Could not find pom.xml - are you sure this is a maven project?")
	}

	return nil
}

// Entrypoint returns the Java runtime Docker entrypoint that will be executed when the function is run
func (lh *JavaMavenLangHelper) Entrypoint() string {
	return "java -cp app/*:lib/* com.oracle.faas.runtime.EntryPoint com.example.faas.HelloFunction::handleRequest"
}

// HasPreBuild returns whether the Java Maven runtime has boilerplate that can be generated.
func (lh *JavaMavenLangHelper) HasBoilerplate() bool { return true }

// GenerateBoilerplate will generate function boilerplate for a Java Maven runtime
func (lh *JavaMavenLangHelper) GenerateBoilerplate() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pathToPomFile := filepath.Join(wd, "pom.xml")
	if exists(pathToPomFile) {
		return ErrBoilerplateExists
	}

	if err := ioutil.WriteFile(pathToPomFile, []byte(pomFile), os.FileMode(0644)); err != nil {
		return err
	}

	helloJavaFunctionFileDir := filepath.Join(wd, "src/main/java/com/example/faas")
	if err = os.MkdirAll(helloJavaFunctionFileDir, os.FileMode(0755)); err != nil {
		os.Remove(pathToPomFile)
		return err
	}

	helloJavaFunctionFile := filepath.Join(helloJavaFunctionFileDir, "HelloFunction.java")
	return ioutil.WriteFile(helloJavaFunctionFile, []byte(helloJavaFunctionBoilerplate), os.FileMode(0644))
}


/*	TODO temporarily generate maven project boilerplate from hardcoded values.
 	Will eventually move to using a maven archetype.
*/

const (
	pomFile = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <properties>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>
    <groupId>com.example.faas</groupId>
    <artifactId>hello</artifactId>
    <version>1.0.0-SNAPSHOT</version>

    <repositories>
        <repository>
            <id>nexus-box</id>
            <url>http://10.167.103.241:8081/repository/maven-snapshots/</url>
        </repository>
    </repositories>

    <dependencies>
        <dependency>
            <groupId>com.oracle.faas</groupId>
            <artifactId>fdk</artifactId>
            <version>1.0.0-SNAPSHOT</version>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.3</version>
                <configuration>
                    <source>1.8</source>
                    <target>1.8</target>
                </configuration>
            </plugin>
        </plugins>
    </build>
</project>
`

	helloJavaFunctionBoilerplate = `package com.example.faas;

public class HelloFunction {

    public String handleRequest(String input) {
        String name = (input == null || input.isEmpty()) ? "world"  : input;

        return "Hello, " + name + "!";
    }

}`
)
