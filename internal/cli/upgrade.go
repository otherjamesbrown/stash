// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
)

// StashMetadata stores version and configuration information for a stash installation.
type StashMetadata struct {
	LastStashVersion string `json:"last_stash_version,omitempty"`
	SchemaVersion    int    `json:"schema_version,omitempty"`
}

// CurrentSchemaVersion is the current database schema version.
// Increment this when making breaking schema changes.
const CurrentSchemaVersion = 1

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Check and manage stash version upgrades",
	Long: `Commands for checking stash version upgrades and reviewing changes.

The upgrade command helps you stay aware of stash version changes:
  - stash upgrade status: Check if stash version changed since last use
  - stash upgrade ack: Acknowledge the current version

Version tracking is automatic - stash records the acknowledged version
in .stash/metadata.json.

Examples:
  stash upgrade status           # Check if version changed
  stash upgrade ack              # Acknowledge current version

AI Agent Examples:
  # Check if upgrade occurred before running commands
  if stash upgrade status --json | jq -e '.upgraded' >/dev/null; then
    echo "Stash was upgraded, running migrate..."
    stash migrate
  fi

Exit Codes:
  0  Success
  1  No .stash directory found`,
}

var upgradeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if stash version has changed",
	Long: `Check if the stash binary version has changed since last acknowledgment.

This compares the current binary version against the last acknowledged version
stored in .stash/metadata.json.

Examples:
  stash upgrade status           # Human-readable output
  stash upgrade status --json    # JSON for scripting

Exit Codes:
  0  Success
  1  No .stash directory found

JSON Output (--json):
  {"current_version": "1.2.0", "last_version": "1.1.0", "upgraded": true}`,
	Args: cobra.NoArgs,
	RunE: runUpgradeStatus,
}

var upgradeAckCmd = &cobra.Command{
	Use:   "ack",
	Short: "Acknowledge the current stash version",
	Long: `Acknowledge the current stash version to suppress upgrade notifications.

This updates .stash/metadata.json with the current binary version.

Examples:
  stash upgrade ack              # Acknowledge current version
  stash upgrade ack --json       # JSON output

Exit Codes:
  0  Success
  1  No .stash directory found`,
	Args: cobra.NoArgs,
	RunE: runUpgradeAck,
}

func init() {
	upgradeCmd.AddCommand(upgradeStatusCmd)
	upgradeCmd.AddCommand(upgradeAckCmd)
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgradeStatus(cmd *cobra.Command, args []string) error {
	// Find .stash directory
	ctx, err := context.Resolve(GetActorName(), "")
	if err != nil || ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load metadata
	metadata, err := loadStashMetadata(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	currentVersion := Version
	lastVersion := metadata.LastStashVersion
	upgraded := lastVersion != "" && lastVersion != currentVersion

	if GetJSONOutput() {
		output := map[string]interface{}{
			"current_version": currentVersion,
			"last_version":    lastVersion,
			"upgraded":        upgraded,
			"schema_version":  CurrentSchemaVersion,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Current version: %s\n", currentVersion)
		if lastVersion == "" {
			fmt.Println("Last version: (not recorded)")
			fmt.Println("\nRun 'stash upgrade ack' to record the current version.")
		} else if upgraded {
			fmt.Printf("Last version: %s\n", lastVersion)
			fmt.Println("\nStash has been upgraded!")
			fmt.Println("Run 'stash migrate' to apply any database migrations.")
			fmt.Println("Run 'stash upgrade ack' to acknowledge this version.")
		} else {
			fmt.Printf("Last version: %s\n", lastVersion)
			fmt.Println("\nNo upgrade detected.")
		}
	}

	return nil
}

func runUpgradeAck(cmd *cobra.Command, args []string) error {
	// Find .stash directory
	ctx, err := context.Resolve(GetActorName(), "")
	if err != nil || ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load existing metadata
	metadata, err := loadStashMetadata(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	previousVersion := metadata.LastStashVersion
	metadata.LastStashVersion = Version

	// Save metadata
	if err := saveStashMetadata(ctx.StashDir, metadata); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if GetJSONOutput() {
		output := map[string]interface{}{
			"acknowledged":     true,
			"version":          Version,
			"previous_version": previousVersion,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if previousVersion == "" {
			fmt.Printf("Acknowledged version: %s (first time)\n", Version)
		} else if previousVersion != Version {
			fmt.Printf("Acknowledged version: %s (was %s)\n", Version, previousVersion)
		} else {
			fmt.Printf("Version %s already acknowledged\n", Version)
		}
	}

	return nil
}

// metadataFilePath returns the path to the metadata.json file.
func metadataFilePath(stashDir string) string {
	return filepath.Join(stashDir, "metadata.json")
}

// loadStashMetadata loads metadata from .stash/metadata.json.
func loadStashMetadata(stashDir string) (*StashMetadata, error) {
	path := metadataFilePath(stashDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StashMetadata{}, nil
		}
		return nil, err
	}

	var metadata StashMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// saveStashMetadata saves metadata to .stash/metadata.json.
func saveStashMetadata(stashDir string, metadata *StashMetadata) error {
	path := metadataFilePath(stashDir)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
