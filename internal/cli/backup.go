// Package cli provides the command-line interface for stash.
package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
)

var (
	backupOutput string
	backupForce  bool
)

var backupCmd = &cobra.Command{
	Use:   "backup [file]",
	Short: "Create a backup of the stash",
	Long: `Create a compressed backup of the current stash including:
- Configuration (config.json)
- All records (records.jsonl)
- All attached files

The backup is saved as a .tar.gz file. If no filename is specified,
a default name with timestamp is used.

Examples:
  stash backup                     # Create backup with auto-generated name
  stash backup my-backup.tar.gz    # Create backup with specific name
  stash backup --force             # Overwrite existing backup file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBackup,
}

func init() {
	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output file (deprecated, use positional arg)")
	backupCmd.Flags().BoolVarP(&backupForce, "force", "f", false, "Overwrite existing file without warning")
	rootCmd.AddCommand(backupCmd)
}

func runBackup(cmd *cobra.Command, args []string) error {
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

	// Determine output file
	outputFile := backupOutput
	if len(args) > 0 {
		outputFile = args[0]
	}
	if outputFile == "" {
		// Generate default filename with timestamp
		timestamp := time.Now().Format("20060102-150405")
		outputFile = fmt.Sprintf("%s-backup-%s.tar.gz", ctx.Stash, timestamp)
	}

	// Check if output file exists (unless --force)
	if !backupForce {
		if _, err := os.Stat(outputFile); err == nil {
			fmt.Fprintf(os.Stderr, "Error: file '%s' already exists (use --force to overwrite)\n", outputFile)
			Exit(1)
			return nil
		}
	}

	// Get stash directory path
	stashPath := filepath.Join(ctx.StashDir, ctx.Stash)

	// Check stash directory exists
	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: stash '%s' not found\n", ctx.Stash)
		Exit(1)
		return nil
	}

	// Create backup file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Write metadata
	metadata := map[string]interface{}{
		"version":    "1.0",
		"stash":      ctx.Stash,
		"created_at": time.Now().Format(time.RFC3339),
		"created_by": ctx.Actor,
	}
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	if err := addToTar(tarWriter, "backup.json", metadataJSON); err != nil {
		return fmt.Errorf("failed to add metadata: %w", err)
	}

	// Walk the stash directory and add all files
	filesAdded := 0
	err = filepath.Walk(stashPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the SQLite cache file
		if info.Name() == "cache.db" || info.Name() == "cache.db-wal" || info.Name() == "cache.db-shm" {
			return nil
		}

		// Get relative path for tar
		relPath, err := filepath.Rel(stashPath, path)
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.Join(ctx.Stash, relPath)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content (skip directories)
		if !info.IsDir() {
			content, err := os.Open(path)
			if err != nil {
				return err
			}
			defer content.Close()

			if _, err := io.Copy(tarWriter, content); err != nil {
				return err
			}
			filesAdded++
		}

		return nil
	})

	if err != nil {
		os.Remove(outputFile) // Clean up on error
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Get file size
	fileInfo, _ := file.Stat()
	size := fileInfo.Size()

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"backup_file":  outputFile,
			"stash":        ctx.Stash,
			"files":        filesAdded,
			"size_bytes":   size,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Backup created: %s\n", outputFile)
		fmt.Printf("Stash: %s\n", ctx.Stash)
		fmt.Printf("Files: %d\n", filesAdded)
		fmt.Printf("Size: %s\n", formatBytes(size))
	}

	return nil
}

// addToTar adds content to a tar archive with the given filename.
func addToTar(tw *tar.Writer, filename string, content []byte) error {
	header := &tar.Header{
		Name: filename,
		Size: int64(len(content)),
		Mode: 0644,
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}

// formatBytes formats bytes as human-readable size.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Additional helper for model import check
var _ = model.ErrStashNotFound
