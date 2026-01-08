package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/stash/internal/storage"
)

// TestUC_QRY_004_ViewChangeHistory tests UC-QRY-004: View Change History
func TestUC_QRY_004_ViewChangeHistory(t *testing.T) {
	t.Run("AC-01: show all recent changes", func(t *testing.T) {
		// Given: Stash has records with multiple changes
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create multiple records (creates multiple change entries)
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history`
		rootCmd.SetArgs([]string{"history"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Recent operations are listed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Shows timestamp, operation, ID, actor
		// Output should contain these headers or data fields
		if !strings.Contains(output, "create") {
			t.Error("expected output to contain 'create' operation")
		}
	})

	t.Run("AC-02: show history for specific record", func(t *testing.T) {
		// Given: Record inv-ex4j has been updated multiple times
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Update the record
		rootCmd.SetArgs([]string{"set", recordID, "Price=999"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Update again
		rootCmd.SetArgs([]string{"set", recordID, "Price=899"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history inv-ex4j`
		rootCmd.SetArgs([]string{"history", recordID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only changes to that record are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Includes all operations (create, updates)
		if !strings.Contains(output, recordID) {
			t.Error("expected output to contain record ID")
		}
		if !strings.Contains(output, "create") {
			t.Error("expected output to contain 'create' operation")
		}
		if !strings.Contains(output, "update") {
			t.Error("expected output to contain 'update' operation")
		}
	})

	t.Run("AC-03: filter by actor", func(t *testing.T) {
		// Given: Changes by multiple actors exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create records with different actors
		rootCmd.SetArgs([]string{"add", "Laptop", "--actor", "alice"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse", "--actor", "bob"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history --by alice`
		rootCmd.SetArgs([]string{"history", "--by", "alice"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only changes by alice are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		if !strings.Contains(output, "alice") {
			t.Error("expected output to contain 'alice'")
		}
		// Bob's changes should not be shown
		if strings.Contains(output, "bob") {
			t.Error("expected output NOT to contain 'bob'")
		}
		// Should show exactly 1 change (alice's)
		if !strings.Contains(output, "1 change(s)") {
			t.Error("expected output to show 1 change(s)")
		}
	})

	t.Run("AC-04: filter by time", func(t *testing.T) {
		// Given: Changes over past week exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record (will be within last 24h)
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history --since 24h`
		rootCmd.SetArgs([]string{"history", "--since", "24h"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only changes in last 24 hours shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should contain the recent record
		if !strings.Contains(output, "Laptop") && !strings.Contains(output, "create") {
			t.Error("expected output to show recent changes")
		}
	})

	t.Run("AC-05: limit results", func(t *testing.T) {
		// Given: Many changes exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create multiple records
		for i := 0; i < 5; i++ {
			rootCmd.SetArgs([]string{"add", "Item" + string(rune('A'+i))})
			rootCmd.Execute()
			resetFlags()
			ExitCode = 0
			time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history --limit 2`
		rootCmd.SetArgs([]string{"history", "--limit", "2"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only 2 most recent changes shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Count the number of "create" lines (rough check for limited results)
		createCount := strings.Count(output, "create")
		if createCount > 2 {
			t.Errorf("expected at most 2 entries, output shows %d creates", createCount)
		}
	})

	t.Run("AC-06: JSON output", func(t *testing.T) {
		// Given: Changes exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash history --json`
		rootCmd.SetArgs([]string{"history", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON array
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		var jsonResult []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonResult); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		// Should have at least one entry
		if len(jsonResult) == 0 {
			t.Error("expected at least one history entry")
		}
	})

	t.Run("reject non-existent record", func(t *testing.T) {
		// Given: No record inv-fake exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash history inv-fake`
		rootCmd.SetArgs([]string{"history", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4 (record not found)
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for non-existent record, got %d", ExitCode)
		}
	})
}

// TestUC_QRY_004_ViewChangeHistory_MustNot tests anti-requirements
func TestUC_QRY_004_ViewChangeHistory_MustNot(t *testing.T) {
	t.Run("must not modify data", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Get initial state
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		origHash := records[0].Hash
		origUpdatedAt := records[0].UpdatedAt
		store.Close()

		// When: User runs history command
		rootCmd.SetArgs([]string{"history"})
		rootCmd.Execute()

		// Then: Record should not be modified
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()
		records, _ = store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if records[0].Hash != origHash {
			t.Error("history command should not modify record hash")
		}
		if !records[0].UpdatedAt.Equal(origUpdatedAt) {
			t.Error("history command should not modify updated_at")
		}
	})
}
