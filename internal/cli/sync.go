package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/storage"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize JSONL and SQLite cache",
	Long: `Synchronize JSONL source of truth with SQLite cache.

The sync command ensures consistency between JSONL files (source of truth)
and the SQLite cache (for fast queries).

Flags:
  --status      Show sync status for all stashes
  --rebuild     Rebuild SQLite cache from JSONL files
  --flush       Write current state to compacted JSONL
  --from-main   Pull JSONL changes from main branch (for worktrees)`,
	RunE: runSync,
}

var (
	syncStatus   bool
	syncRebuild  bool
	syncFlush    bool
	syncFromMain bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncStatus, "status", false, "Show sync status")
	syncCmd.Flags().BoolVar(&syncRebuild, "rebuild", false, "Rebuild SQLite from JSONL")
	syncCmd.Flags().BoolVar(&syncFlush, "flush", false, "Flush changes to JSONL")
	syncCmd.Flags().BoolVar(&syncFromMain, "from-main", false, "Pull changes from main branch")
	rootCmd.AddCommand(syncCmd)
}

// SyncStatusOutput represents JSON output for sync status
type SyncStatusOutput struct {
	Stashes  []StashStatus `json:"stashes"`
	LastSync *string       `json:"last_sync,omitempty"`
}

// StashStatus represents status for a single stash
type StashStatus struct {
	Name    string `json:"name"`
	Prefix  string `json:"prefix"`
	Synced  bool   `json:"synced"`
	Records int    `json:"records"`
}

func runSync(cmd *cobra.Command, args []string) error {
	// Resolve context (stash dir, etc.)
	ctx, err := context.Resolve(actorName, stashName)
	if err != nil {
		return err
	}

	if ctx.StashDir == "" {
		return fmt.Errorf("no .stash directory found")
	}

	// Open store
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer store.Close()

	// Determine which operation to perform
	if syncStatus {
		return showSyncStatus(cmd, store, ctx)
	}

	if syncFromMain {
		return syncFromMainBranch(cmd, store, ctx)
	}

	if syncRebuild {
		return rebuildCache(cmd, store, ctx)
	}

	if syncFlush {
		return flushToJSONL(cmd, store, ctx)
	}

	// Default: show status if no flags
	return showSyncStatus(cmd, store, ctx)
}

func showSyncStatus(cmd *cobra.Command, store *storage.Store, ctx *context.Context) error {
	stashes, err := store.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	if jsonOutput {
		output := SyncStatusOutput{
			Stashes: make([]StashStatus, 0, len(stashes)),
		}

		for _, stash := range stashes {
			count, _ := store.CountRecords(stash.Name)
			output.Stashes = append(output.Stashes, StashStatus{
				Name:    stash.Name,
				Prefix:  stash.Prefix,
				Synced:  true, // For now, always synced
				Records: count,
			})
		}

		// Get last sync time
		if lastSync, err := store.GetLastSyncTime(); err == nil && !lastSync.IsZero() {
			s := lastSync.Format(time.RFC3339)
			output.LastSync = &s
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text output
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Stash Status:")

	if len(stashes) == 0 {
		fmt.Fprintln(out, "  No stashes found")
		return nil
	}

	for _, stash := range stashes {
		count, _ := store.CountRecords(stash.Name)
		fmt.Fprintf(out, "  %s (%s)  synced, %d records\n", stash.Name, stash.Prefix, count)
	}

	// Show last sync time
	if lastSync, err := store.GetLastSyncTime(); err == nil && !lastSync.IsZero() {
		elapsed := time.Since(lastSync)
		fmt.Fprintf(out, "\nLast sync: %s ago\n", formatDuration(elapsed))
	}

	return nil
}

func rebuildCache(cmd *cobra.Command, store *storage.Store, ctx *context.Context) error {
	// If specific stash, rebuild only that one
	if ctx.Stash != "" {
		// Verify stash exists
		if _, err := store.GetStash(ctx.Stash); err != nil {
			return fmt.Errorf("stash '%s' not found", ctx.Stash)
		}
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilding cache for %s...\n", ctx.Stash)
		}
		if err := store.RebuildCache(ctx.Stash); err != nil {
			return fmt.Errorf("failed to rebuild cache for %s: %w", ctx.Stash, err)
		}
		if !quiet {
			fmt.Fprintln(cmd.OutOrStdout(), "Done.")
		}
		return nil
	}

	// Rebuild all stashes
	stashes, err := store.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	for _, stash := range stashes {
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilding cache for %s...\n", stash.Name)
		}
		if err := store.RebuildCache(stash.Name); err != nil {
			return fmt.Errorf("failed to rebuild cache for %s: %w", stash.Name, err)
		}
	}

	if !quiet {
		fmt.Fprintln(cmd.OutOrStdout(), "Done.")
	}
	return nil
}

func flushToJSONL(cmd *cobra.Command, store *storage.Store, ctx *context.Context) error {
	// If specific stash, flush only that one
	if ctx.Stash != "" {
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Flushing %s to JSONL...\n", ctx.Stash)
		}
		if err := store.FlushToJSONL(ctx.Stash); err != nil {
			return fmt.Errorf("failed to flush %s: %w", ctx.Stash, err)
		}
		if !quiet {
			fmt.Fprintln(cmd.OutOrStdout(), "Done.")
		}
		return nil
	}

	// Flush all stashes
	stashes, err := store.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	for _, stash := range stashes {
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Flushing %s to JSONL...\n", stash.Name)
		}
		if err := store.FlushToJSONL(stash.Name); err != nil {
			return fmt.Errorf("failed to flush %s: %w", stash.Name, err)
		}
	}

	if !quiet {
		fmt.Fprintln(cmd.OutOrStdout(), "Done.")
	}
	return nil
}

func syncFromMainBranch(cmd *cobra.Command, store *storage.Store, ctx *context.Context) error {
	// Find main worktree path
	mainPath, err := findMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to find main worktree: %w", err)
	}

	mainStashDir := filepath.Join(mainPath, ".stash")
	if _, err := os.Stat(mainStashDir); os.IsNotExist(err) {
		return fmt.Errorf("no .stash directory in main worktree at %s", mainPath)
	}

	// If specific stash, sync only that one
	stashName := ctx.Stash
	if stashName != "" {
		return syncStashFromMain(cmd, store, ctx, mainStashDir, stashName)
	}

	// Sync all stashes
	stashes, err := store.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	for _, stash := range stashes {
		if err := syncStashFromMain(cmd, store, ctx, mainStashDir, stash.Name); err != nil {
			return err
		}
	}

	return nil
}

func syncStashFromMain(cmd *cobra.Command, store *storage.Store, ctx *context.Context, mainStashDir, stashName string) error {
	srcJSONL := filepath.Join(mainStashDir, stashName, "records.jsonl")
	dstJSONL := filepath.Join(ctx.StashDir, stashName, "records.jsonl")

	if _, err := os.Stat(srcJSONL); os.IsNotExist(err) {
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s: no records.jsonl in main\n", stashName)
		}
		return nil
	}

	if !quiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Pulling %s from main...\n", stashName)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstJSONL), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Copy JSONL file
	if err := copyFile(srcJSONL, dstJSONL); err != nil {
		return fmt.Errorf("failed to copy JSONL: %w", err)
	}

	// Rebuild cache from new JSONL
	if err := store.RebuildCache(stashName); err != nil {
		return fmt.Errorf("failed to rebuild cache: %w", err)
	}

	if !quiet {
		fmt.Fprintln(cmd.OutOrStdout(), "Done.")
	}

	return nil
}

// findMainWorktreePath returns the path to the main git worktree
func findMainWorktreePath() (string, error) {
	// Check if we're in a worktree
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}

	commonDir := strings.TrimSpace(string(out))

	// Get the main worktree path
	// git common dir is typically .git for main repo or ../.git for worktrees
	cmd = exec.Command("git", "worktree", "list", "--porcelain")
	out, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse worktree list to find main worktree
	lines := strings.Split(string(out), "\n")
	var mainPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// The first worktree in the list is typically the main one
			if mainPath == "" {
				mainPath = path
			}
		}
	}

	if mainPath == "" {
		// Not in a worktree, use current repo
		// Check for parent .git
		gitDir := strings.TrimSpace(commonDir)
		if strings.HasSuffix(gitDir, ".git") {
			mainPath = filepath.Dir(gitDir)
		} else {
			// Common dir might be ../main/.git or similar
			mainPath = filepath.Dir(gitDir)
		}
	}

	return mainPath, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

// formatDuration is defined in daemon.go
