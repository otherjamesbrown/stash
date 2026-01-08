package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/storage"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show stash information and status",
	Long: `Show information about all stashes.

Displays record counts, deleted record counts, file counts,
daemon status, and current actor/branch context.

Examples:
  stash info
  stash info --json`,
	Args: cobra.NoArgs,
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

// StashInfo represents information about a single stash
type StashInfo struct {
	Name         string `json:"name"`
	Prefix       string `json:"prefix"`
	Columns      int    `json:"columns"`
	Records      int    `json:"records"`
	Deleted      int    `json:"deleted"`
	Files        int    `json:"files"`
	CreatedBy    string `json:"created_by"`
	CreatedAt    string `json:"created_at"`
}

// InfoOutput represents the full info output
type InfoOutput struct {
	Stashes []StashInfo `json:"stashes"`
	Context struct {
		Actor  string `json:"actor"`
		Branch string `json:"branch"`
	} `json:"context"`
	Daemon struct {
		Running bool   `json:"running"`
		PID     int    `json:"pid,omitempty"`
		Status  string `json:"status"`
	} `json:"daemon"`
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Resolve context
	ctx, _ := context.Resolve(GetActorName(), "")

	// Check if stash directory exists
	if ctx.StashDir == "" {
		if GetJSONOutput() {
			output := InfoOutput{}
			output.Context.Actor = ctx.Actor
			output.Context.Branch = ctx.Branch
			output.Daemon.Running = false
			output.Daemon.Status = "not running"
			output.Stashes = []StashInfo{}
			data, _ := json.Marshal(output)
			fmt.Println(string(data))
		} else {
			fmt.Println("No stashes found.")
			fmt.Printf("\nContext:\n")
			fmt.Printf("  Actor:  %s\n", ctx.Actor)
			fmt.Printf("  Branch: %s\n", ctx.Branch)
		}
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

	// Build info for each stash
	stashInfos := make([]StashInfo, 0, len(stashes))
	for _, stash := range stashes {
		info := StashInfo{
			Name:      stash.Name,
			Prefix:    stash.Prefix,
			Columns:   len(stash.Columns),
			CreatedBy: stash.CreatedBy,
			CreatedAt: stash.Created.Format("2006-01-02 15:04:05"),
		}

		// Count records
		records, err := store.ListRecords(stash.Name, storage.ListOptions{
			ParentID:       "*", // All records
			IncludeDeleted: false,
		})
		if err == nil {
			info.Records = len(records)
		}

		// Count deleted records
		allRecords, err := store.ListRecords(stash.Name, storage.ListOptions{
			ParentID:       "*", // All records
			IncludeDeleted: true,
		})
		if err == nil {
			info.Deleted = len(allRecords) - info.Records
		}

		// Count files
		filesDir := filepath.Join(ctx.StashDir, stash.Name, "files")
		files, err := os.ReadDir(filesDir)
		if err == nil {
			info.Files = len(files)
		}

		stashInfos = append(stashInfos, info)
	}

	// Check daemon status (placeholder - daemon not yet implemented)
	daemonRunning := false
	daemonPID := 0
	daemonStatus := "not running"

	// Output result
	if GetJSONOutput() {
		output := InfoOutput{}
		output.Stashes = stashInfos
		output.Context.Actor = ctx.Actor
		output.Context.Branch = ctx.Branch
		output.Daemon.Running = daemonRunning
		output.Daemon.PID = daemonPID
		output.Daemon.Status = daemonStatus
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else {
		if len(stashInfos) == 0 {
			fmt.Println("No stashes found.")
		} else {
			fmt.Println("Stashes:")
			for _, info := range stashInfos {
				fmt.Printf("\n  %s (prefix: %s)\n", info.Name, info.Prefix)
				fmt.Printf("    Columns: %d\n", info.Columns)
				fmt.Printf("    Records: %d", info.Records)
				if info.Deleted > 0 {
					fmt.Printf(" (%d deleted)", info.Deleted)
				}
				fmt.Println()
				fmt.Printf("    Files:   %d\n", info.Files)
				if IsVerbose() {
					fmt.Printf("    Created: %s by %s\n", info.CreatedAt, info.CreatedBy)
				}
			}
		}

		fmt.Printf("\nContext:\n")
		fmt.Printf("  Actor:  %s\n", ctx.Actor)
		fmt.Printf("  Branch: %s\n", ctx.Branch)

		fmt.Printf("\nDaemon:\n")
		fmt.Printf("  Status: %s\n", daemonStatus)
	}

	return nil
}
