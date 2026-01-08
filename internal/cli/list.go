// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	listAll      bool
	listDeleted  bool
	listParent   string
	listLimit    int
	listOffset   int
	listOrderBy  string
	listDesc     bool
	listWhere    []string
	listSearch   string
	listColumns  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all records",
	Long: `List records in the current stash.

By default, shows root-level records (not deleted). Use flags to filter:

  --all              Show all records including children
  --deleted          Include soft-deleted records
  --parent ID        Show only children of the specified parent
  --limit N          Limit results to N records
  --offset N         Skip first N records
  --order-by FIELD   Sort by field (default: _updated_at)
  --desc             Sort descending
  --where CONDITION  Filter by field value (can be repeated)
  --search TERM      Search across all fields
  --columns COLS     Select specific columns (comma-separated)

WHERE clause format:
  field=value        Equals
  field!=value       Not equals
  field>value        Greater than (numeric)
  field<value        Less than (numeric)
  field>=value       Greater than or equal
  field<=value       Less than or equal
  field LIKE pattern Pattern match (use % for wildcard)

Examples:
  stash list
  stash list --json
  stash list --all
  stash list --parent inv-ex4j
  stash list --limit 10 --order-by Name
  stash list --deleted
  stash list --where "Category=electronics"
  stash list --where "Price>100" --where "Category=electronics"
  stash list --search "laptop"
  stash list --columns "Name,Price"`,
	Args: cobra.NoArgs,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "Show all records including children")
	listCmd.Flags().BoolVar(&listDeleted, "deleted", false, "Include soft-deleted records")
	listCmd.Flags().StringVar(&listParent, "parent", "", "Show only children of the specified parent")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "Limit results to N records (0 = no limit)")
	listCmd.Flags().IntVar(&listOffset, "offset", 0, "Skip first N records")
	listCmd.Flags().StringVar(&listOrderBy, "order-by", "", "Sort by field (default: _updated_at)")
	listCmd.Flags().BoolVar(&listDesc, "desc", false, "Sort descending")
	listCmd.Flags().StringArrayVar(&listWhere, "where", nil, "Filter by field value (can be repeated)")
	listCmd.Flags().StringVar(&listSearch, "search", "", "Search across all fields")
	listCmd.Flags().StringVar(&listColumns, "columns", "", "Select specific columns (comma-separated)")
	rootCmd.AddCommand(listCmd)
}

// parseWhereClause parses a WHERE clause string into a WhereCondition.
// Supported formats:
//   - field=value
//   - field!=value
//   - field>value, field<value, field>=value, field<=value
//   - field LIKE pattern
func parseWhereClause(clause string) (storage.WhereCondition, error) {
	clause = strings.TrimSpace(clause)

	// Check for LIKE operator (case-insensitive)
	likeRegex := regexp.MustCompile(`(?i)^(\S+)\s+LIKE\s+(.+)$`)
	if matches := likeRegex.FindStringSubmatch(clause); len(matches) == 3 {
		return storage.WhereCondition{
			Field:    matches[1],
			Operator: "LIKE",
			Value:    stripQuotes(matches[2]),
		}, nil
	}

	// Check for comparison operators (order matters: >= before >, <= before <, != before =)
	operators := []string{"!=", ">=", "<=", "<>", ">", "<", "="}
	for _, op := range operators {
		if idx := strings.Index(clause, op); idx > 0 {
			field := strings.TrimSpace(clause[:idx])
			value := strings.TrimSpace(clause[idx+len(op):])
			return storage.WhereCondition{
				Field:    field,
				Operator: op,
				Value:    stripQuotes(value),
			}, nil
		}
	}

	return storage.WhereCondition{}, fmt.Errorf("invalid WHERE clause: %s (expected format: field=value, field>value, or field LIKE pattern)", clause)
}

// stripQuotes removes surrounding quotes from a string.
func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Parse WHERE clauses
	var whereConditions []storage.WhereCondition
	for _, clause := range listWhere {
		cond, err := parseWhereClause(clause)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			Exit(1)
			return nil
		}
		whereConditions = append(whereConditions, cond)
	}

	// Parse columns selection
	var selectedColumns []string
	if listColumns != "" {
		for _, col := range strings.Split(listColumns, ",") {
			col = strings.TrimSpace(col)
			if col != "" {
				selectedColumns = append(selectedColumns, col)
			}
		}
	}

	// Build list options
	opts := storage.ListOptions{
		IncludeDeleted: listDeleted,
		Limit:          listLimit,
		Offset:         listOffset,
		OrderBy:        listOrderBy,
		Descending:     listDesc,
		Where:          whereConditions,
		Search:         listSearch,
		Columns:        selectedColumns,
	}

	// Handle parent filtering
	if listParent != "" {
		opts.ParentID = listParent
	} else if listAll {
		opts.ParentID = "*" // All records
	} else {
		opts.ParentID = "" // Root records only
	}

	// List records
	records, err := store.ListRecords(ctx.Stash, opts)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// JSON output
	if GetJSONOutput() {
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(records) == 0 {
		fmt.Println("No records found.")
		return nil
	}

	// Determine which columns to display
	var displayColumns []string
	if len(selectedColumns) > 0 {
		// Use user-specified columns
		displayColumns = selectedColumns
	} else {
		// Use primary column by default
		primaryCol := stash.PrimaryColumn()
		if primaryCol != nil {
			displayColumns = []string{primaryCol.Name}
		}
	}

	// Calculate column widths
	idWidth := 4 // "ID" header
	colWidths := make(map[string]int)
	for _, col := range displayColumns {
		colWidths[col] = len(col)
	}
	statusWidth := 6 // "Status" header

	for _, rec := range records {
		if len(rec.ID) > idWidth {
			idWidth = len(rec.ID)
		}
		for _, col := range displayColumns {
			if val, ok := rec.Fields[col]; ok {
				s := fmt.Sprintf("%v", val)
				if len(s) > colWidths[col] {
					colWidths[col] = len(s)
				}
			}
		}
	}

	// Cap column widths for readability
	if idWidth > 20 {
		idWidth = 20
	}
	for col := range colWidths {
		if colWidths[col] > 40 {
			colWidths[col] = 40
		}
	}

	// Print header
	headerParts := []string{fmt.Sprintf("%-*s", idWidth, "ID")}
	separatorParts := []string{strings.Repeat("-", idWidth)}
	for _, col := range displayColumns {
		headerParts = append(headerParts, fmt.Sprintf("%-*s", colWidths[col], col))
		separatorParts = append(separatorParts, strings.Repeat("-", colWidths[col]))
	}
	headerParts = append(headerParts, fmt.Sprintf("%-*s", statusWidth, "Status"))
	separatorParts = append(separatorParts, strings.Repeat("-", statusWidth))
	headerParts = append(headerParts, "Updated")
	separatorParts = append(separatorParts, strings.Repeat("-", 19))

	fmt.Println(strings.Join(headerParts, "  "))
	fmt.Println(strings.Join(separatorParts, "  "))

	// Print records
	for _, rec := range records {
		// Format ID (truncate if needed)
		id := rec.ID
		if len(id) > idWidth {
			id = id[:idWidth-3] + "..."
		}

		rowParts := []string{fmt.Sprintf("%-*s", idWidth, id)}

		// Format column values
		for _, col := range displayColumns {
			val := ""
			if v, ok := rec.Fields[col]; ok {
				val = fmt.Sprintf("%v", v)
				if len(val) > colWidths[col] {
					val = val[:colWidths[col]-3] + "..."
				}
			}
			rowParts = append(rowParts, fmt.Sprintf("%-*s", colWidths[col], val))
		}

		// Format status
		status := "active"
		if rec.IsDeleted() {
			status = "deleted"
		}
		rowParts = append(rowParts, fmt.Sprintf("%-*s", statusWidth, status))

		// Format updated time
		updated := rec.UpdatedAt.Format("2006-01-02 15:04:05")
		rowParts = append(rowParts, updated)

		fmt.Println(strings.Join(rowParts, "  "))
	}

	// Print count
	fmt.Printf("\nTotal: %d record(s)\n", len(records))

	return nil
}
