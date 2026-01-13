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
	columnDesc     string
	columnValidate string
	columnEnum     string
	columnRequired bool
)

var columnCmd = &cobra.Command{
	Use:     "column",
	Aliases: []string{"col"},
	Short:   "Manage stash columns",
	Long: `Manage columns in a stash schema.

Columns define the structure of records and can have descriptions
to help agents understand their purpose.

Examples:
  stash column add Name
  stash column add Name Price Category
  stash column add Price --desc "Price in USD"
  stash column list
  stash column list --json
  stash column describe Price "Price in USD"`,
}

var columnAddCmd = &cobra.Command{
	Use:   "add <name> [name...]",
	Short: "Add one or more columns to the stash",
	Long: `Add one or more columns to the stash schema.

Column names must:
  - Start with a letter
  - Contain only letters, numbers, and underscores
  - Be at most 64 characters
  - Not be a reserved name (_id, _hash, etc.)

The first column added becomes the "primary" column used for
the default value in 'stash add' commands.

Validation Options:
  --validate TYPE  Validate format: email, url, number, date
  --enum VALUES    Comma-separated list of allowed values
  --required       Field must have a non-empty value

Examples:
  stash column add Name
  stash column add Name Price Category
  stash column add Price --desc "Price in USD"
  stash column add email --validate email
  stash column add status --enum "pending,active,closed"
  stash column add priority --required

AI Agent Examples:
  # Add email column with validation
  stash column add contact_email --validate email --desc "Primary contact email"

  # Add status column with enum constraint
  stash column add status --enum "pending,active,closed" --required

  # Check column constraints
  stash column list --json | jq '.[] | select(.validate != null)'

Exit Codes:
  0  Success - column added
  1  Stash not found, column already exists
  2  Validation error (invalid column name, invalid validation type)

JSON Output (--json):
  [{"name": "email", "validate": "email", "required": false}]
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runColumnAdd,
}

var columnListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all columns in the stash",
	Long: `List all columns in the stash with their descriptions
and population statistics.

Examples:
  stash column list
  stash column list --json`,
	Args: cobra.NoArgs,
	RunE: runColumnList,
}

var columnDescribeCmd = &cobra.Command{
	Use:   "describe <name> <description>",
	Short: "Set or update a column description",
	Long: `Set or update the description for a column.

Descriptions help agents understand the purpose and format
of each column.

Examples:
  stash column describe Price "Price in USD"
  stash column describe Name "Product display name"`,
	Args: cobra.ExactArgs(2),
	RunE: runColumnDescribe,
}

func init() {
	columnAddCmd.Flags().StringVar(&columnDesc, "desc", "", "Column description")
	columnAddCmd.Flags().StringVar(&columnValidate, "validate", "", "Validation type: email, url, number, date")
	columnAddCmd.Flags().StringVar(&columnEnum, "enum", "", "Comma-separated list of allowed values")
	columnAddCmd.Flags().BoolVar(&columnRequired, "required", false, "Field is required (non-empty)")

	columnCmd.AddCommand(columnAddCmd)
	columnCmd.AddCommand(columnListCmd)
	columnCmd.AddCommand(columnDescribeCmd)
	rootCmd.AddCommand(columnCmd)
}

func runColumnAdd(cmd *cobra.Command, args []string) error {
	// Resolve context - stash is required
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			fmt.Fprintln(os.Stderr, "Error: no stash found (run 'stash init' first)")
			Exit(1)
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			fmt.Fprintln(os.Stderr, "Error: multiple stashes exist, use --stash to specify")
			Exit(1)
			return nil
		}
		return err
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get stash to verify it exists
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Track added columns for output
	var addedColumns []model.Column
	now := time.Now()

	// If any constraint flags are provided, only one column name is allowed
	hasConstraints := columnDesc != "" || columnValidate != "" || columnEnum != "" || columnRequired
	if hasConstraints && len(args) > 1 {
		fmt.Fprintln(os.Stderr, "Error: --desc, --validate, --enum, and --required can only be used when adding a single column")
		Exit(2)
		return nil
	}

	// Validate the --validate flag value
	if columnValidate != "" && !IsValidValidationType(columnValidate) {
		fmt.Fprintf(os.Stderr, "Error: invalid validation type '%s' (valid types: %s)\n",
			columnValidate, strings.Join(ValidValidationTypes, ", "))
		Exit(2)
		return nil
	}

	// Parse enum values
	var enumValues []string
	if columnEnum != "" {
		for _, v := range strings.Split(columnEnum, ",") {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				enumValues = append(enumValues, trimmed)
			}
		}
		if len(enumValues) == 0 {
			fmt.Fprintln(os.Stderr, "Error: --enum requires at least one non-empty value")
			Exit(2)
			return nil
		}
	}

	// Add each column
	for _, name := range args {
		// Validate column name first (for better error messages)
		if model.IsReservedColumn(name) {
			fmt.Fprintf(os.Stderr, "Error: '%s' is a reserved column name\n", name)
			Exit(2)
			return nil
		}

		if err := model.ValidateColumnName(name); err != nil {
			if errors.Is(err, model.ErrReservedColumn) {
				fmt.Fprintf(os.Stderr, "Error: '%s' is a reserved column name\n", name)
			} else {
				fmt.Fprintf(os.Stderr, "Error: invalid column name '%s': must start with a letter and contain only letters, numbers, and underscores\n", name)
			}
			Exit(2)
			return nil
		}

		// Check for duplicate (case-insensitive)
		if existing := stash.Columns.Find(name); existing != nil {
			fmt.Fprintf(os.Stderr, "Error: column '%s' already exists\n", existing.Name)
			Exit(1)
			return nil
		}

		col := model.Column{
			Name:     name,
			Desc:     columnDesc,
			Added:    now,
			AddedBy:  ctx.Actor,
			Validate: columnValidate,
			Enum:     enumValues,
			Required: columnRequired,
		}

		if err := store.AddColumn(ctx.Stash, col); err != nil {
			if errors.Is(err, model.ErrColumnExists) {
				// Find the existing column name to show original case
				existing := stash.Columns.Find(name)
				if existing != nil {
					fmt.Fprintf(os.Stderr, "Error: column '%s' already exists\n", existing.Name)
				} else {
					fmt.Fprintf(os.Stderr, "Error: column '%s' already exists\n", name)
				}
				Exit(1)
				return nil
			}
			return fmt.Errorf("failed to add column '%s': %w", name, err)
		}

		addedColumns = append(addedColumns, col)
		// Update local stash reference to track added columns for subsequent checks
		stash.Columns = append(stash.Columns, col)
	}

	// Output result
	if GetJSONOutput() {
		output := make([]map[string]interface{}, len(addedColumns))
		for i, col := range addedColumns {
			output[i] = map[string]interface{}{
				"name":     col.Name,
				"desc":     col.Desc,
				"added":    col.Added.Format(time.RFC3339),
				"added_by": col.AddedBy,
				"validate": col.Validate,
				"enum":     col.Enum,
				"required": col.Required,
			}
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(addedColumns) == 1 {
			fmt.Printf("Added column '%s' to stash '%s'\n", addedColumns[0].Name, ctx.Stash)
		} else {
			names := make([]string, len(addedColumns))
			for i, col := range addedColumns {
				names[i] = col.Name
			}
			fmt.Printf("Added %d columns to stash '%s'\n", len(addedColumns), ctx.Stash)
			if IsVerbose() {
				for _, col := range addedColumns {
					fmt.Printf("  %s\n", col.Name)
				}
			}
		}
	}

	// Reset flags for next call (important for tests)
	columnDesc = ""
	columnValidate = ""
	columnEnum = ""
	columnRequired = false

	return nil
}

// ColumnInfo represents column information for list output
type ColumnInfo struct {
	Name      string   `json:"name"`
	Desc      string   `json:"desc"`
	Validate  string   `json:"validate,omitempty"`
	Enum      []string `json:"enum,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Populated int      `json:"populated"`
	Empty     int      `json:"empty"`
}

func runColumnList(cmd *cobra.Command, args []string) error {
	// Resolve context - stash is required
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			fmt.Fprintln(os.Stderr, "Error: no stash found (run 'stash init' first)")
			Exit(1)
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			fmt.Fprintln(os.Stderr, "Error: multiple stashes exist, use --stash to specify")
			Exit(1)
			return nil
		}
		return err
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get stash
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Get all records to calculate population stats
	records, err := store.ListRecords(ctx.Stash, storage.ListOptions{
		ParentID:       "*",
		IncludeDeleted: false,
	})
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// Build column info with population stats
	columnInfos := make([]ColumnInfo, len(stash.Columns))
	for i, col := range stash.Columns {
		columnInfos[i] = ColumnInfo{
			Name:     col.Name,
			Desc:     col.Desc,
			Validate: col.Validate,
			Enum:     col.Enum,
			Required: col.Required,
		}

		// Count populated and empty
		for _, record := range records {
			if val, ok := record.Fields[col.Name]; ok && val != nil && val != "" {
				columnInfos[i].Populated++
			} else {
				columnInfos[i].Empty++
			}
		}
	}

	// Output result
	if GetJSONOutput() {
		data, _ := json.Marshal(columnInfos)
		fmt.Println(string(data))
	} else {
		if len(columnInfos) == 0 {
			fmt.Printf("No columns in stash '%s'\n", ctx.Stash)
		} else {
			fmt.Printf("Columns in stash '%s':\n", ctx.Stash)
			for _, info := range columnInfos {
				fmt.Printf("\n  %s\n", info.Name)
				if info.Desc != "" {
					fmt.Printf("    Description: %s\n", info.Desc)
				}
				if info.Validate != "" {
					fmt.Printf("    Validate: %s\n", info.Validate)
				}
				if len(info.Enum) > 0 {
					fmt.Printf("    Enum: %s\n", strings.Join(info.Enum, ", "))
				}
				if info.Required {
					fmt.Printf("    Required: yes\n")
				}
				if len(records) > 0 {
					fmt.Printf("    Populated: %d, Empty: %d\n", info.Populated, info.Empty)
				}
			}
		}
	}

	return nil
}

func runColumnDescribe(cmd *cobra.Command, args []string) error {
	columnName := args[0]
	description := args[1]

	// Resolve context - stash is required
	ctx, err := context.ResolveRequired(GetActorName(), GetStashName())
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			fmt.Fprintln(os.Stderr, "Error: no stash found (run 'stash init' first)")
			Exit(1)
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			fmt.Fprintln(os.Stderr, "Error: multiple stashes exist, use --stash to specify")
			Exit(1)
			return nil
		}
		return err
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get stash
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Find column (case-insensitive)
	col := stash.Columns.Find(columnName)
	if col == nil {
		fmt.Fprintf(os.Stderr, "Error: column '%s' not found\n", columnName)
		Exit(1)
		return nil
	}

	// Update description
	col.Desc = description

	// Save updated stash config
	if err := store.UpdateStashConfig(stash); err != nil {
		return fmt.Errorf("failed to update column description: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"name": col.Name,
			"desc": col.Desc,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Updated description for column '%s'\n", col.Name)
	}

	return nil
}
