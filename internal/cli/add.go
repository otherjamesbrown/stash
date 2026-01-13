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

var (
	addSetFlags  []string
	addParentID  string
)

var addCmd = &cobra.Command{
	Use:   "add <value>",
	Short: "Add a new record",
	Long: `Add a new record to the current stash.

Requires at least one column to be defined first:
  stash column add Name Price Category

The value is assigned to the first (primary) column. Additional fields
can be set using --set flags. Column names use underscores, not hyphens.

Records get a unique ID based on the stash prefix (e.g., inv-ex4j).
Child records can be created with --parent, getting IDs like inv-ex4j.1.

Examples:
  stash add "Laptop"
  stash add "Laptop" --set Price=999 --set Category="electronics"
  stash add "Charger" --parent inv-ex4j

AI Agent Examples:
  # Capture new record ID for subsequent operations
  ID=$(stash add "New Item" --set status="pending")
  stash set "$ID" processed_at="$(date -Iseconds)"

  # Add with JSON output for parsing
  stash add "New Item" --json | jq -r '._id'

  # Batch add from external data
  cat items.json | jq -r '.[] | @base64' | while read item; do
      name=$(echo "$item" | base64 -d | jq -r '.name')
      price=$(echo "$item" | base64 -d | jq -r '.price')
      stash add "$name" --set Price="$price"
  done

Exit Codes:
  0  Success - record created
  1  Stash or column not found
  2  Validation error (empty value, invalid field format)
  4  Parent record not found (with --parent)`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringArrayVar(&addSetFlags, "set", nil, "Set field value (can be repeated): --set Field=Value")
	addCmd.Flags().StringVar(&addParentID, "parent", "", "Parent record ID for creating child records")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	primaryValue := strings.TrimSpace(args[0])

	// AC-06: Reject empty primary value
	if primaryValue == "" {
		ExitValidationError("primary value cannot be empty", nil)
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

	// Must have at least one column
	if !stash.HasColumns() {
		ExitValidationError("cannot add record - stash has no columns defined (use 'stash column add <name>' first)",
			map[string]interface{}{"stash": ctx.Stash})
		return nil
	}

	// Parse --set flags into field map
	fields := make(map[string]interface{})

	// Set primary value to first column (AC-07: trimmed)
	primaryCol := stash.PrimaryColumn()
	fields[primaryCol.Name] = primaryValue

	// Parse additional --set flags
	for _, setFlag := range addSetFlags {
		parts := strings.SplitN(setFlag, "=", 2)
		if len(parts) != 2 {
			ExitValidationError(fmt.Sprintf("invalid --set format: %s (expected Field=Value)", setFlag),
				map[string]interface{}{"input": setFlag})
			return nil
		}
		fieldName := strings.TrimSpace(parts[0])
		fieldValue := strings.TrimSpace(parts[1])

		// Validate column exists
		if !stash.Columns.Exists(fieldName) {
			ExitColumnNotFound(fieldName)
			return nil
		}

		fields[fieldName] = fieldValue
	}

	// Validate fields against column constraints
	validationResult := ValidateFields(stash, fields)
	if !validationResult.Valid {
		// Report first validation error
		if len(validationResult.Errors) > 0 {
			err := validationResult.Errors[0]
			ExitValidationError(err.Message,
				map[string]interface{}{
					"column": err.Column,
					"value":  err.Value,
					"rule":   err.Rule,
				})
			return nil
		}
		ExitValidationError("validation failed", nil)
		return nil
	}

	// Handle parent ID for child records (AC-03, AC-04)
	var recordID string
	var parentID string
	if addParentID != "" {
		// Validate parent exists
		_, err := store.GetRecord(ctx.Stash, addParentID)
		if err != nil {
			if errors.Is(err, model.ErrRecordNotFound) || errors.Is(err, model.ErrRecordDeleted) {
				ExitReferenceError(fmt.Sprintf("parent record '%s' not found", addParentID),
					map[string]interface{}{"parent_id": addParentID})
				return nil
			}
			return fmt.Errorf("failed to get parent record: %w", err)
		}

		// Generate child ID
		nextSeq, err := store.GetNextChildSeq(ctx.Stash, addParentID)
		if err != nil {
			return fmt.Errorf("failed to get next child sequence: %w", err)
		}
		recordID = model.GenerateChildID(addParentID, nextSeq)
		parentID = addParentID
	} else {
		// Generate new root ID
		recordID, err = model.GenerateID(stash.Prefix)
		if err != nil {
			return fmt.Errorf("failed to generate ID: %w", err)
		}
	}

	// Create record
	now := time.Now()
	record := &model.Record{
		ID:        recordID,
		ParentID:  parentID,
		CreatedAt: now,
		CreatedBy: ctx.Actor,
		UpdatedAt: now,
		UpdatedBy: ctx.Actor,
		Branch:    ctx.Branch,
		Fields:    fields,
	}

	// Save record
	if err := store.CreateRecord(ctx.Stash, record); err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		// AC-05: JSON output format
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		// AC-01: ID is output to stdout
		fmt.Println(recordID)
		if IsVerbose() {
			fmt.Printf("  hash: %s\n", record.Hash)
			fmt.Printf("  created_by: %s\n", record.CreatedBy)
			fmt.Printf("  branch: %s\n", record.Branch)
		}
	}

	return nil
}
