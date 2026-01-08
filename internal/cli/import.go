// Package cli provides the command-line interface for stash.
package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	importConfirm bool
	importDryRun  bool
	importColumn  string
	importFormat  string
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import records from a file",
	Long: `Import records from a CSV, JSON, or JSONL file.

The file format is auto-detected from the extension, or can be specified
with --format. CSV is the default.

For CSV files:
- The first row must be column headers
- Missing columns will be created automatically
- The first column (or --column) is used as the primary value

Examples:
  stash import products.csv                 # Interactive import
  stash import products.csv --confirm       # Skip confirmation
  stash import products.csv --dry-run       # Preview changes
  stash import products.csv --column Name   # Use Name as primary column
  stash import products.json --format json  # Import JSON array`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	importCmd.Flags().BoolVar(&importConfirm, "confirm", false, "Skip confirmation prompt")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview what would be imported")
	importCmd.Flags().StringVar(&importColumn, "column", "", "Specify primary column name")
	importCmd.Flags().StringVar(&importFormat, "format", "", "File format: csv, json, jsonl (default: auto-detect)")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	filename := args[0]

	// Check file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file '%s' not found\n", filename)
		Exit(1)
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

	// Detect format
	format := importFormat
	if format == "" {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".csv":
			format = "csv"
		case ".json":
			format = "json"
		case ".jsonl":
			format = "jsonl"
		default:
			format = "csv" // Default to CSV
		}
	}
	format = strings.ToLower(format)

	// Parse file
	var columns []string
	var records []map[string]interface{}

	switch format {
	case "csv":
		columns, records, err = parseCSV(filename)
	case "json":
		columns, records, err = parseJSON(filename)
	case "jsonl":
		columns, records, err = parseJSONL(filename)
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (must be csv, json, or jsonl)\n", format)
		Exit(1)
		return nil
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
		Exit(1)
		return nil
	}

	if len(records) == 0 {
		fmt.Fprintln(os.Stderr, "No records to import")
		Exit(0)
		return nil
	}

	// Determine primary column
	primaryColumn := importColumn
	if primaryColumn == "" {
		if len(columns) > 0 {
			primaryColumn = columns[0]
		}
	}

	// Check which columns need to be created
	var missingColumns []string
	for _, col := range columns {
		if !stash.Columns.Exists(col) {
			missingColumns = append(missingColumns, col)
		}
	}

	// Show preview
	if !importConfirm && !GetJSONOutput() {
		fmt.Println("Import Preview")
		fmt.Println("==============")
		fmt.Printf("File: %s\n", filename)
		fmt.Printf("Format: %s\n", format)
		fmt.Printf("Records: %d\n", len(records))
		fmt.Printf("Columns: %s\n", strings.Join(columns, ", "))
		fmt.Printf("Primary column: %s\n", primaryColumn)

		if len(missingColumns) > 0 {
			fmt.Printf("New columns to create: %s\n", strings.Join(missingColumns, ", "))
		}

		// Show sample data (first 3 records)
		fmt.Println("\nSample data:")
		sampleCount := 3
		if len(records) < sampleCount {
			sampleCount = len(records)
		}
		for i := 0; i < sampleCount; i++ {
			rec := records[i]
			var parts []string
			for _, col := range columns {
				if val, ok := rec[col]; ok {
					parts = append(parts, fmt.Sprintf("%s=%v", col, val))
				}
			}
			fmt.Printf("  %d. %s\n", i+1, strings.Join(parts, ", "))
		}
		if len(records) > sampleCount {
			fmt.Printf("  ... and %d more\n", len(records)-sampleCount)
		}
		fmt.Println()
	}

	// Dry run mode
	if importDryRun {
		if GetJSONOutput() {
			output := map[string]interface{}{
				"dry_run":         true,
				"records_count":   len(records),
				"columns":         columns,
				"new_columns":     missingColumns,
				"primary_column":  primaryColumn,
			}
			data, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println("Dry run complete. No records were imported.")
		}
		return nil
	}

	// Interactive confirmation
	if !importConfirm {
		fmt.Print("Proceed with import? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Import cancelled.")
			Exit(0)
			return nil
		}
	}

	// Create missing columns
	for _, colName := range missingColumns {
		col := model.Column{
			Name:    colName,
			Added:   time.Now(),
			AddedBy: ctx.Actor,
		}
		if err := store.AddColumn(ctx.Stash, col); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating column '%s': %v\n", colName, err)
			Exit(1)
			return nil
		}
		if !IsQuiet() {
			fmt.Fprintf(os.Stderr, "Created column: %s\n", colName)
		}
	}

	// Refresh stash to get updated columns
	stash, _ = store.GetStash(ctx.Stash)

	// Import records
	imported := 0
	for i, rec := range records {
		// Get primary value
		primaryVal := ""
		if val, ok := rec[primaryColumn]; ok && val != nil {
			primaryVal = fmt.Sprintf("%v", val)
		}

		// Create record
		now := time.Now()
		recordID, err := model.GenerateID(stash.Prefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating ID for record %d: %v\n", i+1, err)
			continue
		}
		record := &model.Record{
			ID:        recordID,
			CreatedAt: now,
			CreatedBy: ctx.Actor,
			UpdatedAt: now,
			UpdatedBy: ctx.Actor,
			Fields:    make(map[string]interface{}),
		}

		// Set fields
		for _, col := range columns {
			if val, ok := rec[col]; ok {
				record.Fields[col] = val
			}
		}

		// Create the record
		if err := store.CreateRecord(ctx.Stash, record); err != nil {
			fmt.Fprintf(os.Stderr, "Error importing record %d (%s): %v\n", i+1, primaryVal, err)
			// Continue with other records
			continue
		}
		imported++
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"imported":     imported,
			"total":        len(records),
			"new_columns":  len(missingColumns),
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Imported %d of %d record(s)\n", imported, len(records))
	}

	return nil
}

// parseCSV reads a CSV file and returns columns and records.
func parseCSV(filename string) ([]string, []map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Trim whitespace from headers
	columns := make([]string, len(header))
	for i, h := range header {
		columns[i] = strings.TrimSpace(h)
	}

	// Read records
	var records []map[string]interface{}
	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read CSV row %d: %w", lineNum+1, err)
		}
		lineNum++

		rec := make(map[string]interface{})
		for i, val := range row {
			if i < len(columns) {
				// Handle empty values as empty string (not nil)
				rec[columns[i]] = strings.TrimSpace(val)
			}
		}
		records = append(records, rec)
	}

	return columns, records, nil
}

// parseJSON reads a JSON file (array of objects) and returns columns and records.
func parseJSON(filename string) ([]string, []map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var records []map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&records); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract unique columns from all records
	columnSet := make(map[string]bool)
	for _, rec := range records {
		for key := range rec {
			if !strings.HasPrefix(key, "_") { // Skip system fields
				columnSet[key] = true
			}
		}
	}

	var columns []string
	for col := range columnSet {
		columns = append(columns, col)
	}

	return columns, records, nil
}

// parseJSONL reads a JSONL file (newline-delimited JSON) and returns columns and records.
func parseJSONL(filename string) ([]string, []map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var records []map[string]interface{}
	columnSet := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var rec map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, nil, fmt.Errorf("failed to parse JSON at line %d: %w", lineNum, err)
		}

		// Collect columns
		for key := range rec {
			if !strings.HasPrefix(key, "_") {
				columnSet[key] = true
			}
		}

		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	var columns []string
	for col := range columnSet {
		columns = append(columns, col)
	}

	return columns, records, nil
}

