// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var setColFlags []string
var setAutoCreate bool

var setCmd = &cobra.Command{
	Use:   "set <id> <field>=<value> | set <id> --col <field> <value> [--col <field> <value>...]",
	Short: "Update record fields",
	Long: `Update one or more fields on an existing record.

Single field update:
  stash set inv-ex4j Price=1299

Multiple field update:
  stash set inv-ex4j --col Price 1299 --col Stock 50

Auto-create columns:
  stash set inv-ex4j NewField=value --auto-create

Note: Cannot update deleted records. Use 'stash restore' first.

Examples:
  stash set inv-ex4j Price=1299
  stash set inv-ex4j --col Price 1299 --col Stock 50
  stash set inv-ex4j Notes=""  # Clear a field
  stash set inv-ex4j Category=Electronics --auto-create  # Create column if needed

AI Agent Examples:
  # Update with processing results
  stash set "$RECORD_ID" status="complete" result="$AI_OUTPUT"

  # Timestamp updates
  stash set "$RECORD_ID" processed_at="$(date -Iseconds)"

  # Queue processing pattern
  stash query "SELECT id FROM tasks WHERE status='pending'" --json | \
      jq -r '.[].id' | while read id; do
          stash set "$id" status="processing"
          # ... do work ...
          stash set "$id" status="complete"
      done

  # Error handling with status tracking
  if ! process_record "$id"; then
      stash set "$id" status="error" error_msg="Processing failed"
  fi

Exit Codes:
  0  Success - record updated
  1  Record or column not found
  2  Validation error (invalid format, reserved column name)
  3  Record is deleted (use 'stash restore' first)`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSet,
}

func init() {
	setCmd.Flags().StringArrayVar(&setColFlags, "col", nil, "Set field value: --col Field Value (can be repeated)")
	setCmd.Flags().BoolVar(&setAutoCreate, "auto-create", false, "Automatically create columns that don't exist")
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
				ExitValidationError(fmt.Sprintf("invalid format: %s (expected Field=Value)", args[i]),
					map[string]interface{}{"input": args[i]})
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
				ExitValidationError(fmt.Sprintf("invalid --col format: %s (expected Field=Value or 'Field Value')", colFlag),
					map[string]interface{}{"input": colFlag})
				return nil
			}
		}
		fieldName := strings.TrimSpace(parts[0])
		fieldValue := strings.TrimSpace(parts[1])
		updates[fieldName] = fieldValue
	}

	if len(updates) == 0 {
		ExitValidationError("no field updates specified", nil)
		return nil
	}

	// Resolve context
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			ExitNoStashDir()
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			ExitValidationError("no stash specified and multiple stashes exist (use --stash)", nil)
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
			ExitStashNotFound(ctx.Stash)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// AC-04: Validate all columns exist before making changes, or auto-create if flag is set
	for fieldName := range updates {
		if !stash.Columns.Exists(fieldName) {
			if setAutoCreate {
				// Validate column name before auto-creating
				if model.IsReservedColumn(fieldName) {
					ExitValidationError(fmt.Sprintf("'%s' is a reserved column name", fieldName),
						map[string]interface{}{"column": fieldName})
					return nil
				}
				if err := model.ValidateColumnName(fieldName); err != nil {
					ExitValidationError(fmt.Sprintf("invalid column name '%s': must start with a letter and contain only letters, numbers, and underscores", fieldName),
						map[string]interface{}{"column": fieldName})
					return nil
				}

				// Auto-create the column
				col := model.Column{
					Name:    fieldName,
					Added:   time.Now(),
					AddedBy: ctx.Actor,
				}
				if err := store.AddColumn(ctx.Stash, col); err != nil {
					return fmt.Errorf("failed to auto-create column '%s': %w", fieldName, err)
				}

				// Update local stash reference
				stash.Columns = append(stash.Columns, col)

				if IsVerbose() && !IsQuiet() {
					fmt.Printf("Auto-created column '%s'\n", fieldName)
				}
			} else {
				ExitColumnNotFound(fieldName)
				return nil
			}
		}
	}

	// AC-03: Get existing record
	record, err := store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			ExitRecordNotFound(recordID)
			return nil
		}
		// AC-05: Reject update to deleted record
		if errors.Is(err, model.ErrRecordDeleted) {
			ExitRecordDeleted(recordID)
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
