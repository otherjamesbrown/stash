// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	rmCascade bool
	rmYes     bool
)

var rmCmd = &cobra.Command{
	Use:     "rm <id>",
	Aliases: []string{"delete", "remove"},
	Short:   "Soft-delete a record",
	Long: `Soft-delete a record by setting _deleted_at and _deleted_by fields.

The record remains in the database but is excluded from normal queries.
Use 'stash restore' to undo a soft-delete.
Use 'stash purge' to permanently remove soft-deleted records.

Examples:
  stash rm inv-ex4j
  stash rm inv-ex4j --yes         # Skip confirmation
  stash rm inv-ex4j --cascade     # Delete parent and children
  stash rm inv-ex4j --json        # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: runRm,
}

func init() {
	rmCmd.Flags().BoolVar(&rmCascade, "cascade", false, "Delete parent and all children")
	rmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	// Resolve context
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			fmt.Fprintln(os.Stderr, "Error: no .stash directory found")
			Exit(1)
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			fmt.Fprintln(os.Stderr, "Error: no stash specified and multiple stashes exist (use --stash)")
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get stash configuration
	_, err = store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Get record to verify it exists and is not already deleted
	record, err := store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
		if errors.Is(err, model.ErrRecordDeleted) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is already deleted\n", recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// Check for children (AC-03)
	children, err := store.GetChildren(ctx.Stash, recordID)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	if len(children) > 0 && !rmCascade {
		fmt.Fprintf(os.Stderr, "Error: record '%s' has %d child record(s)\n", recordID, len(children))
		fmt.Fprintln(os.Stderr, "Hint: use --cascade to delete parent and children together")
		Exit(1)
		return nil
	}

	// Build list of records to delete
	toDelete := []*model.Record{record}
	if rmCascade && len(children) > 0 {
		toDelete = append(toDelete, children...)
		// Get nested children recursively
		toDelete, err = collectAllChildren(store, ctx.Stash, toDelete, children)
		if err != nil {
			return fmt.Errorf("failed to collect children: %w", err)
		}
	}

	// Confirmation (AC-04)
	if !rmYes && !IsQuiet() {
		fmt.Printf("Delete %d record(s)? [y/N]: ", len(toDelete))
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			Exit(1)
			return nil
		}
	}

	// Delete records
	var deletedRecords []*model.Record
	for _, rec := range toDelete {
		if err := store.DeleteRecord(ctx.Stash, rec.ID, ctx.Actor); err != nil {
			if errors.Is(err, model.ErrRecordDeleted) {
				// Already deleted, skip
				continue
			}
			return fmt.Errorf("failed to delete record %s: %w", rec.ID, err)
		}
		deletedRecords = append(deletedRecords, rec)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"deleted": len(deletedRecords),
			"ids":     getRecordIDs(deletedRecords),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(deletedRecords) == 1 {
			fmt.Printf("Deleted %s\n", recordID)
		} else {
			fmt.Printf("Deleted %d record(s)\n", len(deletedRecords))
		}
		if IsVerbose() {
			for _, rec := range deletedRecords {
				fmt.Printf("  - %s\n", rec.ID)
			}
		}
	}

	return nil
}

// collectAllChildren recursively collects all children of the given records.
func collectAllChildren(store *storage.Store, stashName string, collected []*model.Record, parents []*model.Record) ([]*model.Record, error) {
	for _, parent := range parents {
		children, err := store.GetChildren(stashName, parent.ID)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 {
			collected = append(collected, children...)
			collected, err = collectAllChildren(store, stashName, collected, children)
			if err != nil {
				return nil, err
			}
		}
	}
	return collected, nil
}

// getRecordIDs extracts IDs from a list of records.
func getRecordIDs(records []*model.Record) []string {
	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}
	return ids
}
