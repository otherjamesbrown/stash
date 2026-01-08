// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	purgeID     string
	purgeBefore string
	purgeAll    bool
	purgeDryRun bool
	purgeYes    bool
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Permanently delete soft-deleted records",
	Long: `Permanently remove soft-deleted records from the database.

This is irreversible! Records will be removed from JSONL and SQLite.
Associated files will also be deleted.

Use --dry-run to preview what would be deleted without making changes.

Examples:
  stash purge --id inv-ex4j --yes           # Purge specific record
  stash purge --before 30d --yes            # Purge records deleted > 30 days ago
  stash purge --all --yes                   # Purge all deleted records
  stash purge --before 7d --dry-run         # Preview what would be purged`,
	Args: cobra.NoArgs,
	RunE: runPurge,
}

func init() {
	purgeCmd.Flags().StringVar(&purgeID, "id", "", "Purge specific record by ID")
	purgeCmd.Flags().StringVar(&purgeBefore, "before", "", "Purge records deleted before duration (e.g., 30d, 7d, 24h)")
	purgeCmd.Flags().BoolVar(&purgeAll, "all", false, "Purge all deleted records")
	purgeCmd.Flags().BoolVar(&purgeDryRun, "dry-run", false, "Preview what would be purged without making changes")
	purgeCmd.Flags().BoolVarP(&purgeYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(purgeCmd)
}

func runPurge(cmd *cobra.Command, args []string) error {
	// Validate flags - need at least one selection criteria
	if purgeID == "" && purgeBefore == "" && !purgeAll {
		fmt.Fprintln(os.Stderr, "Error: specify --id, --before, or --all to select records to purge")
		Exit(2)
		return nil
	}

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

	// Build list of records to purge
	var toPurge []*model.Record

	if purgeID != "" {
		// AC-02: Purge specific record
		record, err := store.GetRecordIncludeDeleted(ctx.Stash, purgeID)
		if err != nil {
			if errors.Is(err, model.ErrRecordNotFound) {
				fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", purgeID)
				Exit(4)
				return nil
			}
			return fmt.Errorf("failed to get record: %w", err)
		}

		if !record.IsDeleted() {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is not deleted; cannot purge active records\n", purgeID)
			Exit(1)
			return nil
		}

		toPurge = append(toPurge, record)
	} else {
		// Parse --before duration
		var beforeTime *time.Time
		if purgeBefore != "" {
			duration, err := parsePurgeDuration(purgeBefore)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid duration '%s': %v\n", purgeBefore, err)
				Exit(2)
				return nil
			}
			t := time.Now().Add(-duration)
			beforeTime = &t
		}

		// AC-01: Get deleted records (optionally filtered by age)
		deleted, err := store.ListDeletedRecords(ctx.Stash, beforeTime)
		if err != nil {
			return fmt.Errorf("failed to list deleted records: %w", err)
		}
		toPurge = deleted
	}

	if len(toPurge) == 0 {
		if !IsQuiet() {
			fmt.Println("No deleted records found matching criteria.")
		}
		return nil
	}

	// AC-03: Dry run preview
	if purgeDryRun {
		if GetJSONOutput() {
			result := map[string]interface{}{
				"dry_run":     true,
				"would_purge": len(toPurge),
				"ids":         getRecordIDs(toPurge),
			}
			data, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("Would purge %d record(s):\n", len(toPurge))
			for _, rec := range toPurge {
				deletedAt := "unknown"
				if rec.DeletedAt != nil {
					deletedAt = rec.DeletedAt.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("  - %s (deleted: %s)\n", rec.ID, deletedAt)
			}
		}
		return nil
	}

	// AC-04: Confirmation
	if !purgeYes && !IsQuiet() {
		fmt.Printf("Permanently delete %d record(s)? This cannot be undone! [y/N]: ", len(toPurge))
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			Exit(1)
			return nil
		}
	}

	// Purge records
	var purgedRecords []*model.Record
	for _, rec := range toPurge {
		if err := store.PurgeRecord(ctx.Stash, rec.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to purge %s: %v\n", rec.ID, err)
			continue
		}
		purgedRecords = append(purgedRecords, rec)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"purged": len(purgedRecords),
			"ids":    getRecordIDs(purgedRecords),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(purgedRecords) == 1 {
			fmt.Printf("Purged %s\n", purgedRecords[0].ID)
		} else {
			fmt.Printf("Purged %d record(s)\n", len(purgedRecords))
		}
		if IsVerbose() {
			for _, rec := range purgedRecords {
				fmt.Printf("  - %s\n", rec.ID)
			}
		}
	}

	return nil
}

// parsePurgeDuration parses a duration string like "30d", "7d", "24h", "1h30m".
func parsePurgeDuration(s string) (time.Duration, error) {
	// Try standard Go duration format first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Handle day suffix (e.g., "30d")
	re := regexp.MustCompile(`^(\d+)d$`)
	if matches := re.FindStringSubmatch(s); len(matches) == 2 {
		days, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid duration format (use '30d', '7d', '24h', etc.)")
}
