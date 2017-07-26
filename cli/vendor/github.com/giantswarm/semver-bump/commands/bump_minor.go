package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var bumpMinorCommand = &cobra.Command{
	Use:   "minor-release",
	Short: "Bump a minor release",
	Long:  `Increments the minor version and bumps it.`,
	Run: func(cmd *cobra.Command, args []string) {
		sb, err := getSemverBumper()

		if err != nil {
			log.Fatal(err)
		}

		v, err := sb.BumpMinorVersion(versionPreReleaseSuffix, versionMetadataSuffix)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Bumped to minor version %s\n", v.String())
	},
}
