// Package cli provides the command-line interface for stash.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Global flags
var (
	jsonOutput bool
	stashName  string
	actorName  string
	quiet      bool
	verbose    bool
	noDaemon   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stash",
	Short: "A record-centric structured data store for AI agents",
	Long: `Stash is a lightweight, single-binary tool that provides AI agents
with a structured way to collect, organize, and query any kind of data.

Features:
  - Fluid schema: Add columns anytime without migrations
  - Hierarchical records: Parent-child relationships with dot notation IDs
  - Dual storage: JSONL source of truth + SQLite cache for queries
  - Full audit trail: Track who created/modified records and when
  - Agent-native: JSON output, context injection, conversational commands`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (for agent parsing)")
	rootCmd.PersistentFlags().StringVar(&stashName, "stash", "", "Target specific stash (default: auto-detect or $STASH_DEFAULT)")
	rootCmd.PersistentFlags().StringVar(&actorName, "actor", "", "Override actor for audit trail (default: $STASH_ACTOR or $USER)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable debug output")
	rootCmd.PersistentFlags().BoolVar(&noDaemon, "no-daemon", false, "Bypass daemon, direct file access")
}

// ExitCode is used to communicate exit codes for testing
var ExitCode int

// ExitFunc is the function called to exit the program
// Can be overridden for testing
var ExitFunc = os.Exit

// Exit sets the exit code and calls the exit function
func Exit(code int) {
	ExitCode = code
	ExitFunc(code)
}

// GetJSONOutput returns whether JSON output is enabled
func GetJSONOutput() bool {
	return jsonOutput
}

// GetStashName returns the target stash name
func GetStashName() string {
	return stashName
}

// GetActorName returns the actor name override
func GetActorName() string {
	return actorName
}

// IsQuiet returns whether quiet mode is enabled
func IsQuiet() bool {
	return quiet
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// NoDaemon returns whether daemon should be bypassed
func NoDaemon() bool {
	return noDaemon
}
