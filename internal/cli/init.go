// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var initPrefix string

var initCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Initialize a new stash",
	Long: `Initialize a new stash with the given name and prefix.

A stash is a named collection of records with a unique ID prefix.
The prefix is used to generate record IDs (e.g., inv-001, inv-002).

Prefix requirements:
  - 3-5 characters total
  - 2-4 lowercase letters followed by a dash
  - Examples: ab-, inv-, abcd-

Examples:
  stash init inventory --prefix inv-
  stash init contacts --prefix ct- --no-daemon`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initPrefix, "prefix", "", "Record ID prefix (required, e.g., inv-)")
	initCmd.MarkFlagRequired("prefix")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate stash name
	if err := model.ValidateStashName(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		Exit(2)
		return nil // Won't reach in normal execution
	}

	// Validate prefix
	if err := model.ValidatePrefix(initPrefix); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		Exit(2)
		return nil // Won't reach in normal execution
	}

	// Resolve context
	ctx, _ := context.Resolve(GetActorName(), "")

	// Determine base directory - use current directory
	baseDir := ".stash"
	if ctx.StashDir != "" {
		baseDir = ctx.StashDir
	}

	// Create storage
	store, err := storage.NewStore(baseDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Create stash configuration
	now := time.Now()
	stash := &model.Stash{
		Name:      name,
		Prefix:    initPrefix,
		Created:   now,
		CreatedBy: ctx.Actor,
		Columns:   model.ColumnList{},
	}

	// Create stash
	if err := store.CreateStash(name, initPrefix, stash); err != nil {
		if errors.Is(err, model.ErrStashExists) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' already exists\n", name)
			Exit(1)
			return nil // Won't reach in normal execution
		}
		return fmt.Errorf("failed to create stash: %w", err)
	}

	// Create empty records.jsonl file
	stashDir := filepath.Join(baseDir, name)
	recordsPath := filepath.Join(stashDir, "records.jsonl")
	if _, err := os.Stat(recordsPath); os.IsNotExist(err) {
		f, err := os.Create(recordsPath)
		if err != nil {
			return fmt.Errorf("failed to create records.jsonl: %w", err)
		}
		f.Close()
	}

	// Create files/ subdirectory
	filesDir := filepath.Join(stashDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("failed to create files directory: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"name":       name,
			"prefix":     initPrefix,
			"created_at": now.Format(time.RFC3339),
			"created_by": ctx.Actor,
			"path":       stashDir,
			"daemon":     !NoDaemon(),
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Created stash '%s' with prefix '%s'\n", name, initPrefix)
		if IsVerbose() {
			fmt.Printf("  path: %s\n", stashDir)
			fmt.Printf("  actor: %s\n", ctx.Actor)
			fmt.Printf("  branch: %s\n", ctx.Branch)
		}
	}

	// TODO: Start daemon unless --no-daemon is specified
	// For now, daemon functionality is not implemented

	return nil
}
