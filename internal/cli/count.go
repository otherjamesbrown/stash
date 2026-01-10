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
	countAll     bool
	countDeleted bool
	countWhere   []string
)

var countCmd = &cobra.Command{
	Use:   "count",
	Short: "Count records",
	Long: `Count records in the current stash.

By default, counts root-level non-deleted records. Use flags to filter:

  --all              Count all records including children
  --deleted          Include soft-deleted records
  --where CONDITION  Filter by field value (can be repeated)

WHERE clause format:
  field=value        Equals
  field!=value       Not equals
  field>value        Greater than (numeric)
  field<value        Less than (numeric)
  field>=value       Greater than or equal
  field<=value       Less than or equal
  field LIKE pattern Pattern match (use % for wildcard)
  field IS NULL      Field is null/unset
  field IS NOT NULL  Field has a value
  field IS EMPTY     Field is null or empty string
  field IS NOT EMPTY Field has a non-empty value

Examples:
  stash count
  stash count --all
  stash count --where "status=pending"
  stash count --where "notes IS EMPTY"
  stash count --json`,
	Args: cobra.NoArgs,
	RunE: runCount,
}

func init() {
	countCmd.Flags().BoolVar(&countAll, "all", false, "Count all records including children")
	countCmd.Flags().BoolVar(&countDeleted, "deleted", false, "Include soft-deleted records")
	countCmd.Flags().StringArrayVar(&countWhere, "where", nil, "Filter by field value (can be repeated)")
	rootCmd.AddCommand(countCmd)
}

func runCount(cmd *cobra.Command, args []string) error {
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

	// Parse WHERE clauses
	var whereConditions []storage.WhereCondition
	for _, clause := range countWhere {
		cond, err := parseWhereClause(clause)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			Exit(1)
			return nil
		}
		whereConditions = append(whereConditions, cond)
	}

	// Build list options
	opts := storage.ListOptions{
		IncludeDeleted: countDeleted,
		Where:          whereConditions,
	}

	// Handle parent filtering
	if countAll {
		opts.ParentID = "*" // All records
	} else {
		opts.ParentID = "" // Root records only
	}

	// List records and count
	records, err := store.ListRecords(ctx.Stash, opts)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	count := len(records)

	// JSON output
	if GetJSONOutput() {
		data, err := json.Marshal(map[string]int{"count": count})
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Plain output (just the number for scripting)
	fmt.Println(count)

	return nil
}
