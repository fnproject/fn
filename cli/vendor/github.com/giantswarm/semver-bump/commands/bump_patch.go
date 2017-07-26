package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var bumpPatchCommand = &cobra.Command{
	Use:   "patch-release",
	Short: "Bump a patch release",
	Long:  `Increments the patch version and bumps it.`,
	Run: func(cmd *cobra.Command, args []string) {
		sb, err := getSemverBumper()

		if err != nil {
			log.Fatal(err)
		}

		v, err := sb.BumpPatchVersion(versionPreReleaseSuffix, versionMetadataSuffix)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Bumped to patch version %s\n", v.String())

	},
}
