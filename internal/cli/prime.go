package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/storage"
)

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Generate context for agent injection",
	Long: `Generate current stash context for agent injection.

Outputs context including:
- Actor and branch information
- Column list with descriptions
- Record counts and statistics
- Recent changes summary

Use --stash to filter to a specific stash.

Examples:
  stash prime
  stash prime --stash inventory`,
	Args: cobra.NoArgs,
	RunE: runPrime,
}

func init() {
	rootCmd.AddCommand(primeCmd)
}

func runPrime(cmd *cobra.Command, args []string) error {
	// Resolve context
	ctx, _ := context.Resolve(GetActorName(), GetStashName())

	// Always output workflow rules first
	printWorkflowRules()

	// Check if stash directory exists
	if ctx.StashDir == "" {
		fmt.Println("# Current Context")
		fmt.Println()
		fmt.Println("No stashes found in current directory tree.")
		fmt.Println()
		fmt.Printf("**Actor**: %s\n", ctx.Actor)
		fmt.Printf("**Branch**: %s\n", ctx.Branch)
		return nil
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// List all stashes
	stashes, err := store.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	// Filter to specific stash if requested
	targetStash := GetStashName()
	if targetStash != "" {
		filtered := stashes[:0]
		for _, s := range stashes {
			if s.Name == targetStash {
				filtered = append(filtered, s)
			}
		}
		stashes = filtered
	}

	if len(stashes) == 0 {
		fmt.Println("# Stash Context")
		fmt.Println()
		if targetStash != "" {
			fmt.Printf("Stash '%s' not found.\n", targetStash)
		} else {
			fmt.Println("No stashes found.")
		}
		fmt.Println()
		fmt.Printf("**Actor**: %s\n", ctx.Actor)
		fmt.Printf("**Branch**: %s\n", ctx.Branch)
		return nil
	}

	// Output header
	fmt.Println("# Current Context")
	fmt.Println()
	fmt.Printf("**Actor**: %s\n", ctx.Actor)
	fmt.Printf("**Branch**: %s\n", ctx.Branch)
	fmt.Printf("**Generated**: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Output each stash
	for _, stash := range stashes {
		fmt.Printf("## %s\n", stash.Name)
		fmt.Println()
		fmt.Printf("**Prefix**: `%s`\n", stash.Prefix)
		fmt.Printf("**Created**: %s by %s\n", stash.Created.Format("2006-01-02 15:04:05"), stash.CreatedBy)
		fmt.Println()

		// Columns
		fmt.Println("### Columns")
		fmt.Println()
		if len(stash.Columns) == 0 {
			fmt.Println("No columns defined yet.")
		} else {
			fmt.Println("| Name | Description | Added |")
			fmt.Println("|------|-------------|-------|")
			for _, col := range stash.Columns {
				desc := col.Desc
				if desc == "" {
					desc = "-"
				}
				fmt.Printf("| %s | %s | %s |\n", col.Name, desc, col.Added.Format("2006-01-02"))
			}
		}
		fmt.Println()

		// Statistics
		fmt.Println("### Statistics")
		fmt.Println()

		// Count records
		records, err := store.ListRecords(stash.Name, storage.ListOptions{
			ParentID:       "*",
			IncludeDeleted: false,
		})
		recordCount := 0
		if err == nil {
			recordCount = len(records)
		}

		// Count deleted
		allRecords, err := store.ListRecords(stash.Name, storage.ListOptions{
			ParentID:       "*",
			IncludeDeleted: true,
		})
		deletedCount := 0
		if err == nil {
			deletedCount = len(allRecords) - recordCount
		}

		fmt.Printf("- **Total Records**: %d\n", recordCount)
		fmt.Printf("- **Deleted Records**: %d\n", deletedCount)
		fmt.Println()

		// Recent changes (last 5 records by updated_at)
		fmt.Println("### Recent Changes")
		fmt.Println()

		recentRecords, err := store.ListRecords(stash.Name, storage.ListOptions{
			ParentID:   "*",
			Limit:      5,
			OrderBy:    "updated_at",
			Descending: true,
		})
		if err == nil && len(recentRecords) > 0 {
			fmt.Println("| ID | Updated | By |")
			fmt.Println("|----|---------|-----|")
			for _, rec := range recentRecords {
				// Get primary value if available
				primaryCol := stash.PrimaryColumn()
				idDisplay := rec.ID
				if primaryCol != nil {
					if val, ok := rec.Fields[primaryCol.Name]; ok {
						if s, ok := val.(string); ok && s != "" {
							idDisplay = fmt.Sprintf("%s (%s)", rec.ID, truncate(s, 20))
						}
					}
				}
				fmt.Printf("| %s | %s | %s |\n",
					idDisplay,
					rec.UpdatedAt.Format("2006-01-02 15:04"),
					rec.UpdatedBy,
				)
			}
		} else {
			fmt.Println("No recent changes.")
		}
		fmt.Println()

		// Files directory info
		filesDir := filepath.Join(ctx.StashDir, stash.Name, "files")
		fmt.Println("### Files")
		fmt.Println()
		fmt.Printf("- **Directory**: `%s`\n", filesDir)
		fmt.Println()
	}

	return nil
}

// truncate shortens a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// printWorkflowRules outputs the workflow rules section for AI agents
func printWorkflowRules() {
	fmt.Print(`# Stash Workflow

## Setup (Column-First)

Stash requires columns before records:

` + "```" + `bash
stash init <name> --prefix <pfx>-   # Create stash
stash column add <col1> <col2> ...  # Define columns FIRST
stash add <value> --set key=value   # Then add records
` + "```" + `

## Column Naming

- Use underscores, not hyphens: ` + "`company_name`" + ` not ` + "`company-name`" + `
- Start with letter, alphanumeric + underscore only
- First column is "primary" - receives the value in ` + "`stash add <value>`" + `

## Essential Commands

| Action | Command |
|--------|---------|
| Add record | ` + "`stash add \"value\" --set field=value`" + ` |
| List records | ` + "`stash list`" + ` or ` + "`stash list --where \"field=value\"`" + ` |
| Show record | ` + "`stash show <id>`" + ` |
| Update field | ` + "`stash set <id> field=value`" + ` |
| Delete record | ` + "`stash rm <id>`" + ` |
| Add column | ` + "`stash column add <name>`" + ` |
| Export data | ` + "`stash export <file.jsonl>`" + ` |

## Common Patterns

**Batch column setup:**
` + "```" + `bash
stash column add name status priority notes created_at
` + "```" + `

**Add with multiple fields:**
` + "```" + `bash
stash add "Task title" --set status=pending --set priority=high
` + "```" + `

**Filter and query:**
` + "```" + `bash
stash list --where "status=pending"
stash list --where "notes IS NULL"       # Find records with unset field
stash list --where "notes IS EMPTY"      # Find records with NULL or empty string
stash list --where "notes IS NOT EMPTY"  # Find records with values
stash query "SELECT * FROM records WHERE priority='high'"
` + "```" + `

---

`)
}
