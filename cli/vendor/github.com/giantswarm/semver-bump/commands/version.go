package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of semver-bump",
	Long:  `Print the version number of semver-bump.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Semantic Versioning Bumper %s\n", projectVersion)
	},
}
