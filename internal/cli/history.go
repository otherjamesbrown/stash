// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	historyBy    string
	historySince string
	historyLimit int
)

var historyCmd = &cobra.Command{
	Use:   "history [id]",
	Short: "Show change history",
	Long: `Display the change history for a stash or specific record.

Without an ID, shows all recent changes. With an ID, shows only changes
for that specific record.

Options:
  --by <actor>     Filter by actor (who made the change)
  --since <dur>    Filter by time (e.g., 24h, 7d, 1w)
  --limit <n>      Limit to N most recent changes

Examples:
  stash history                    # All recent changes
  stash history inv-ex4j           # Changes for specific record
  stash history --by alice         # Changes by alice
  stash history --since 24h        # Changes in last 24 hours
  stash history --limit 50         # Last 50 changes
  stash history --json             # JSON output`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHistory,
}

func init() {
	historyCmd.Flags().StringVar(&historyBy, "by", "", "Filter by actor")
	historyCmd.Flags().StringVar(&historySince, "since", "", "Filter by time (e.g., 24h, 7d)")
	historyCmd.Flags().IntVar(&historyLimit, "limit", 0, "Limit results (0 = no limit)")
	rootCmd.AddCommand(historyCmd)
}

// parseDuration parses duration strings like "24h", "7d", "1w"
func parseDuration(s string) (time.Duration, error) {
	// Handle week suffix
	if strings.HasSuffix(s, "w") {
		weeks := strings.TrimSuffix(s, "w")
		var n int
		if _, err := fmt.Sscanf(weeks, "%d", &n); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}

	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	// Otherwise, use standard Go duration parsing
	return time.ParseDuration(s)
}

func runHistory(cmd *cobra.Command, args []string) error {
	var recordID string
	if len(args) > 0 {
		recordID = args[0]
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

	// Verify stash exists
	_, err = store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Get history
	var history []*model.Record
	if recordID != "" {
		// AC-02: Show history for specific record
		// First verify record exists (in any state)
		history, err = store.GetRecordHistory(ctx.Stash, recordID)
		if err != nil {
			return fmt.Errorf("failed to get record history: %w", err)
		}
		if len(history) == 0 {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
	} else {
		// AC-01: Show all recent changes
		history, err = store.GetAllHistory(ctx.Stash)
		if err != nil {
			return fmt.Errorf("failed to get history: %w", err)
		}
	}

	// AC-03: Filter by actor
	if historyBy != "" {
		filtered := make([]*model.Record, 0)
		for _, rec := range history {
			if rec.UpdatedBy == historyBy || rec.CreatedBy == historyBy {
				filtered = append(filtered, rec)
			}
		}
		history = filtered
	}

	// AC-04: Filter by time
	if historySince != "" {
		duration, err := parseDuration(historySince)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid duration: %s\n", historySince)
			Exit(2)
			return nil
		}
		cutoff := time.Now().Add(-duration)
		filtered := make([]*model.Record, 0)
		for _, rec := range history {
			if rec.UpdatedAt.After(cutoff) {
				filtered = append(filtered, rec)
			}
		}
		history = filtered
	}

	// Sort by timestamp (most recent first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].UpdatedAt.After(history[j].UpdatedAt)
	})

	// AC-05: Limit results
	if historyLimit > 0 && len(history) > historyLimit {
		history = history[:historyLimit]
	}

	// AC-06: JSON output
	if GetJSONOutput() {
		// Build JSON output with relevant history fields
		output := make([]map[string]interface{}, len(history))
		for i, rec := range history {
			entry := map[string]interface{}{
				"_id":         rec.ID,
				"_op":         rec.Operation,
				"_updated_at": rec.UpdatedAt,
				"_updated_by": rec.UpdatedBy,
				"_hash":       rec.Hash,
			}
			if rec.Branch != "" {
				entry["_branch"] = rec.Branch
			}
			// Include primary field if available
			for k, v := range rec.Fields {
				entry[k] = v
			}
			output[i] = entry
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(history) == 0 {
		fmt.Println("No history found.")
		return nil
	}

	// Print header
	fmt.Printf("%-19s  %-8s  %-20s  %-15s  %s\n",
		"Timestamp", "Op", "ID", "Actor", "Branch")
	fmt.Printf("%s  %s  %s  %s  %s\n",
		strings.Repeat("-", 19),
		strings.Repeat("-", 8),
		strings.Repeat("-", 20),
		strings.Repeat("-", 15),
		strings.Repeat("-", 10),
	)

	// Print history entries
	for _, rec := range history {
		timestamp := rec.UpdatedAt.Format("2006-01-02 15:04:05")
		op := rec.Operation
		id := rec.ID
		if len(id) > 20 {
			id = id[:17] + "..."
		}
		actor := rec.UpdatedBy
		if len(actor) > 15 {
			actor = actor[:12] + "..."
		}
		branch := rec.Branch
		if len(branch) > 10 {
			branch = branch[:7] + "..."
		}

		fmt.Printf("%-19s  %-8s  %-20s  %-15s  %s\n",
			timestamp, op, id, actor, branch)
	}

	fmt.Printf("\n%d change(s)\n", len(history))

	return nil
}
