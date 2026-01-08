// Package cli provides the command-line interface for stash.
package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/storage"
)

var (
	restoreBackupForce   bool
	restoreBackupConfirm bool
	restoreBackupDryRun  bool
)

var restoreBackupCmd = &cobra.Command{
	Use:   "restore-backup <file>",
	Short: "Restore stash from a backup file",
	Long: `Restore a stash from a backup file created by 'stash backup'.

This command extracts the backup archive and restores the stash
including configuration, records, and attached files.

WARNING: This will overwrite the existing stash if it exists.
Use --confirm to skip the confirmation prompt.

Examples:
  stash restore-backup my-backup.tar.gz              # Interactive restore
  stash restore-backup my-backup.tar.gz --confirm    # Skip confirmation
  stash restore-backup my-backup.tar.gz --dry-run    # Preview restore
  stash restore-backup my-backup.tar.gz --force      # Overwrite existing`,
	Args: cobra.ExactArgs(1),
	RunE: runRestoreBackup,
}

func init() {
	restoreBackupCmd.Flags().BoolVarP(&restoreBackupForce, "force", "f", false, "Overwrite existing stash without warning")
	restoreBackupCmd.Flags().BoolVar(&restoreBackupConfirm, "confirm", false, "Skip confirmation prompt")
	restoreBackupCmd.Flags().BoolVar(&restoreBackupDryRun, "dry-run", false, "Preview what would be restored")
	rootCmd.AddCommand(restoreBackupCmd)
}

func runRestoreBackup(cmd *cobra.Command, args []string) error {
	backupFile := args[0]

	// Check backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: backup file '%s' not found\n", backupFile)
		Exit(1)
		return nil
	}

	// Open backup file
	file, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to decompress backup file (invalid gzip format)\n")
		Exit(1)
		return nil
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Read backup metadata and collect file entries
	var metadata map[string]interface{}
	var files []tarFileEntry
	var stashName string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read backup: %w", err)
		}

		// Read backup.json metadata
		if header.Name == "backup.json" {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("failed to read backup metadata: %w", err)
			}
			if err := json.Unmarshal(content, &metadata); err != nil {
				return fmt.Errorf("failed to parse backup metadata: %w", err)
			}
			if s, ok := metadata["stash"].(string); ok {
				stashName = s
			}
			continue
		}

		// Collect file entries
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("failed to read file from backup: %w", err)
			}
			files = append(files, tarFileEntry{
				Name:    header.Name,
				Content: content,
				Mode:    header.Mode,
			})
		} else if header.Typeflag == tar.TypeDir {
			files = append(files, tarFileEntry{
				Name:  header.Name,
				IsDir: true,
				Mode:  header.Mode,
			})
		}
	}

	if stashName == "" {
		fmt.Fprintln(os.Stderr, "Error: invalid backup file (missing stash name)")
		Exit(1)
		return nil
	}

	// Resolve context (for stash directory)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Determine stash directory
	stashDir := ctx.StashDir
	if stashDir == "" {
		stashDir = storage.DefaultBaseDir()
	}

	stashPath := filepath.Join(stashDir, stashName)

	// Check if stash already exists
	stashExists := false
	if _, err := os.Stat(stashPath); err == nil {
		stashExists = true
	}

	// Show preview
	if !restoreBackupConfirm && !GetJSONOutput() {
		fmt.Println("Restore Preview")
		fmt.Println("===============")
		fmt.Printf("Backup file: %s\n", backupFile)
		fmt.Printf("Stash: %s\n", stashName)
		if createdAt, ok := metadata["created_at"].(string); ok {
			fmt.Printf("Backup date: %s\n", createdAt)
		}
		if createdBy, ok := metadata["created_by"].(string); ok {
			fmt.Printf("Created by: %s\n", createdBy)
		}
		fmt.Printf("Files: %d\n", len(files))
		if stashExists {
			fmt.Println("\nWARNING: Stash already exists and will be overwritten!")
		}
		fmt.Println()
	}

	// Dry run mode
	if restoreBackupDryRun {
		if GetJSONOutput() {
			output := map[string]interface{}{
				"dry_run":      true,
				"stash":        stashName,
				"files":        len(files),
				"stash_exists": stashExists,
				"metadata":     metadata,
			}
			data, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println("Dry run complete. No changes were made.")
		}
		return nil
	}

	// Check for existing stash without --force
	if stashExists && !restoreBackupForce {
		if !restoreBackupConfirm {
			fmt.Print("Overwrite existing stash? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Restore cancelled.")
				Exit(0)
				return nil
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: stash '%s' already exists (use --force to overwrite)\n", stashName)
			Exit(1)
			return nil
		}
	}

	// Interactive confirmation
	if !restoreBackupConfirm && !stashExists {
		fmt.Print("Proceed with restore? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Restore cancelled.")
			Exit(0)
			return nil
		}
	}

	// Remove existing stash if exists
	if stashExists {
		if err := os.RemoveAll(stashPath); err != nil {
			return fmt.Errorf("failed to remove existing stash: %w", err)
		}
	}

	// Ensure stash directory exists
	if err := os.MkdirAll(stashDir, 0755); err != nil {
		return fmt.Errorf("failed to create stash directory: %w", err)
	}

	// Extract files
	filesRestored := 0
	for _, entry := range files {
		targetPath := filepath.Join(stashDir, entry.Name)

		if entry.IsDir {
			if err := os.MkdirAll(targetPath, os.FileMode(entry.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", entry.Name, err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", entry.Name, err)
		}

		// Write file
		if err := os.WriteFile(targetPath, entry.Content, os.FileMode(entry.Mode)); err != nil {
			return fmt.Errorf("failed to write file %s: %w", entry.Name, err)
		}
		filesRestored++
	}

	// Rebuild SQLite cache
	store, err := storage.NewStore(stashDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize storage for cache rebuild: %v\n", err)
	} else {
		defer store.Close()
		if err := store.RebuildCache(stashName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to rebuild cache: %v\n", err)
		}
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"stash":          stashName,
			"files_restored": filesRestored,
			"stash_path":     stashPath,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Restored stash '%s' from backup\n", stashName)
		fmt.Printf("Files restored: %d\n", filesRestored)
	}

	return nil
}

// tarFileEntry represents a file entry from a tar archive.
type tarFileEntry struct {
	Name    string
	Content []byte
	Mode    int64
	IsDir   bool
}
