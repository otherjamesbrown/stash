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
	bulkSetWhere []string
	bulkSetSet   []string
)

var bulkSetCmd = &cobra.Command{
	Use:   "bulk-set --where CONDITION --set FIELD=VALUE",
	Short: "Update multiple records matching a condition",
	Long: `Update fields on all records matching the WHERE condition.

This command allows bulk updates to records based on a filter condition.
Only non-deleted records are updated.

The --where flag is required and specifies which records to update.
The --set flag is required and specifies which field(s) to update.

WHERE clause format (same as 'stash list --where'):
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

Multiple --where flags are ANDed together.
Multiple --set flags update multiple fields.

Examples:
  stash bulk-set --where "Category=electronics" --set Priority=high
  stash bulk-set --where "Status=pending" --set Status=complete --set ReviewedBy=admin
  stash bulk-set --where "Priority IS NULL" --set Priority=normal
  stash bulk-set --where "Category=electronics" --where "Price>100" --set Featured=yes`,
	Args: cobra.NoArgs,
	RunE: runBulkSet,
}

func init() {
	bulkSetCmd.Flags().StringArrayVar(&bulkSetWhere, "where", nil, "Filter condition (required, can be repeated)")
	bulkSetCmd.Flags().StringArrayVar(&bulkSetSet, "set", nil, "Field=Value to set (required, can be repeated)")
	rootCmd.AddCommand(bulkSetCmd)
}

func runBulkSet(cmd *cobra.Command, args []string) error {
	// Validate required flags
	if len(bulkSetWhere) == 0 {
		fmt.Fprintln(os.Stderr, "Error: --where flag is required")
		Exit(2)
		return nil
	}

	if len(bulkSetSet) == 0 {
		fmt.Fprintln(os.Stderr, "Error: --set flag is required")
		Exit(2)
		return nil
	}

	// Parse SET clauses
	updates := make(map[string]interface{})
	for _, setClause := range bulkSetSet {
		parts := strings.SplitN(setClause, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: invalid --set format: %s (expected Field=Value)\n", setClause)
			Exit(2)
			return nil
		}
		fieldName := strings.TrimSpace(parts[0])
		fieldValue := strings.TrimSpace(parts[1])
		updates[fieldName] = fieldValue
	}

	// Parse WHERE clauses
	var whereConditions []storage.WhereCondition
	for _, clause := range bulkSetWhere {
		cond, err := parseWhereClause(clause)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			Exit(2)
			return nil
		}
		whereConditions = append(whereConditions, cond)
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
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Validate all columns exist before making changes
	for fieldName := range updates {
		if !stash.Columns.Exists(fieldName) {
			fmt.Fprintf(os.Stderr, "Error: column '%s' not found\n", fieldName)
			Exit(1)
			return nil
		}
	}

	// Query matching records (non-deleted only)
	opts := storage.ListOptions{
		ParentID:       "*", // All records
		IncludeDeleted: false,
		Where:          whereConditions,
	}

	records, err := store.ListRecords(ctx.Stash, opts)
	if err != nil {
		return fmt.Errorf("failed to query records: %w", err)
	}

	// Update each matching record
	var updatedIDs []string
	for _, record := range records {
		// Apply updates to fields
		for fieldName, fieldValue := range updates {
			// Use the column's actual name case
			col := stash.Columns.Find(fieldName)
			if col != nil {
				record.SetField(col.Name, fieldValue)
			}
		}

		// Update audit trail
		record.UpdatedAt = time.Now()
		record.UpdatedBy = ctx.Actor

		// Save record
		if err := store.UpdateRecord(ctx.Stash, record); err != nil {
			return fmt.Errorf("failed to update record %s: %w", record.ID, err)
		}

		updatedIDs = append(updatedIDs, record.ID)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"count":   len(updatedIDs),
			"updated": updatedIDs,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(updatedIDs) == 0 {
			fmt.Println("No records matched the condition.")
		} else if len(updatedIDs) == 1 {
			fmt.Printf("Updated 1 record: %s\n", updatedIDs[0])
		} else {
			fmt.Printf("Updated %d records\n", len(updatedIDs))
			if IsVerbose() {
				for _, id := range updatedIDs {
					fmt.Printf("  %s\n", id)
				}
			}
		}
	}

	return nil
}
