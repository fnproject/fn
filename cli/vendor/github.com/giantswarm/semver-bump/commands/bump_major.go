package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var bumpMajorCommand = &cobra.Command{
	Use:   "major-release",
	Short: "Bump a major release",
	Long:  `Increments the major version and bumps it.`,
	Run: func(cmd *cobra.Command, args []string) {
		sb, err := getSemverBumper()

		if err != nil {
			log.Fatal(err)
		}

		v, err := sb.BumpMajorVersion(versionPreReleaseSuffix, versionMetadataSuffix)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Bumped to major version %s\n", v.String())
	},
}
