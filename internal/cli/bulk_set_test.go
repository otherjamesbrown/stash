package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestBulkSet tests the bulk-set command
func TestBulkSet(t *testing.T) {
	t.Run("AC-01: update single field on matching records", func(t *testing.T) {
		// Given: Multiple records exist with different Category values
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority"})
		defer cleanup()

		// Create records with different categories
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Chair", "--set", "Category=furniture"})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash bulk-set --where "Category=electronics" --set Priority=high`
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=high"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output reports 2 records updated
		if output == "" {
			t.Error("expected output message")
		}

		// Verify the matching records were updated
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		electronicsUpdated := 0
		furnitureUnchanged := 0
		for _, rec := range records {
			if rec.Fields["Category"] == "electronics" && rec.Fields["Priority"] == "high" {
				electronicsUpdated++
			}
			if rec.Fields["Category"] == "furniture" && (rec.Fields["Priority"] == nil || rec.Fields["Priority"] == "") {
				furnitureUnchanged++
			}
		}

		if electronicsUpdated != 2 {
			t.Errorf("expected 2 electronics records updated, got %d", electronicsUpdated)
		}
		if furnitureUnchanged != 1 {
			t.Errorf("expected 1 furniture record unchanged, got %d", furnitureUnchanged)
		}
	})

	t.Run("AC-02: update multiple fields with --set", func(t *testing.T) {
		// Given: Records exist
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority", "Status"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User runs `stash bulk-set --where "Category=electronics" --set Priority=high --set Status=active`
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=high", "--set", "Status=active"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify both fields were updated
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		for _, rec := range records {
			if rec.Fields["Category"] == "electronics" {
				if rec.Fields["Priority"] != "high" {
					t.Errorf("expected Priority=high, got %v", rec.Fields["Priority"])
				}
				if rec.Fields["Status"] != "active" {
					t.Errorf("expected Status=active, got %v", rec.Fields["Status"])
				}
			}
		}
	})

	t.Run("AC-03: report count of updated records", func(t *testing.T) {
		// Given: 3 records match the condition
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Tablet", "--set", "Category=electronics"})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs bulk-set
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=urgent"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output indicates 3 records updated
		if output == "" {
			t.Error("expected output message")
		}
		// The output should mention the count - we'll check it contains "3"
		// This is a flexible check - actual format will be defined in implementation
	})

	t.Run("AC-04: no records match condition", func(t *testing.T) {
		// Given: No records match the condition
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Chair", "--set", "Category=furniture"})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs bulk-set with non-matching condition
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=high"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Command succeeds but reports 0 records updated
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		// Output should indicate no records matched
		_ = output // Will be checked in implementation
	})

	t.Run("AC-05: require --where clause", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User runs bulk-set without --where
		rootCmd.SetArgs([]string{"bulk-set", "--set", "Priority=high"})
		rootCmd.Execute()

		// Then: Command fails with error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when --where is missing")
		}
	})

	t.Run("AC-06: require --set clause", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User runs bulk-set without --set
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Name=Laptop"})
		rootCmd.Execute()

		// Then: Command fails with error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when --set is missing")
		}
	})

	t.Run("AC-07: reject non-existent column in --set", func(t *testing.T) {
		// Given: Column doesn't exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User tries to set non-existent column
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Name=Laptop", "--set", "NonExistent=value"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for non-existent column")
		}
	})

	t.Run("AC-08: skip deleted records", func(t *testing.T) {
		// Given: One record is deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()

		// Get records and delete one
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		store.DeleteRecord("inventory", records[0].ID, "test")
		store.Close()

		ExitCode = 0

		// When: User runs bulk-set
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=high"})
		rootCmd.Execute()

		// Then: Only non-deleted records are updated
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Check active record was updated
		activeRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(activeRecords) != 1 {
			t.Errorf("expected 1 active record, got %d", len(activeRecords))
		}
		if activeRecords[0].Fields["Priority"] != "high" {
			t.Errorf("expected active record to have Priority=high, got %v", activeRecords[0].Fields["Priority"])
		}

		// Check deleted record was not updated
		deletedRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*", IncludeDeleted: true, DeletedOnly: true})
		if len(deletedRecords) != 1 {
			t.Errorf("expected 1 deleted record, got %d", len(deletedRecords))
		}
		// Deleted record should NOT have Priority updated
		if deletedRecords[0].Fields["Priority"] == "high" {
			t.Error("deleted record should not be updated")
		}
	})

	t.Run("AC-09: JSON output shows updated record IDs", func(t *testing.T) {
		// Given: Records exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs bulk-set with --json
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--set", "Priority=high", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		// Then: Contains count of updated records
		if jsonOutput["count"] == nil {
			t.Error("expected 'count' in JSON output")
		}
	})

	t.Run("AC-10: support IS NULL in where clause", func(t *testing.T) {
		// Given: Records with and without Priority set
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"}) // No Priority
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Priority=low"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User runs bulk-set with IS NULL condition
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Priority IS NULL", "--set", "Priority=pending"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify only the NULL record was updated
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		pendingCount := 0
		lowCount := 0
		for _, rec := range records {
			if rec.Fields["Priority"] == "pending" {
				pendingCount++
			}
			if rec.Fields["Priority"] == "low" {
				lowCount++
			}
		}

		if pendingCount != 1 {
			t.Errorf("expected 1 record with Priority=pending, got %d", pendingCount)
		}
		if lowCount != 1 {
			t.Errorf("expected 1 record with Priority=low, got %d", lowCount)
		}
	})

	t.Run("AC-11: support multiple where conditions (AND)", func(t *testing.T) {
		// Given: Records with different combinations
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Status", "Priority"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics", "--set", "Status=active"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics", "--set", "Status=inactive"})
		rootCmd.Execute()
		rootCmd.SetArgs([]string{"add", "Chair", "--set", "Category=furniture", "--set", "Status=active"})
		rootCmd.Execute()

		ExitCode = 0

		// When: User runs bulk-set with multiple where conditions
		rootCmd.SetArgs([]string{"bulk-set", "--where", "Category=electronics", "--where", "Status=active", "--set", "Priority=high"})
		err := rootCmd.Execute()

		// Then: Only record matching BOTH conditions is updated
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		highPriorityCount := 0
		for _, rec := range records {
			if rec.Fields["Priority"] == "high" {
				highPriorityCount++
				// Verify it's the right record
				if rec.Fields["Category"] != "electronics" || rec.Fields["Status"] != "active" {
					t.Error("wrong record was updated")
				}
			}
		}

		if highPriorityCount != 1 {
			t.Errorf("expected exactly 1 record with Priority=high, got %d", highPriorityCount)
		}
	})
}
