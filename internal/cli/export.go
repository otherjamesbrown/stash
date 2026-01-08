// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/csv"
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
	exportFormat         string
	exportOutput         string
	exportWhere          []string
	exportIncludeDeleted bool
	exportForce          bool
)

var exportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export records to a file",
	Long: `Export records from the current stash to CSV, JSON, or JSONL format.

By default, exports to CSV format. Use --format to specify the output format.
If no file is specified, writes to stdout.

Examples:
  stash export                              # Export all to stdout (CSV)
  stash export products.csv                 # Export all to CSV file
  stash export products.json --format json  # Export all to JSON file
  stash export --format jsonl               # Export all to stdout (JSONL)
  stash export --where "Category=electronics"  # Export filtered records
  stash export --include-deleted            # Include soft-deleted records`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "csv", "Output format: csv, json, jsonl")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	exportCmd.Flags().StringArrayVar(&exportWhere, "where", nil, "Filter by field value (can be repeated)")
	exportCmd.Flags().BoolVar(&exportIncludeDeleted, "include-deleted", false, "Include soft-deleted records")
	exportCmd.Flags().BoolVarP(&exportForce, "force", "f", false, "Overwrite existing file without warning")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
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

	// Validate format
	format := strings.ToLower(exportFormat)
	if format != "csv" && format != "json" && format != "jsonl" {
		fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (must be csv, json, or jsonl)\n", exportFormat)
		Exit(1)
		return nil
	}

	// Determine output file
	outputFile := exportOutput
	if len(args) > 0 {
		outputFile = args[0]
	}

	// Check if output file exists (unless --force)
	if outputFile != "" && !exportForce {
		if _, err := os.Stat(outputFile); err == nil {
			fmt.Fprintf(os.Stderr, "Error: file '%s' already exists (use --force to overwrite)\n", outputFile)
			Exit(1)
			return nil
		}
	}

	// Parse WHERE clauses
	var whereConditions []storage.WhereCondition
	for _, clause := range exportWhere {
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
		IncludeDeleted: exportIncludeDeleted,
		ParentID:       "*", // All records
		Where:          whereConditions,
	}

	// List records
	records, err := store.ListRecords(ctx.Stash, opts)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// Determine output writer
	var writer *os.File
	if outputFile == "" {
		writer = os.Stdout
	} else {
		writer, err = os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer writer.Close()
	}

	// Get column names
	columnNames := stash.Columns.Names()

	// Export based on format
	switch format {
	case "csv":
		err = exportCSV(writer, records, columnNames)
	case "json":
		err = exportJSON(writer, records)
	case "jsonl":
		err = exportJSONL(writer, records)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		Exit(1)
		return nil
	}

	// Success message (unless writing to stdout)
	if outputFile != "" && !IsQuiet() {
		fmt.Fprintf(os.Stderr, "Exported %d record(s) to %s\n", len(records), outputFile)
	}

	return nil
}

// exportCSV writes records in CSV format.
func exportCSV(w *os.File, records []*model.Record, columnNames []string) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	if err := writer.Write(columnNames); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write records
	for _, rec := range records {
		row := make([]string, len(columnNames))
		for i, col := range columnNames {
			if val, ok := rec.Fields[col]; ok {
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// exportJSON writes records as a JSON array.
func exportJSON(w *os.File, records []*model.Record) error {
	// Build output structure (only user fields)
	output := make([]map[string]interface{}, len(records))
	for i, rec := range records {
		output[i] = rec.Fields
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	return nil
}

// exportJSONL writes records as newline-delimited JSON.
func exportJSONL(w *os.File, records []*model.Record) error {
	encoder := json.NewEncoder(w)
	for _, rec := range records {
		if err := encoder.Encode(rec.Fields); err != nil {
			return fmt.Errorf("failed to write JSONL: %w", err)
		}
	}

	return nil
}
