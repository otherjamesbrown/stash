// Package cli provides the command-line interface for stash.
package cli

import (
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

var childrenCmd = &cobra.Command{
	Use:   "children <parent-id>",
	Short: "List direct children of a record",
	Long: `List the direct children of a record.

Only direct children are shown - grandchildren and deeper descendants
are not included. Use --parent on list for filtered listing or --all
to see entire hierarchy.

Examples:
  stash children inv-ex4j
  stash children inv-ex4j --json`,
	Args: cobra.ExactArgs(1),
	RunE: runChildren,
}

func init() {
	rootCmd.AddCommand(childrenCmd)
}

func runChildren(cmd *cobra.Command, args []string) error {
	parentID := args[0]

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

	// Verify parent record exists
	_, err = store.GetRecord(ctx.Stash, parentID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' not found\n", parentID)
			Exit(4)
			return nil
		}
		if errors.Is(err, model.ErrRecordDeleted) {
			fmt.Fprintf(os.Stderr, "Error: record '%s' is deleted\n", parentID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	// Get direct children
	children, err := store.GetChildren(ctx.Stash, parentID)
	if err != nil {
		return fmt.Errorf("failed to get children: %w", err)
	}

	// JSON output
	if GetJSONOutput() {
		if children == nil {
			children = []*model.Record{}
		}
		data, err := json.MarshalIndent(children, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(children) == 0 {
		fmt.Println("No children.")
		return nil
	}

	// Get primary column for display
	primaryCol := stash.PrimaryColumn()

	// Calculate column widths
	idWidth := 4 // "ID" header
	nameWidth := 12
	if primaryCol != nil {
		nameWidth = len(primaryCol.Name)
	}

	for _, child := range children {
		if len(child.ID) > idWidth {
			idWidth = len(child.ID)
		}
		if primaryCol != nil {
			if val, ok := child.Fields[primaryCol.Name]; ok {
				s := fmt.Sprintf("%v", val)
				if len(s) > nameWidth {
					nameWidth = len(s)
				}
			}
		}
	}

	// Cap widths
	if idWidth > 20 {
		idWidth = 20
	}
	if nameWidth > 40 {
		nameWidth = 40
	}

	// Print header
	headerName := "Value"
	if primaryCol != nil {
		headerName = primaryCol.Name
	}

	fmt.Printf("%-*s  %-*s  %s\n", idWidth, "ID", nameWidth, headerName, "Updated")
	fmt.Printf("%s  %s  %s\n", strings.Repeat("-", idWidth), strings.Repeat("-", nameWidth), strings.Repeat("-", 19))

	// Print children
	for _, child := range children {
		id := child.ID
		if len(id) > idWidth {
			id = id[:idWidth-3] + "..."
		}

		name := ""
		if primaryCol != nil {
			if val, ok := child.Fields[primaryCol.Name]; ok {
				name = fmt.Sprintf("%v", val)
				if len(name) > nameWidth {
					name = name[:nameWidth-3] + "..."
				}
			}
		}

		updated := child.UpdatedAt.Format("2006-01-02 15:04:05")

		fmt.Printf("%-*s  %-*s  %s\n", idWidth, id, nameWidth, name, updated)
	}

	fmt.Printf("\nTotal: %d child(ren)\n", len(children))

	return nil
}
