package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information (set at build time)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			fmt.Printf(`{"version":"%s","commit":"%s","date":"%s"}`+"\n", Version, GitCommit, BuildDate)
		} else {
			fmt.Printf("stash version %s\n", Version)
			if verbose {
				fmt.Printf("  commit: %s\n", GitCommit)
				fmt.Printf("  built:  %s\n", BuildDate)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
