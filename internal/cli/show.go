// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	showWithFiles  bool
	showHistory    bool
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single record",
	Long: `Display detailed information about a single record.

Shows:
- Record ID and hash
- Parent record (if any)
- Creation and update timestamps and actors
- All user-defined fields
- Child records (if any)

Options:
  --with-files    Include inline file contents
  --history       Show change history

Examples:
  stash show inv-ex4j
  stash show inv-ex4j --json
  stash show inv-ex4j --with-files
  stash show inv-ex4j --history`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showWithFiles, "with-files", false, "Include inline file contents")
	showCmd.Flags().BoolVar(&showHistory, "history", false, "Show change history")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	recordID := args[0]

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

	// Get record
	record, err := store.GetRecord(ctx.Stash, recordID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", recordID)
			Exit(4)
			return nil
		}
		if errors.Is(err, model.ErrRecordDeleted) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is deleted\n", recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// Get children
	children, err := store.GetChildren(ctx.Stash, recordID)
	if err != nil {
		// Non-fatal, continue without children
		children = nil
	}

	// AC-02: JSON output format
	if GetJSONOutput() {
		// Build output map manually since Record has custom MarshalJSON
		output := make(map[string]interface{})

		// Marshal record to get its fields
		recordData, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal record: %w", err)
		}
		if err := json.Unmarshal(recordData, &output); err != nil {
			return fmt.Errorf("failed to unmarshal record: %w", err)
		}

		// Add children array
		if children == nil {
			children = []*model.Record{}
		}
		output["_children"] = children

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// AC-01: Human-readable output
	fmt.Printf("# Record %s\n", record.ID)
	fmt.Println()

	// System fields
	fmt.Printf("**Hash**: %s\n", record.Hash)
	if record.ParentID != "" {
		fmt.Printf("**Parent**: %s\n", record.ParentID)
	}
	fmt.Printf("**Created**: %s by %s\n", record.CreatedAt.Format("2006-01-02 15:04:05"), record.CreatedBy)
	fmt.Printf("**Updated**: %s by %s\n", record.UpdatedAt.Format("2006-01-02 15:04:05"), record.UpdatedBy)
	if record.Branch != "" {
		fmt.Printf("**Branch**: %s\n", record.Branch)
	}
	fmt.Println()

	// User fields
	fmt.Println("## Fields")
	fmt.Println()
	if len(record.Fields) > 0 {
		// Sort field names for consistent output
		fieldNames := make([]string, 0, len(record.Fields))
		for name := range record.Fields {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)

		for _, name := range fieldNames {
			value := record.Fields[name]
			fmt.Printf("- **%s**: %v\n", name, value)
		}
	} else {
		fmt.Println("No fields set.")
	}
	fmt.Println()

	// Children
	fmt.Println("## Children")
	fmt.Println()
	if len(children) > 0 {
		fmt.Println("| ID | Primary Value |")
		fmt.Println("|----|---------------|")
		primaryCol := stash.PrimaryColumn()
		for _, child := range children {
			primaryValue := ""
			if primaryCol != nil {
				if val, ok := child.Fields[primaryCol.Name]; ok {
					primaryValue = fmt.Sprintf("%v", val)
				}
			}
			fmt.Printf("| %s | %s |\n", child.ID, primaryValue)
		}
	} else {
		fmt.Println("No children.")
	}
	fmt.Println()

	// AC-03: With files
	if showWithFiles {
		fmt.Println("## Files")
		fmt.Println()
		filesDir := filepath.Join(ctx.StashDir, ctx.Stash, "files")
		filePath := filepath.Join(filesDir, recordID+".md")
		if content, err := os.ReadFile(filePath); err == nil {
			fmt.Println("```markdown")
			fmt.Println(string(content))
			fmt.Println("```")
		} else {
			fmt.Println("No attached files.")
		}
		fmt.Println()
	}

	// AC-04: History (placeholder - requires reading JSONL)
	if showHistory {
		fmt.Println("## History")
		fmt.Println()
		fmt.Println("| Timestamp | Operation | Actor | Branch |")
		fmt.Println("|-----------|-----------|-------|--------|")
		// Show current state as latest entry
		fmt.Printf("| %s | %s | %s | %s |\n",
			record.UpdatedAt.Format("2006-01-02 15:04:05"),
			record.Operation,
			record.UpdatedBy,
			record.Branch,
		)
		// TODO: Read full history from JSONL
		fmt.Println()
		fmt.Println("*Note: Full history requires reading JSONL file*")
		fmt.Println()
	}

	return nil
}
