// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var filesCmd = &cobra.Command{
	Use:   "files <record-id>",
	Short: "List files attached to a record",
	Long: `List all files attached to a record.

Shows filename, size, and hash for each attachment.

Examples:
  stash files inv-ex4j
  stash files inv-ex4j --json`,
	Args: cobra.ExactArgs(1),
	RunE: runFiles,
}

func init() {
	rootCmd.AddCommand(filesCmd)
}

func runFiles(cmd *cobra.Command, args []string) error {
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

	// Verify stash exists
	_, err = store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// List attachments
	attachments, err := store.ListAttachments(ctx.Stash, recordID)
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
		return fmt.Errorf("failed to list attachments: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"record_id": recordID,
			"count":     len(attachments),
			"files":     attachments,
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		if len(attachments) == 0 {
			if !IsQuiet() {
				fmt.Printf("No files attached to record %s\n", recordID)
			}
			return nil
		}

		fmt.Printf("# Files for record %s\n\n", recordID)
		fmt.Println("| Name | Size | Hash |")
		fmt.Println("|------|------|------|")
		for _, a := range attachments {
			fmt.Printf("| %s | %s | %s |\n", a.Name, formatSize(a.Size), a.Hash[:12])
		}
	}

	return nil
}

// formatSize formats a file size in human-readable format.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
