package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var dropYes bool

var dropCmd = &cobra.Command{
	Use:   "drop <name>",
	Short: "Delete a stash and all its data",
	Long: `Permanently delete a stash and all its data.

This operation is destructive and cannot be undone.
All records, files, and configuration will be removed.

By default, you will be prompted for confirmation.
Use --yes to skip the confirmation prompt.

Examples:
  stash drop inventory
  stash drop inventory --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runDrop,
}

func init() {
	dropCmd.Flags().BoolVar(&dropYes, "yes", false, "Skip confirmation prompt")
	rootCmd.AddCommand(dropCmd)
}

func runDrop(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Resolve context to find stash directory
	ctx, _ := context.Resolve(GetActorName(), "")

	// Determine base directory
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

	// Check if stash exists
	stash, err := store.GetStash(name)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", name)
			Exit(3)
			return nil // Won't reach in normal execution
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Confirm deletion unless --yes is specified
	if !dropYes {
		fmt.Printf("Are you sure you want to delete stash '%s'? This cannot be undone. [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Drop the stash
	if err := store.DropStash(name); err != nil {
		return fmt.Errorf("failed to drop stash: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"name":    name,
			"prefix":  stash.Prefix,
			"deleted": true,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Deleted stash '%s'\n", name)
	}

	return nil
}
