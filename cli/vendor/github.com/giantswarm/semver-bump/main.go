package main

import "github.com/giantswarm/semver-bump/commands"

var projectVersion string = "dev"

func main() {
	commands.Execute(projectVersion)
}
