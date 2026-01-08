package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_005_DeleteRecord tests UC-REC-005: Delete Record (Soft)
func TestUC_REC_005_DeleteRecord(t *testing.T) {
	t.Run("AC-01: soft-delete record", func(t *testing.T) {
		// Given: Record inv-ex4j exists and is active
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash rm inv-ex4j --yes`
		rootCmd.SetArgs([]string{"rm", recordID, "--yes"})
		err := rootCmd.Execute()

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: _deleted_at is set to now
		// Then: _deleted_by is set to current actor
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecordIncludeDeleted("inventory", recordID)
		if err != nil {
			t.Fatalf("failed to get record: %v", err)
		}

		if rec.DeletedAt == nil {
			t.Error("expected _deleted_at to be set")
		}
		if rec.DeletedBy == "" {
			t.Error("expected _deleted_by to be set")
		}

		// Then: Record is excluded from `stash list`
		activeRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		for _, r := range activeRecords {
			if r.ID == recordID {
				t.Error("expected deleted record to be excluded from list")
			}
		}
	})

	t.Run("AC-02: delete with cascade", func(t *testing.T) {
		// Given: Record inv-ex4j has children inv-ex4j.1 and inv-ex4j.2
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child records
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Case", "--parent", parentID})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash rm inv-ex4j --cascade --yes`
		rootCmd.SetArgs([]string{"rm", parentID, "--cascade", "--yes"})
		err := rootCmd.Execute()

		// Then: Parent and all children are soft-deleted
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Parent should be deleted
		parent, _ := store.GetRecordIncludeDeleted("inventory", parentID)
		if !parent.IsDeleted() {
			t.Error("expected parent to be deleted")
		}

		// All children should be deleted
		children, _ := store.GetChildrenIncludeDeleted("inventory", parentID)
		if len(children) != 2 {
			t.Errorf("expected 2 children, got %d", len(children))
		}
		for _, child := range children {
			if !child.IsDeleted() {
				t.Errorf("expected child %s to be deleted", child.ID)
			}
		}
	})

	t.Run("AC-03: reject delete without cascade when children exist", func(t *testing.T) {
		// Given: Record inv-ex4j has children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child record
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash rm inv-ex4j` (no --cascade)
		rootCmd.SetArgs([]string{"rm", parentID, "--yes"})
		rootCmd.Execute()

		// Then: Command fails with appropriate error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when deleting record with children")
		}

		// Then: No records are deleted
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		parent, _ := store.GetRecord("inventory", parentID)
		if parent.IsDeleted() {
			t.Error("expected parent NOT to be deleted")
		}
	})

	t.Run("AC-04: skip confirmation with --yes", func(t *testing.T) {
		// Given: Record inv-ex4j exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash rm inv-ex4j --yes`
		rootCmd.SetArgs([]string{"rm", recordID, "--yes"})
		err := rootCmd.Execute()

		// Then: Record is deleted without prompting
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecordIncludeDeleted("inventory", recordID)
		if !rec.IsDeleted() {
			t.Error("expected record to be deleted")
		}
	})
}

// TestUC_REC_005_DeleteRecord_MustNot tests anti-requirements
func TestUC_REC_005_DeleteRecord_MustNot(t *testing.T) {
	t.Run("must not permanently remove data", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User deletes the record
		rootCmd.SetArgs([]string{"rm", recordID, "--yes"})
		rootCmd.Execute()

		// Then: Record still exists (just marked as deleted)
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecordIncludeDeleted("inventory", recordID)
		if err != nil {
			t.Fatalf("expected record to still exist, got error: %v", err)
		}
		if !rec.IsDeleted() {
			t.Error("expected record to be marked as deleted")
		}
	})

	t.Run("must not delete children without --cascade flag", func(t *testing.T) {
		// Given: Record has children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: User tries to delete parent without --cascade
		rootCmd.SetArgs([]string{"rm", parentID, "--yes"})
		rootCmd.Execute()

		// Then: Command should fail
		if ExitCode == 0 {
			t.Error("expected non-zero exit code")
		}
	})
}

// TestUC_REC_005_DeleteRecord_JSONOutput tests JSON output for rm command
func TestUC_REC_005_DeleteRecord_JSONOutput(t *testing.T) {
	t.Run("JSON output shows deleted record", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash rm inv-ex4j --yes --json`
		rootCmd.SetArgs([]string{"rm", recordID, "--yes", "--json"})
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

		// Then: Contains deleted count
		if jsonOutput["deleted"] != float64(1) {
			t.Errorf("expected deleted=1, got %v", jsonOutput["deleted"])
		}
	})
}

// TestUC_REC_005_RejectAlreadyDeleted tests that rm rejects already deleted records
func TestUC_REC_005_RejectAlreadyDeleted(t *testing.T) {
	t.Run("reject delete of already deleted record", func(t *testing.T) {
		// Given: Record is already deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User tries to delete again
		rootCmd.SetArgs([]string{"rm", recordID, "--yes"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})
}

// TestUC_REC_005_RejectNonExistent tests that rm rejects non-existent records
func TestUC_REC_005_RejectNonExistent(t *testing.T) {
	t.Run("reject delete of non-existent record", func(t *testing.T) {
		// Given: No record exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: User tries to delete non-existent record
		rootCmd.SetArgs([]string{"rm", "inv-fake", "--yes"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})
}
