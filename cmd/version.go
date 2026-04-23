package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X github.com/equisolve/teamwork-cli/cmd.version=..."
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
