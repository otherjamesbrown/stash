// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	moveParentID string
)

var moveCmd = &cobra.Command{
	Use:   "move <id>",
	Short: "Move a record to a new parent",
	Long: `Move a record (and all its descendants) to a new parent.

This changes the record's ID and all descendant IDs to reflect the new
hierarchy. The record's data is preserved.

Use --parent "" or --parent with no value to move to root level.

Examples:
  stash move inv-ex4j.1 --parent inv-ab12      # Move to new parent
  stash move inv-ex4j.1 --parent ""            # Move to root level
  stash move inv-ex4j.1 --parent "" --json     # JSON output`,
	Args: cobra.ExactArgs(1),
	RunE: runMove,
}

func init() {
	moveCmd.Flags().StringVar(&moveParentID, "parent", "", "New parent record ID (empty for root)")
	moveCmd.MarkFlagRequired("parent")
	rootCmd.AddCommand(moveCmd)
}

func runMove(cmd *cobra.Command, args []string) error {
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
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Get record to move
	record, err := store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
		if errors.Is(err, model.ErrRecordDeleted) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is deleted\n", recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// Validate new parent
	newParentID := moveParentID
	if newParentID != "" {
		// Check new parent exists
		_, err := store.GetRecord(ctx.Stash, newParentID)
		if err != nil {
			if errors.Is(err, model.ErrRecordNotFound) || errors.Is(err, model.ErrRecordDeleted) {
				fmt.Fprintf(os.Stderr, "Error: parent record '%s' not found\n", newParentID)
				Exit(4)
				return nil
			}
			return fmt.Errorf("failed to get parent record: %w", err)
		}

		// Cannot move to self
		if newParentID == recordID {
			fmt.Fprintln(os.Stderr, "Error: cannot move record to itself")
			Exit(1)
			return nil
		}

		// Cannot move to own descendant (would create cycle)
		if model.IsDescendantOf(newParentID, recordID) {
			fmt.Fprintln(os.Stderr, "Error: cannot move record to its own descendant (would create cycle)")
			Exit(1)
			return nil
		}
	}

	// Get all descendants
	allRecords := []*model.Record{record}
	descendants, err := collectAllDescendants(store, ctx.Stash, recordID)
	if err != nil {
		return fmt.Errorf("failed to collect descendants: %w", err)
	}
	allRecords = append(allRecords, descendants...)

	// Generate new ID for the moved record
	var newRecordID string
	if newParentID == "" {
		// Moving to root - generate new root ID
		newRecordID, err = model.GenerateID(stash.Prefix)
		if err != nil {
			return fmt.Errorf("failed to generate ID: %w", err)
		}
	} else {
		// Moving to new parent - get next child sequence
		nextSeq, err := store.GetNextChildSeq(ctx.Stash, newParentID)
		if err != nil {
			return fmt.Errorf("failed to get next child sequence: %w", err)
		}
		newRecordID = model.GenerateChildID(newParentID, nextSeq)
	}

	// Build ID mapping (old -> new)
	idMapping := make(map[string]string)
	idMapping[recordID] = newRecordID

	// Map descendant IDs
	for _, desc := range descendants {
		// Replace the old prefix with new prefix
		// e.g., if moving inv-ex4j.1 to inv-ab12.1
		// then inv-ex4j.1.2 becomes inv-ab12.1.2
		suffix := strings.TrimPrefix(desc.ID, recordID)
		newDescID := newRecordID + suffix
		idMapping[desc.ID] = newDescID
	}

	// Create new records with new IDs (soft-delete old ones)
	now := time.Now()
	movedRecords := make([]*model.Record, 0, len(allRecords))

	for _, oldRec := range allRecords {
		newID := idMapping[oldRec.ID]

		// Determine new parent ID
		var newParent string
		if oldRec.ID == recordID {
			newParent = newParentID
		} else {
			// For descendants, map their parent ID
			newParent = idMapping[oldRec.ParentID]
		}

		// Create new record with same data but new ID
		newRec := &model.Record{
			ID:        newID,
			ParentID:  newParent,
			Hash:      oldRec.Hash, // Will be recalculated on create
			CreatedAt: oldRec.CreatedAt,
			CreatedBy: oldRec.CreatedBy,
			UpdatedAt: now,
			UpdatedBy: ctx.Actor,
			Branch:    ctx.Branch,
			Fields:    oldRec.Fields,
		}

		// Create the new record
		if err := store.CreateRecord(ctx.Stash, newRec); err != nil {
			return fmt.Errorf("failed to create moved record %s: %w", newID, err)
		}

		movedRecords = append(movedRecords, newRec)

		// Soft-delete the old record
		if err := store.DeleteRecord(ctx.Stash, oldRec.ID, ctx.Actor); err != nil {
			// Try to continue - the move is more important
			if IsVerbose() {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete old record %s: %v\n", oldRec.ID, err)
			}
		}
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"old_id":    recordID,
			"new_id":    newRecordID,
			"parent_id": newParentID,
			"moved":     len(movedRecords),
		}

		// Include all ID mappings in verbose mode
		if IsVerbose() {
			result["id_mapping"] = idMapping
		}

		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("%s -> %s\n", recordID, newRecordID)
		if len(movedRecords) > 1 {
			fmt.Printf("Moved %d record(s)\n", len(movedRecords))
		}
		if IsVerbose() {
			for oldID, newID := range idMapping {
				if oldID != recordID {
					fmt.Printf("  %s -> %s\n", oldID, newID)
				}
			}
		}
	}

	return nil
}

// collectAllDescendants recursively collects all descendants of a record.
func collectAllDescendants(store *storage.Store, stashName string, parentID string) ([]*model.Record, error) {
	var descendants []*model.Record

	children, err := store.GetChildren(stashName, parentID)
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		descendants = append(descendants, child)
		// Recursively get descendants
		childDescendants, err := collectAllDescendants(store, stashName, child.ID)
		if err != nil {
			return nil, err
		}
		descendants = append(descendants, childDescendants...)
	}

	return descendants, nil
}
