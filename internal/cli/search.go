// Package cli provides the command-line interface for stash.
package cli

import (
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

var (
	searchIn []string // Columns to search in
)

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search for records matching a term",
	Long: `Search for records containing a term across text fields.

By default, searches all text columns. Use --in to limit search to specific columns.

The search is case-insensitive and matches partial strings (contains).

Examples:
  stash search "disney"                    # Search all columns
  stash search "disney" --in company_name  # Search only company_name column
  stash search "disney" --in Name --in Description  # Search multiple columns
  stash search "disney" --json             # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringArrayVar(&searchIn, "in", nil, "Column(s) to search in (can be repeated)")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	searchTerm := args[0]

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

	// Build list options for search
	opts := storage.ListOptions{
		IncludeDeleted: false,
		ParentID:       "*", // Search all records, not just root
	}

	var records []*model.Record

	if len(searchIn) > 0 {
		// Search in specific columns: fetch all records then filter in memory
		// This is needed because the storage layer doesn't support OR conditions
		allRecords, err := store.ListRecords(ctx.Stash, opts)
		if err != nil {
			return fmt.Errorf("failed to list records: %w", err)
		}
		records = filterRecordsBySearch(allRecords, searchTerm, searchIn)
	} else {
		// Search all columns using the built-in Search option
		opts.Search = searchTerm
		var err error
		records, err = store.ListRecords(ctx.Stash, opts)
		if err != nil {
			return fmt.Errorf("failed to search records: %w", err)
		}
	}

	// JSON output
	if GetJSONOutput() {
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(records) == 0 {
		fmt.Println("No records found.")
		return nil
	}

	// Determine which columns to display
	var displayColumns []string
	primaryCol := stash.PrimaryColumn()
	if primaryCol != nil {
		displayColumns = []string{primaryCol.Name}
	}

	// Calculate column widths
	idWidth := 4 // "ID" header
	colWidths := make(map[string]int)
	for _, col := range displayColumns {
		colWidths[col] = len(col)
	}
	statusWidth := 6 // "Status" header

	for _, rec := range records {
		if len(rec.ID) > idWidth {
			idWidth = len(rec.ID)
		}
		for _, col := range displayColumns {
			if val, ok := rec.Fields[col]; ok {
				s := fmt.Sprintf("%v", val)
				if len(s) > colWidths[col] {
					colWidths[col] = len(s)
				}
			}
		}
	}

	// Cap column widths for readability
	if idWidth > 20 {
		idWidth = 20
	}
	for col := range colWidths {
		if colWidths[col] > 40 {
			colWidths[col] = 40
		}
	}

	// Print header
	headerParts := []string{fmt.Sprintf("%-*s", idWidth, "ID")}
	separatorParts := []string{strings.Repeat("-", idWidth)}
	for _, col := range displayColumns {
		headerParts = append(headerParts, fmt.Sprintf("%-*s", colWidths[col], col))
		separatorParts = append(separatorParts, strings.Repeat("-", colWidths[col]))
	}
	headerParts = append(headerParts, fmt.Sprintf("%-*s", statusWidth, "Status"))
	separatorParts = append(separatorParts, strings.Repeat("-", statusWidth))
	headerParts = append(headerParts, "Updated")
	separatorParts = append(separatorParts, strings.Repeat("-", 19))

	fmt.Println(strings.Join(headerParts, "  "))
	fmt.Println(strings.Join(separatorParts, "  "))

	// Print records
	for _, rec := range records {
		// Format ID (truncate if needed)
		id := rec.ID
		if len(id) > idWidth {
			id = id[:idWidth-3] + "..."
		}

		rowParts := []string{fmt.Sprintf("%-*s", idWidth, id)}

		// Format column values
		for _, col := range displayColumns {
			val := ""
			if v, ok := rec.Fields[col]; ok {
				val = fmt.Sprintf("%v", v)
				if len(val) > colWidths[col] {
					val = val[:colWidths[col]-3] + "..."
				}
			}
			rowParts = append(rowParts, fmt.Sprintf("%-*s", colWidths[col], val))
		}

		// Format status
		status := "active"
		if rec.IsDeleted() {
			status = "deleted"
		}
		rowParts = append(rowParts, fmt.Sprintf("%-*s", statusWidth, status))

		// Format updated time
		updated := rec.UpdatedAt.Format("2006-01-02 15:04:05")
		rowParts = append(rowParts, updated)

		fmt.Println(strings.Join(rowParts, "  "))
	}

	// Print count
	fmt.Printf("\nTotal: %d record(s)\n", len(records))

	return nil
}

// filterRecordsBySearch filters records by search term in specific columns.
// This performs case-insensitive partial matching (contains).
func filterRecordsBySearch(records []*model.Record, term string, columns []string) []*model.Record {
	termLower := strings.ToLower(term)
	var filtered []*model.Record

	for _, rec := range records {
		for _, col := range columns {
			if val, ok := rec.Fields[col]; ok {
				valStr := fmt.Sprintf("%v", val)
				if strings.Contains(strings.ToLower(valStr), termLower) {
					filtered = append(filtered, rec)
					break // Found match, no need to check other columns
				}
			}
		}
	}

	return filtered
}
