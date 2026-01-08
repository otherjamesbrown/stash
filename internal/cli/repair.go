package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair stash data issues",
	Long: `Repair stash data issues detected by doctor.

The repair command can fix various issues:
  - Rebuild SQLite cache from JSONL (--source jsonl)
  - Rebuild JSONL from SQLite (--source db)
  - Clean orphaned files (--clean-orphans)
  - Recalculate record hashes (--rehash)

All repairs require confirmation unless --yes is specified.
Use --dry-run to preview changes without making them.

Examples:
  stash repair --source jsonl        # Rebuild SQLite from JSONL
  stash repair --source db           # Rebuild JSONL from SQLite
  stash repair --clean-orphans       # Remove orphaned files
  stash repair --rehash              # Recalculate all hashes
  stash repair --dry-run             # Preview what would be repaired`,
	RunE: runRepair,
}

var (
	repairDryRun       bool
	repairYes          bool
	repairSource       string
	repairCleanOrphans bool
	repairRehash       bool
)

func init() {
	repairCmd.Flags().BoolVar(&repairDryRun, "dry-run", false, "Preview repairs without making changes")
	repairCmd.Flags().BoolVar(&repairYes, "yes", false, "Skip confirmation prompts")
	repairCmd.Flags().StringVar(&repairSource, "source", "", "Rebuild from source: 'jsonl' or 'db'")
	repairCmd.Flags().BoolVar(&repairCleanOrphans, "clean-orphans", false, "Remove orphaned files")
	repairCmd.Flags().BoolVar(&repairRehash, "rehash", false, "Recalculate all record hashes")
	rootCmd.AddCommand(repairCmd)
}

// RepairAction represents a single repair action
type RepairAction struct {
	Action  string `json:"action"`
	Target  string `json:"target"`
	Details string `json:"details,omitempty"`
	Status  string `json:"status"` // "pending", "success", "failed", "skipped"
	Error   string `json:"error,omitempty"`
}

// RepairOutput represents the JSON output for repair command
type RepairOutput struct {
	DryRun  bool           `json:"dry_run"`
	Actions []RepairAction `json:"actions"`
	Summary struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
		Skipped   int `json:"skipped"`
	} `json:"summary"`
}

func runRepair(cmd *cobra.Command, args []string) error {
	// Resolve context (stash dir, etc.)
	ctx, err := context.Resolve(actorName, stashName)
	if err != nil {
		return err
	}

	if ctx.StashDir == "" {
		return fmt.Errorf("no .stash directory found")
	}

	// Validate flags
	if repairSource != "" && repairSource != "jsonl" && repairSource != "db" {
		return fmt.Errorf("invalid --source value: must be 'jsonl' or 'db'")
	}

	// Determine what repairs to perform
	actions := planRepairs(cmd, ctx)

	if len(actions) == 0 {
		if !quiet {
			fmt.Fprintln(cmd.OutOrStdout(), "No repairs needed. Use --source, --clean-orphans, or --rehash to specify repair type.")
		}
		return nil
	}

	// Show planned actions
	if repairDryRun {
		return outputRepairResults(cmd, actions, true)
	}

	// Confirm before proceeding
	if !repairYes {
		fmt.Fprintln(cmd.OutOrStdout(), "Planned repairs:")
		for _, a := range actions {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", a.Action, a.Target)
			if a.Details != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", a.Details)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprint(cmd.OutOrStdout(), "Proceed with repairs? [y/N]: ")
		reader := bufio.NewReader(cmd.InOrStdin())
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
	}

	// Execute repairs
	actions = executeRepairs(cmd, ctx, actions)

	return outputRepairResults(cmd, actions, false)
}

func planRepairs(cmd *cobra.Command, ctx *context.Context) []RepairAction {
	var actions []RepairAction

	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return actions
	}
	defer store.Close()

	stashes, err := store.ListStashes()
	if err != nil {
		return actions
	}

	// Filter to specific stash if specified
	targetStash := stashName
	if targetStash != "" {
		var filtered []*model.Stash
		for _, s := range stashes {
			if s.Name == targetStash {
				filtered = append(filtered, s)
				break
			}
		}
		stashes = filtered
	}

	// Plan source rebuild
	if repairSource == "jsonl" {
		for _, stash := range stashes {
			actions = append(actions, RepairAction{
				Action:  "rebuild_cache",
				Target:  stash.Name,
				Details: "Rebuild SQLite cache from JSONL",
				Status:  "pending",
			})
		}
	} else if repairSource == "db" {
		for _, stash := range stashes {
			actions = append(actions, RepairAction{
				Action:  "rebuild_jsonl",
				Target:  stash.Name,
				Details: "Rebuild JSONL from SQLite database",
				Status:  "pending",
			})
		}
	}

	// Plan orphan cleanup
	if repairCleanOrphans {
		for _, stash := range stashes {
			orphans := findOrphanedFiles(ctx, stash.Name)
			if len(orphans) > 0 {
				actions = append(actions, RepairAction{
					Action:  "clean_orphans",
					Target:  stash.Name,
					Details: fmt.Sprintf("Remove %d orphaned file(s): %s", len(orphans), strings.Join(orphans, ", ")),
					Status:  "pending",
				})
			}
		}
	}

	// Plan rehash
	if repairRehash {
		for _, stash := range stashes {
			mismatches := findHashMismatches(ctx, store, stash.Name)
			if len(mismatches) > 0 {
				actions = append(actions, RepairAction{
					Action:  "rehash",
					Target:  stash.Name,
					Details: fmt.Sprintf("Recalculate hashes for %d record(s)", len(mismatches)),
					Status:  "pending",
				})
			} else {
				// Even if no mismatches, include if explicitly requested
				actions = append(actions, RepairAction{
					Action:  "rehash",
					Target:  stash.Name,
					Details: "Recalculate all record hashes",
					Status:  "pending",
				})
			}
		}
	}

	return actions
}

func findOrphanedFiles(ctx *context.Context, stashName string) []string {
	filesDir := filepath.Join(ctx.StashDir, stashName, "files")

	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return nil
	}

	// Load records to check file references
	jsonl := storage.NewJSONLStore(ctx.StashDir)
	records, _ := jsonl.ReadAllRecords(stashName)

	// Build set of referenced files
	referencedFiles := make(map[string]bool)
	for _, record := range records {
		for _, v := range record.Fields {
			if s, ok := v.(string); ok {
				if strings.HasPrefix(s, "files/") {
					referencedFiles[strings.TrimPrefix(s, "files/")] = true
				}
			}
		}
	}

	// Find orphaned files
	var orphaned []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !referencedFiles[entry.Name()] {
			orphaned = append(orphaned, entry.Name())
		}
	}

	return orphaned
}

func findHashMismatches(ctx *context.Context, store *storage.Store, stashName string) []string {
	records, err := store.ListRecords(stashName, storage.ListOptions{ParentID: "*", IncludeDeleted: false})
	if err != nil {
		return nil
	}

	var mismatches []string
	for _, record := range records {
		expectedHash := model.CalculateHash(record.Fields)
		if record.Hash != expectedHash {
			mismatches = append(mismatches, record.ID)
		}
	}

	return mismatches
}

func executeRepairs(cmd *cobra.Command, ctx *context.Context, actions []RepairAction) []RepairAction {
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		for i := range actions {
			actions[i].Status = "failed"
			actions[i].Error = fmt.Sprintf("Cannot open store: %v", err)
		}
		return actions
	}
	defer store.Close()

	for i, action := range actions {
		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Executing: %s on %s...\n", action.Action, action.Target)
		}

		switch action.Action {
		case "rebuild_cache":
			err := store.RebuildCache(action.Target)
			if err != nil {
				actions[i].Status = "failed"
				actions[i].Error = err.Error()
			} else {
				actions[i].Status = "success"
			}

		case "rebuild_jsonl":
			err := store.FlushToJSONL(action.Target)
			if err != nil {
				actions[i].Status = "failed"
				actions[i].Error = err.Error()
			} else {
				actions[i].Status = "success"
			}

		case "clean_orphans":
			err := cleanOrphanedFiles(ctx, action.Target)
			if err != nil {
				actions[i].Status = "failed"
				actions[i].Error = err.Error()
			} else {
				actions[i].Status = "success"
			}

		case "rehash":
			err := rehashRecords(ctx, store, action.Target)
			if err != nil {
				actions[i].Status = "failed"
				actions[i].Error = err.Error()
			} else {
				actions[i].Status = "success"
			}

		default:
			actions[i].Status = "skipped"
			actions[i].Error = "Unknown action"
		}
	}

	return actions
}

func cleanOrphanedFiles(ctx *context.Context, stashName string) error {
	orphans := findOrphanedFiles(ctx, stashName)
	filesDir := filepath.Join(ctx.StashDir, stashName, "files")

	for _, orphan := range orphans {
		filePath := filepath.Join(filesDir, orphan)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", orphan, err)
		}
	}

	return nil
}

func rehashRecords(ctx *context.Context, store *storage.Store, stashName string) error {
	stash, err := store.GetStash(stashName)
	if err != nil {
		return err
	}

	records, err := store.ListRecords(stashName, storage.ListOptions{ParentID: "*", IncludeDeleted: false})
	if err != nil {
		return err
	}

	// Get actor for updates
	actor := actorName
	if actor == "" {
		actor = os.Getenv("STASH_ACTOR")
		if actor == "" {
			actor = os.Getenv("USER")
			if actor == "" {
				actor = "system"
			}
		}
	}

	for _, record := range records {
		newHash := model.CalculateHash(record.Fields)
		if record.Hash != newHash {
			record.Hash = newHash
			record.UpdatedBy = actor
			if err := store.UpdateRecord(stash.Name, record); err != nil {
				return fmt.Errorf("failed to update record %s: %w", record.ID, err)
			}
		}
	}

	return nil
}

func outputRepairResults(cmd *cobra.Command, actions []RepairAction, dryRun bool) error {
	// Calculate summary
	var succeeded, failed, skipped int
	for _, a := range actions {
		switch a.Status {
		case "success":
			succeeded++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}

	if jsonOutput {
		output := RepairOutput{
			DryRun:  dryRun,
			Actions: actions,
		}
		output.Summary.Total = len(actions)
		output.Summary.Succeeded = succeeded
		output.Summary.Failed = failed
		output.Summary.Skipped = skipped

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text output
	out := cmd.OutOrStdout()

	if dryRun {
		fmt.Fprintln(out, "Dry Run - No changes made")
		fmt.Fprintln(out, "=========================")
	} else {
		fmt.Fprintln(out, "Repair Results")
		fmt.Fprintln(out, "==============")
	}
	fmt.Fprintln(out)

	for _, a := range actions {
		var statusStr string
		if dryRun {
			statusStr = "[PENDING]"
		} else {
			switch a.Status {
			case "success":
				statusStr = "[OK]"
			case "failed":
				statusStr = "[FAILED]"
			case "skipped":
				statusStr = "[SKIPPED]"
			default:
				statusStr = "[PENDING]"
			}
		}

		fmt.Fprintf(out, "%-10s %s: %s\n", statusStr, a.Action, a.Target)
		if a.Details != "" {
			fmt.Fprintf(out, "           %s\n", a.Details)
		}
		if a.Error != "" {
			fmt.Fprintf(out, "           Error: %s\n", a.Error)
		}
	}

	fmt.Fprintln(out)
	if dryRun {
		fmt.Fprintf(out, "Planned: %d action(s)\n", len(actions))
		fmt.Fprintln(out, "Run without --dry-run to execute repairs.")
	} else {
		fmt.Fprintf(out, "Summary: %d total, %d succeeded, %d failed, %d skipped\n",
			len(actions), succeeded, failed, skipped)
	}

	return nil
}
