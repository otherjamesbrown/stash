// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	attachMove bool
)

var attachCmd = &cobra.Command{
	Use:   "attach <record-id> <file>",
	Short: "Attach a file to a record",
	Long: `Attach a file to a record in the current stash.

The file is copied to .stash/<stash>/files/<record-id>/<filename>.
Use --move to move the file instead of copying.

File metadata (name, size, hash, attached_at, attached_by) is tracked.

Examples:
  stash attach inv-ex4j document.pdf
  stash attach inv-ex4j image.png --move
  stash attach inv-ex4j ./docs/spec.md --json`,
	Args: cobra.ExactArgs(2),
	RunE: runAttach,
}

func init() {
	attachCmd.Flags().BoolVar(&attachMove, "move", false, "Move file instead of copying")
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	recordID := args[0]
	filePath := args[1]

	// Check if source file exists
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid file path: %s\n", filePath)
		Exit(2)
		return nil
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", filePath)
		Exit(2)
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

	// Attach the file
	attachment, err := store.AttachFile(ctx.Stash, recordID, absPath, attachMove, ctx.Actor)
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
		if errors.Is(err, model.ErrFileNotFound) {
			fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", filePath)
			Exit(2)
			return nil
		}
		if errors.Is(err, model.ErrAttachmentExists) {
			fmt.Fprintf(os.Stderr, "Error: attachment '%s' already exists for record '%s'\n", filepath.Base(absPath), recordID)
			Exit(1)
			return nil
		}
		return fmt.Errorf("failed to attach file: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"record_id":   recordID,
			"name":        attachment.Name,
			"size":        attachment.Size,
			"hash":        attachment.Hash,
			"attached_at": attachment.AttachedAt,
			"attached_by": attachment.AttachedBy,
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else if !IsQuiet() {
		action := "Copied"
		if attachMove {
			action = "Moved"
		}
		fmt.Printf("%s '%s' to record %s\n", action, attachment.Name, recordID)
		if IsVerbose() {
			fmt.Printf("  size: %d bytes\n", attachment.Size)
			fmt.Printf("  hash: %s\n", attachment.Hash)
			fmt.Printf("  attached_by: %s\n", attachment.AttachedBy)
		}
	}

	return nil
}
