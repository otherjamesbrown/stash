// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var (
	statusProcessing bool
	statusAgent      string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of records being processed",
	Long: `Show records that are currently being processed by AI agents.

This command helps track what work is in progress across your stash. It shows
records with processing-related status values (processing, in_progress, claimed).

Flags:
  --processing     Show only records in processing state (default behavior)
  --agent NAME     Filter by agent name (matches _updated_by or claimed_by field)
  --json           Output as JSON for machine parsing

Output includes:
  - Record ID
  - Primary column value (name/title)
  - Current status
  - Agent working on it
  - Duration (time since status was set)

Examples:
  stash status                    # Show all processing records
  stash status --agent agent-1    # Show records claimed by agent-1
  stash status --json             # JSON output for scripting

AI Agent Examples:
  # Check if any work is in progress
  stash status --json | jq 'length'

  # Find my claimed records
  stash status --agent $AGENT_NAME --json | jq -r '.[].id'

  # Get longest-running processing task
  stash status --json | jq 'sort_by(-.duration_sec) | first'

Exit Codes:
  0  Success
  1  Stash not found`,
	Args: cobra.NoArgs,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusProcessing, "processing", true, "Show records in processing state")
	statusCmd.Flags().StringVar(&statusAgent, "agent", "", "Filter by agent name")
	rootCmd.AddCommand(statusCmd)
}

// ProcessingRecord represents a record in processing state for output.
type ProcessingRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Agent       string `json:"agent"`
	DurationSec int64  `json:"duration_sec"`
}

// processingStatusValues are the status values that indicate a record is being processed.
var processingStatusValues = []string{"processing", "in_progress", "claimed"}

func runStatus(cmd *cobra.Command, args []string) error {
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

	// Build WHERE conditions to find processing records
	// We need to query for records where status is one of the processing values
	var allRecords []*model.Record

	for _, statusVal := range processingStatusValues {
		opts := storage.ListOptions{
			ParentID: "*", // All records including children
			Where: []storage.WhereCondition{
				{Field: "status", Operator: "=", Value: statusVal},
			},
		}
		records, err := store.ListRecords(ctx.Stash, opts)
		if err != nil {
			return fmt.Errorf("failed to list records: %w", err)
		}
		allRecords = append(allRecords, records...)
	}

	// Filter by agent if specified
	if statusAgent != "" {
		var filtered []*model.Record
		for _, rec := range allRecords {
			agent := getAgentFromRecord(rec)
			if agent == statusAgent {
				filtered = append(filtered, rec)
			}
		}
		allRecords = filtered
	}

	// Get primary column name for display
	primaryColName := ""
	primaryCol := stash.PrimaryColumn()
	if primaryCol != nil {
		primaryColName = primaryCol.Name
	}

	// Build output records
	now := time.Now()
	var processingRecords []ProcessingRecord
	for _, rec := range allRecords {
		// Get name from primary column
		name := ""
		if primaryColName != "" {
			if val, ok := rec.Fields[primaryColName]; ok {
				name = fmt.Sprintf("%v", val)
			}
		}

		// Get status value
		status := ""
		if val, ok := rec.Fields["status"]; ok {
			status = fmt.Sprintf("%v", val)
		}

		// Get agent
		agent := getAgentFromRecord(rec)

		// Calculate duration from _updated_at
		duration := now.Sub(rec.UpdatedAt)
		durationSec := int64(duration.Seconds())

		processingRecords = append(processingRecords, ProcessingRecord{
			ID:          rec.ID,
			Name:        name,
			Status:      status,
			Agent:       agent,
			DurationSec: durationSec,
		})
	}

	// JSON output
	if GetJSONOutput() {
		data, err := json.MarshalIndent(processingRecords, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	if len(processingRecords) == 0 {
		fmt.Println("No records in processing state.")
		return nil
	}

	fmt.Println("Processing Records:")

	// Calculate column widths
	idWidth := 2
	nameWidth := 4
	statusWidth := 6
	agentWidth := 5

	for _, rec := range processingRecords {
		if len(rec.ID) > idWidth {
			idWidth = len(rec.ID)
		}
		if len(rec.Name) > nameWidth {
			nameWidth = len(rec.Name)
		}
		if len(rec.Status) > statusWidth {
			statusWidth = len(rec.Status)
		}
		if len(rec.Agent) > agentWidth {
			agentWidth = len(rec.Agent)
		}
	}

	// Cap widths
	if idWidth > 20 {
		idWidth = 20
	}
	if nameWidth > 30 {
		nameWidth = 30
	}
	if statusWidth > 15 {
		statusWidth = 15
	}
	if agentWidth > 20 {
		agentWidth = 20
	}

	// Print each record
	for _, rec := range processingRecords {
		// Truncate values if needed
		id := rec.ID
		if len(id) > idWidth {
			id = id[:idWidth-3] + "..."
		}

		name := rec.Name
		if len(name) > nameWidth {
			name = name[:nameWidth-3] + "..."
		}

		status := rec.Status
		if len(status) > statusWidth {
			status = status[:statusWidth-3] + "..."
		}

		agent := rec.Agent
		if len(agent) > agentWidth {
			agent = agent[:agentWidth-3] + "..."
		}

		// Format duration
		duration := formatDuration(time.Duration(rec.DurationSec) * time.Second)

		fmt.Printf("  %-*s  %-*q  %-*s  %-*s  %s\n",
			idWidth, id,
			nameWidth, name,
			statusWidth, status,
			agentWidth, agent,
			duration)
	}

	// Print summary
	fmt.Printf("\n%d record(s) in processing state\n", len(processingRecords))

	return nil
}

// getAgentFromRecord extracts the agent name from a record.
// Checks claimed_by field first, then falls back to _updated_by.
func getAgentFromRecord(rec *model.Record) string {
	// Check for claimed_by field first
	if val, ok := rec.Fields["claimed_by"]; ok && val != nil {
		agent := fmt.Sprintf("%v", val)
		if agent != "" {
			return agent
		}
	}

	// Fall back to _updated_by
	return rec.UpdatedBy
}

// resetStatusFlags resets the status command flags (called from resetFlags).
func resetStatusFlags() {
	statusProcessing = true
	statusAgent = ""
}

// isProcessingStatus checks if a status value indicates processing.
func isProcessingStatus(status string) bool {
	statusLower := strings.ToLower(status)
	for _, ps := range processingStatusValues {
		if statusLower == ps {
			return true
		}
	}
	return false
}
