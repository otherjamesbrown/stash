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

var setColFlags []string

var setCmd = &cobra.Command{
	Use:   "set <id> <field>=<value> | set <id> --col <field> <value> [--col <field> <value>...]",
	Short: "Update record fields",
	Long: `Update one or more fields on an existing record.

Single field update:
  stash set inv-ex4j Price=1299

Multiple field update:
  stash set inv-ex4j --col Price 1299 --col Stock 50

Note: Cannot update deleted records. Use 'stash restore' first.

Examples:
  stash set inv-ex4j Price=1299
  stash set inv-ex4j --col Price 1299 --col Stock 50
  stash set inv-ex4j Notes=""  # Clear a field`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSet,
}

func init() {
	setCmd.Flags().StringArrayVar(&setColFlags, "col", nil, "Set field value: --col Field Value (can be repeated)")
	rootCmd.AddCommand(setCmd)
}

func runSet(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	// Parse field updates
	updates := make(map[string]interface{})

	// Parse from positional args (Field=Value format)
	if len(args) > 1 && len(setColFlags) == 0 {
		// Single field update: stash set inv-ex4j Field=Value
		for i := 1; i < len(args); i++ {
			parts := strings.SplitN(args[i], "=", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Error: invalid format: %s (expected Field=Value)\n", args[i])
				Exit(2)
				return nil
			}
			fieldName := strings.TrimSpace(parts[0])
			fieldValue := strings.TrimSpace(parts[1])
			updates[fieldName] = fieldValue
		}
	}

	// Parse --col flags: each flag is "Field Value" but we get them as "Field" and "Value" separately
	// Actually, StringArrayVar gives us the whole value after --col
	// So --col "Price 1299" gives us "Price 1299"
	// But typical usage is --col Price 1299 which gives us "Price" as the value
	// We need to handle this by expecting pairs

	// Correction: with --col Price 1299, cobra would see --col with value "Price"
	// and then "1299" as a positional arg. This is tricky.

	// Let's use a simpler approach: --col Field=Value
	for _, colFlag := range setColFlags {
		parts := strings.SplitN(colFlag, "=", 2)
		if len(parts) != 2 {
			// Try space-separated format: "Field Value"
			parts = strings.SplitN(colFlag, " ", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Error: invalid --col format: %s (expected Field=Value or 'Field Value')\n", colFlag)
				Exit(2)
				return nil
			}
		}
		fieldName := strings.TrimSpace(parts[0])
		fieldValue := strings.TrimSpace(parts[1])
		updates[fieldName] = fieldValue
	}

	if len(updates) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no field updates specified")
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
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// AC-04: Validate all columns exist before making changes
	for fieldName := range updates {
		if !stash.Columns.Exists(fieldName) {
			fmt.Fprintf(os.Stderr, "Error: column '%s' not found\n", fieldName)
			Exit(1)
			return nil
		}
	}

	// AC-03: Get existing record
	record, err := store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
		// AC-05: Reject update to deleted record
		if errors.Is(err, model.ErrRecordDeleted) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is deleted\n", recordID)
			fmt.Fprintf(os.Stderr, "Hint: use `stash restore %s` first\n", recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

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
		return fmt.Errorf("failed to update record: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Updated %s\n", recordID)
		if IsVerbose() {
			fmt.Printf("  hash: %s\n", record.Hash)
			fmt.Printf("  updated_by: %s\n", record.UpdatedBy)
		}
	}

	return nil
}
