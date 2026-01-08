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
	restoreCascade bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a soft-deleted record",
	Long: `Restore a soft-deleted record by clearing _deleted_at and _deleted_by fields.

The record becomes active again and will appear in normal queries.

Examples:
  stash restore inv-ex4j
  stash restore inv-ex4j --cascade  # Restore parent and deleted children
  stash restore inv-ex4j --json     # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreCascade, "cascade", false, "Restore parent and all deleted children")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
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

	// Check if record exists (including deleted)
	record, err := store.GetRecordIncludeDeleted(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// AC-03: Reject restore of active record
	if !record.IsDeleted() {
		fmt.Fprintf(os.Stderr, "Error: record '%s' is not deleted\n", recordID)
		Exit(1)
		return nil
	}

	// Build list of records to restore
	toRestore := []*model.Record{record}

	// AC-02: Cascade restore
	if restoreCascade {
		deletedChildren, err := store.GetChildrenIncludeDeleted(ctx.Stash, recordID)
		if err != nil {
			return fmt.Errorf("failed to get children: %w", err)
		}
		// Filter to only deleted children
		for _, child := range deletedChildren {
			if child.IsDeleted() {
				toRestore = append(toRestore, child)
			}
		}
		// Recursively get nested deleted children
		toRestore, err = collectDeletedChildren(store, ctx.Stash, toRestore, deletedChildren)
		if err != nil {
			return fmt.Errorf("failed to collect children: %w", err)
		}
	}

	// Restore records
	var restoredRecords []*model.Record
	for _, rec := range toRestore {
		if !rec.IsDeleted() {
			continue // Skip non-deleted records
		}
		if err := store.RestoreRecord(ctx.Stash, rec.ID, ctx.Actor); err != nil {
			return fmt.Errorf("failed to restore record %s: %w", rec.ID, err)
		}
		restoredRecords = append(restoredRecords, rec)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"restored": len(restoredRecords),
			"ids":      getRecordIDs(restoredRecords),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(restoredRecords) == 1 {
			fmt.Printf("Restored %s\n", recordID)
		} else {
			fmt.Printf("Restored %d record(s)\n", len(restoredRecords))
		}
		if IsVerbose() {
			for _, rec := range restoredRecords {
				fmt.Printf("  - %s\n", rec.ID)
			}
		}
	}

	return nil
}

// collectDeletedChildren recursively collects all deleted children of the given records.
func collectDeletedChildren(store *storage.Store, stashName string, collected []*model.Record, parents []*model.Record) ([]*model.Record, error) {
	for _, parent := range parents {
		children, err := store.GetChildrenIncludeDeleted(stashName, parent.ID)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if child.IsDeleted() {
				collected = append(collected, child)
			}
		}
		if len(children) > 0 {
			collected, err = collectDeletedChildren(store, stashName, collected, children)
			if err != nil {
				return nil, err
			}
		}
	}
	return collected, nil
}
