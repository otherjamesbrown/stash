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

var detachCmd = &cobra.Command{
	Use:   "detach <record-id> <filename>",
	Short: "Remove an attachment from a record",
	Long: `Remove an attached file from a record.

The file is permanently deleted from .stash/<stash>/files/<record-id>/.

Examples:
  stash detach inv-ex4j document.pdf
  stash detach inv-ex4j image.png --json`,
	Args: cobra.ExactArgs(2),
	RunE: runDetach,
}

func init() {
	rootCmd.AddCommand(detachCmd)
}

func runDetach(cmd *cobra.Command, args []string) error {
	recordID := args[0]
	filename := args[1]

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

	// Detach the file
	err = store.DetachFile(ctx.Stash, recordID, filename)
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
		if errors.Is(err, model.ErrAttachmentNotFound) {
			fmt.Fprintf(os.Stderr, "Error: attachment '%s' not found for record '%s'\n", filename, recordID)
			Exit(4)
			return nil
		}
		return fmt.Errorf("failed to detach file: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"record_id": recordID,
			"filename":  filename,
			"detached":  true,
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Detached '%s' from record %s\n", filename, recordID)
	}

	return nil
}
