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

var queryCmd = &cobra.Command{
	Use:   "query <sql>",
	Short: "Run a raw SQL query against the cache",
	Long: `Execute a SELECT query against the SQLite cache.

Only SELECT statements are allowed. This provides direct access to the
SQLite cache for complex queries, aggregations, and joins.

The table name is the stash name (with hyphens replaced by underscores).

Examples:
  stash query "SELECT Name, Price FROM inventory WHERE Price > 100"
  stash query "SELECT Category, COUNT(*) FROM inventory GROUP BY Category"
  stash query "SELECT * FROM inventory ORDER BY updated_at DESC LIMIT 10"
  stash query "SELECT * FROM inventory" --json

Note: This queries the SQLite cache, not the JSONL source. For most use
cases, the cache is up-to-date, but after manual JSONL edits, run
'stash repair' to rebuild the cache.`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

func init() {
	rootCmd.AddCommand(queryCmd)
}

// isSelectQuery checks if the query is a SELECT statement (read-only).
func isSelectQuery(query string) bool {
	// Normalize query: trim whitespace and convert to uppercase for checking
	normalized := strings.TrimSpace(strings.ToUpper(query))

	// Must start with SELECT
	if !strings.HasPrefix(normalized, "SELECT") {
		return false
	}

	// Reject queries that contain modification keywords
	// (Even in subqueries, these should not be allowed)
	dangerousKeywords := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
		"TRUNCATE", "REPLACE", "ATTACH", "DETACH",
	}

	for _, keyword := range dangerousKeywords {
		// Check for keyword as a whole word (not part of identifier)
		if strings.Contains(" "+normalized+" ", " "+keyword+" ") {
			return false
		}
	}

	return true
}

func runQuery(cmd *cobra.Command, args []string) error {
	query := args[0]

	// AC-02: Reject non-SELECT queries
	if !isSelectQuery(query) {
		fmt.Fprintln(os.Stderr, "Error: only SELECT queries are allowed")
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

	// Execute query
	rows, columns, err := store.RawQuery(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: query failed: %v\n", err)
		Exit(3)
		return nil
	}

	// AC-03: JSON output
	if GetJSONOutput() {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// AC-01, AC-04: Human-readable output
	if len(rows) == 0 {
		fmt.Println("No results.")
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Cap column widths
	for i := range widths {
		if widths[i] > 40 {
			widths[i] = 40
		}
	}

	// Print header
	headerParts := make([]string, len(columns))
	separatorParts := make([]string, len(columns))
	for i, col := range columns {
		headerParts[i] = fmt.Sprintf("%-*s", widths[i], col)
		separatorParts[i] = strings.Repeat("-", widths[i])
	}
	fmt.Println(strings.Join(headerParts, "  "))
	fmt.Println(strings.Join(separatorParts, "  "))

	// Print rows
	for _, row := range rows {
		rowParts := make([]string, len(columns))
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > widths[i] {
				val = val[:widths[i]-3] + "..."
			}
			rowParts[i] = fmt.Sprintf("%-*s", widths[i], val)
		}
		fmt.Println(strings.Join(rowParts, "  "))
	}

	fmt.Printf("\n%d row(s)\n", len(rows))

	return nil
}
