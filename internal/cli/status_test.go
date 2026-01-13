package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// TestStatusCommand tests the stash status command
func TestStatusCommand(t *testing.T) {
	t.Run("shows records with processing status", func(t *testing.T) {
		// Given: Stash with records in various states
		tempDir, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		// Create records with different statuses
		rootCmd.SetArgs([]string{"add", "Task 1", "--set", "status=processing"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Task 2", "--set", "status=complete"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Task 3", "--set", "status=in_progress"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		_ = tempDir // unused but required by helper

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status`
		rootCmd.SetArgs([]string{"status"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Shows processing records
		if !strings.Contains(output, "Task 1") {
			t.Error("expected Task 1 (processing) to be shown")
		}
		if !strings.Contains(output, "Task 3") {
			t.Error("expected Task 3 (in_progress) to be shown")
		}
		// Should not show completed task
		if strings.Contains(output, "Task 2") {
			t.Error("expected Task 2 (complete) to NOT be shown")
		}

		// Then: Shows count
		if !strings.Contains(output, "2 record(s) in processing state") {
			t.Errorf("expected 2 records message, output: %s", output)
		}
	})

	t.Run("shows claimed status", func(t *testing.T) {
		// Given: Stash with a claimed record
		_, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Claimed Task", "--set", "status=claimed"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status`
		rootCmd.SetArgs([]string{"status"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Shows claimed record
		if !strings.Contains(output, "Claimed Task") {
			t.Errorf("expected Claimed Task to be shown, output: %s", output)
		}
		if !strings.Contains(output, "claimed") {
			t.Errorf("expected 'claimed' status in output, output: %s", output)
		}
	})

	t.Run("filters by agent name", func(t *testing.T) {
		// Given: Stash with records claimed by different agents
		tempDir, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status", "claimed_by"})
		defer cleanup()

		// Create records with different claimed_by values
		rootCmd.SetArgs([]string{"add", "Agent1 Task", "--set", "status=processing", "--set", "claimed_by=agent-1"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Agent2 Task", "--set", "status=processing", "--set", "claimed_by=agent-2"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		_ = tempDir

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status --agent agent-1`
		rootCmd.SetArgs([]string{"status", "--agent", "agent-1"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Shows only agent-1's records
		if !strings.Contains(output, "Agent1 Task") {
			t.Errorf("expected Agent1 Task to be shown, output: %s", output)
		}
		if strings.Contains(output, "Agent2 Task") {
			t.Error("expected Agent2 Task to NOT be shown")
		}
		if !strings.Contains(output, "1 record(s)") {
			t.Errorf("expected 1 record message, output: %s", output)
		}
	})

	t.Run("falls back to updated_by for agent", func(t *testing.T) {
		// Given: Record without claimed_by but with _updated_by
		tempDir, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		// Create a record, then update it with a specific actor
		rootCmd.SetArgs([]string{"add", "Test Task", "--set", "status=processing"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("tasks", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Update with specific actor
		rootCmd.SetArgs([]string{"set", recordID, "status=processing", "--actor", "test-agent"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status --agent test-agent`
		rootCmd.SetArgs([]string{"status", "--agent", "test-agent"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows the record
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Test Task") {
			t.Errorf("expected Test Task to be shown with agent filter, output: %s", output)
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		// Given: Stash with processing records
		_, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status", "claimed_by"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "JSON Task", "--set", "status=processing", "--set", "claimed_by=agent-1"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status --json`
		rootCmd.SetArgs([]string{"status", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var records []ProcessingRecord
		if err := json.Unmarshal([]byte(output), &records); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		// Then: Contains expected fields
		if len(records) != 1 {
			t.Errorf("expected 1 record, got %d", len(records))
		}
		if len(records) > 0 {
			rec := records[0]
			if rec.Name != "JSON Task" {
				t.Errorf("expected name 'JSON Task', got %s", rec.Name)
			}
			if rec.Status != "processing" {
				t.Errorf("expected status 'processing', got %s", rec.Status)
			}
			if rec.Agent != "agent-1" {
				t.Errorf("expected agent 'agent-1', got %s", rec.Agent)
			}
			if rec.DurationSec < 0 {
				t.Error("expected non-negative duration")
			}
		}
	})

	t.Run("shows duration since status was set", func(t *testing.T) {
		// Given: Stash with a processing record
		tempDir, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Duration Task", "--set", "status=processing"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Wait a moment to ensure measurable duration
		time.Sleep(100 * time.Millisecond)

		_ = tempDir

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status --json`
		rootCmd.SetArgs([]string{"status", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Duration is populated
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var records []ProcessingRecord
		if err := json.Unmarshal([]byte(output), &records); err != nil {
			t.Fatalf("expected valid JSON, error: %v", err)
		}

		if len(records) > 0 {
			if records[0].DurationSec < 0 {
				t.Error("expected positive duration_sec")
			}
		}
	})

	t.Run("no records in processing state", func(t *testing.T) {
		// Given: Stash with no processing records
		_, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Complete Task", "--set", "status=complete"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status`
		rootCmd.SetArgs([]string{"status"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows "no records" message
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "No records in processing state") {
			t.Errorf("expected 'No records' message, output: %s", output)
		}
	})

	t.Run("empty JSON output when no records", func(t *testing.T) {
		// Given: Stash with no processing records
		_, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status --json`
		rootCmd.SetArgs([]string{"status", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is empty JSON array
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var records []ProcessingRecord
		if err := json.Unmarshal([]byte(output), &records); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		if len(records) != 0 {
			t.Errorf("expected 0 records, got %d", len(records))
		}
	})

	t.Run("no stash returns error", func(t *testing.T) {
		// Given: No stash directory
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash status`
		rootCmd.SetArgs([]string{"status"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})

	t.Run("includes child records", func(t *testing.T) {
		// Given: Parent record with processing child
		tempDir, cleanup := setupTestStashWithColumns(t, "tasks", "tsk-", []string{"Name", "status"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Parent Task", "--set", "status=complete"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Get parent ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("tasks", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child with processing status
		rootCmd.SetArgs([]string{"add", "Child Task", "--set", "status=processing", "--parent", parentID})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash status`
		rootCmd.SetArgs([]string{"status"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows child record
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Child Task") {
			t.Errorf("expected Child Task to be shown, output: %s", output)
		}
		if strings.Contains(output, "Parent Task") {
			t.Error("expected Parent Task (complete) to NOT be shown")
		}
	})
}

// TestFormatDuration tests the duration formatting helper
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0 * time.Second, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{90 * time.Second, "1m 30s"},
		{5*time.Minute + 32*time.Second, "5m 32s"},
		{59*time.Minute + 59*time.Second, "59m 59s"},
		{1 * time.Hour, "1h 0m"},
		{1*time.Hour + 30*time.Minute + 45*time.Second, "1h 30m"},
		{24 * time.Hour, "24h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

// TestIsProcessingStatus tests the status checking helper
func TestIsProcessingStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"processing", true},
		{"in_progress", true},
		{"claimed", true},
		{"PROCESSING", true},
		{"IN_PROGRESS", true},
		{"CLAIMED", true},
		{"Processing", true},
		{"complete", false},
		{"pending", false},
		{"done", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := isProcessingStatus(tt.status)
			if result != tt.expected {
				t.Errorf("isProcessingStatus(%q) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

// TestGetAgentFromRecord tests the agent extraction helper
func TestGetAgentFromRecord(t *testing.T) {
	t.Run("prefers claimed_by over updated_by", func(t *testing.T) {
		rec := &model.Record{
			UpdatedBy: "updated-agent",
			Fields: map[string]interface{}{
				"claimed_by": "claimed-agent",
			},
		}
		agent := getAgentFromRecord(rec)
		if agent != "claimed-agent" {
			t.Errorf("expected 'claimed-agent', got %s", agent)
		}
	})

	t.Run("falls back to updated_by", func(t *testing.T) {
		rec := &model.Record{
			UpdatedBy: "updated-agent",
			Fields:    map[string]interface{}{},
		}
		agent := getAgentFromRecord(rec)
		if agent != "updated-agent" {
			t.Errorf("expected 'updated-agent', got %s", agent)
		}
	})

	t.Run("handles nil claimed_by", func(t *testing.T) {
		rec := &model.Record{
			UpdatedBy: "updated-agent",
			Fields: map[string]interface{}{
				"claimed_by": nil,
			},
		}
		agent := getAgentFromRecord(rec)
		if agent != "updated-agent" {
			t.Errorf("expected 'updated-agent', got %s", agent)
		}
	})

	t.Run("handles empty claimed_by", func(t *testing.T) {
		rec := &model.Record{
			UpdatedBy: "updated-agent",
			Fields: map[string]interface{}{
				"claimed_by": "",
			},
		}
		agent := getAgentFromRecord(rec)
		if agent != "updated-agent" {
			t.Errorf("expected 'updated-agent', got %s", agent)
		}
	})
}
